import { writable, type Readable } from 'svelte/store';
import type { DropZone } from './model';
import { layout, type LayoutStore } from '../../lib/stores/layout';

export interface DragState {
  paneId: string;
  title: string;
  targetId: string | null;
  zone: DropZone | null;
  rect: { x: number; y: number; w: number; h: number } | null;
}

const state = writable<DragState | null>(null);
export const dragState: Readable<DragState | null> = { subscribe: state.subscribe };

let active = false;

export function zoneAt(rect: { x: number; y: number; w: number; h: number }, x: number, y: number): DropZone {
  if (!(rect.w > 0) || !(rect.h > 0)) return 'center';
  const cx = Math.min(Math.max(x, rect.x), rect.x + rect.w);
  const cy = Math.min(Math.max(y, rect.y), rect.y + rect.h);
  const dL = cx - rect.x;
  const dR = rect.x + rect.w - cx;
  const dT = cy - rect.y;
  const dB = rect.y + rect.h - cy;
  const cands: Array<[DropZone, number]> = [];
  if (dT <= 0.25 * rect.h) cands.push(['top', dT]);
  if (dB <= 0.25 * rect.h) cands.push(['bottom', dB]);
  if (dL <= 0.25 * rect.w) cands.push(['left', dL]);
  if (dR <= 0.25 * rect.w) cands.push(['right', dR]);
  if (cands.length === 0) return 'center';
  let best = cands[0];
  for (const c of cands) if (c[1] < best[1]) best = c;
  return best[0];
}

export function startPaneDrag(e: PointerEvent, paneId: string, title: string, store: LayoutStore = layout): void {
  if (e.button !== 0 || active) return;
  active = true;

  const startX = e.clientX;
  const startY = e.clientY;
  const pointerId = e.pointerId;
  const source = e.currentTarget as HTMLElement | null;
  const prevUserSelect = document.body.style.userSelect;

  let started = false;
  let ghost: HTMLDivElement | null = null;
  let cur: DragState | null = null;
  let done = false;

  function setCur(next: DragState) {
    if (cur && cur.targetId === next.targetId && cur.zone === next.zone &&
        cur.rect?.x === next.rect?.x && cur.rect?.y === next.rect?.y &&
        cur.rect?.w === next.rect?.w && cur.rect?.h === next.rect?.h) return;
    cur = next;
    state.set(next);
  }

  function begin() {
    started = true;
    try { source?.setPointerCapture(pointerId); } catch { /* abaikan */ }
    const g = document.createElement('div');
    g.textContent = title;
    g.style.cssText =
      'position:fixed;z-index:10000;pointer-events:none;max-width:240px;overflow:hidden;' +
      'text-overflow:ellipsis;white-space:nowrap;padding:3px 8px;border-radius:4px;font-size:12px;' +
      'opacity:0.9;background:var(--bg-elev,#222);border:1px solid var(--border,#444);color:var(--fg,#ddd);' +
      `left:${startX + 12}px;top:${startY + 12}px`;
    document.body.appendChild(g);
    ghost = g;
    document.body.classList.add('pane-dragging');
    document.body.style.userSelect = 'none';
    cur = { paneId, title, targetId: null, zone: null, rect: null };
    state.set(cur);
  }

  function onMove(ev: PointerEvent) {
    if (!started) {
      if (Math.hypot(ev.clientX - startX, ev.clientY - startY) < 4) return;
      begin();
    }
    if (ghost) {
      ghost.style.left = `${ev.clientX + 12}px`;
      ghost.style.top = `${ev.clientY + 12}px`;
    }
    const el = document.elementFromPoint(ev.clientX, ev.clientY);
    const t = el?.closest('[data-pane-id]') as HTMLElement | null;
    if (t && t.dataset.paneId && t.dataset.paneId !== paneId) {
      const r = t.getBoundingClientRect();
      const rect = { x: r.left, y: r.top, w: r.width, h: r.height };
      setCur({ paneId, title, targetId: t.dataset.paneId, zone: zoneAt(rect, ev.clientX, ev.clientY), rect });
    } else {
      setCur({ paneId, title, targetId: null, zone: null, rect: null });
    }
  }

  function swallowClick() {
    const onClick = (ev: MouseEvent) => {
      ev.stopPropagation();
      ev.preventDefault();
      window.removeEventListener('click', onClick, true);
    };
    window.addEventListener('click', onClick, { capture: true, once: true });
    setTimeout(() => window.removeEventListener('click', onClick, true), 0);
  }

  function onUp() {
    const wasStarted = started;
    const target = cur;
    cleanup();
    if (wasStarted) {
      if (target && target.targetId && target.zone) store.movePane(paneId, target.targetId, target.zone);
      swallowClick();
    }
  }

  function onKey(ev: KeyboardEvent) {
    if (ev.key !== 'Escape') return;
    ev.preventDefault();
    ev.stopPropagation();
    const wasStarted = started;
    cleanup();
    if (wasStarted) swallowClick();
  }

  function onCancel() { cleanup(); }

  function cleanup() {
    if (done) return;
    done = true;
    window.removeEventListener('pointermove', onMove);
    window.removeEventListener('pointerup', onUp);
    window.removeEventListener('pointercancel', onCancel);
    window.removeEventListener('keydown', onKey, true);
    window.removeEventListener('blur', onCancel);
    source?.removeEventListener('lostpointercapture', onCancel);
    try { source?.releasePointerCapture(pointerId); } catch { /* abaikan */ }
    ghost?.remove();
    ghost = null;
    document.body.classList.remove('pane-dragging');
    document.body.style.userSelect = prevUserSelect;
    state.set(null);
    active = false;
  }

  window.addEventListener('pointermove', onMove);
  window.addEventListener('pointerup', onUp);
  window.addEventListener('pointercancel', onCancel);
  window.addEventListener('keydown', onKey, true);
  window.addEventListener('blur', onCancel);
  source?.addEventListener('lostpointercapture', () => {
    // pointerup dikirim sebelum lostpointercapture; bila drag masih berjalan, berarti dibatalkan
    if (!done) onCancel();
  });
}
