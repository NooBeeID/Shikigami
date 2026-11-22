// Drag sesi ke tab grup (pointer events). State UI sementara saja.
import { get, writable, type Readable } from 'svelte/store';
import { activeGroupId, groupOfSession, mainGroupId, moveSession, setActiveGroup } from '../../lib/stores/groups';
import { layout } from '../../lib/stores/layout';
import { activeTab, findLeaf } from '../layout/model';
import { dragState } from '../layout/dnd';

export interface GroupDropState { sessionId: string; overGroupId: string | null }

const dropState = writable<GroupDropState | null>(null);
export const groupDropState: Readable<GroupDropState | null> = { subscribe: dropState.subscribe };
export const onGroupTabDragState = groupDropState;

const HOVER_MS = 600;
const THRESHOLD = 4;

export function hoveredGroupTabId(x: number, y: number): string | null {
  for (const el of document.elementsFromPoint(x, y)) {
    const tab = (el as HTMLElement).closest?.('[data-group-tab-id]') as HTMLElement | null;
    if (tab) return tab.dataset.groupTabId ?? null;
  }
  return null;
}

function currentGroupOf(sessionId: string): string {
  return groupOfSession(sessionId) ?? get(mainGroupId);
}

/** Pelacak hover: set store bila berubah dan jadwalkan aktivasi tab (sekali jalan). */
function createHoverTracker(sessionId: string) {
  let over: string | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  const clear = () => {
    if (timer) clearTimeout(timer);
    timer = null;
  };
  return {
    get over() { return over; },
    update(x: number, y: number) {
      const id = hoveredGroupTabId(x, y);
      if (id === over) return;
      over = id;
      clear();
      dropState.set({ sessionId, overGroupId: id });
      if (id && id !== get(activeGroupId)) {
        timer = setTimeout(() => {
          timer = null;
          if (over === id) setActiveGroup(id).catch(() => {});
        }, HOVER_MS);
      }
    },
    reset() {
      over = null;
      clear();
      dropState.set({ sessionId, overGroupId: null });
    },
    dispose() {
      over = null;
      clear();
      dropState.set(null);
    },
  };
}

let dragging = false;

export function startSessionDrag(e: PointerEvent, sessionId: string, title: string): void {
  if (e.button !== 0 || dragging) return;
  dragging = true;
  const sx = e.clientX;
  const sy = e.clientY;
  let started = false;
  let ghost: HTMLDivElement | null = null;
  const hover = createHoverTracker(sessionId);

  const begin = () => {
    started = true;
    ghost = document.createElement('div');
    ghost.textContent = title;
    ghost.style.cssText =
      'position:fixed;pointer-events:none;z-index:10000;padding:0.25rem 0.625rem;font-size:0.75rem;' +
      'max-width:16rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;border-radius:0.25rem;' +
      'background:var(--bg-elev);border:1px solid var(--border);color:var(--fg);';
    document.body.appendChild(ghost);
    document.body.style.userSelect = 'none';
    dropState.set({ sessionId, overGroupId: null });
  };

  const swallowClick = () => {
    const h = (ev: Event) => {
      ev.stopPropagation();
      ev.preventDefault();
    };
    window.addEventListener('click', h, true);
    setTimeout(() => window.removeEventListener('click', h, true), 0);
  };

  const cleanup = () => {
    window.removeEventListener('pointermove', onMove, true);
    window.removeEventListener('pointerup', onUp, true);
    window.removeEventListener('pointercancel', onCancel, true);
    window.removeEventListener('keydown', onKey, true);
    window.removeEventListener('blur', onCancel);
    ghost?.remove();
    ghost = null;
    document.body.style.userSelect = '';
    hover.dispose();
    dragging = false;
  };

  function onMove(ev: PointerEvent) {
    if (!started) {
      if (Math.hypot(ev.clientX - sx, ev.clientY - sy) < THRESHOLD) return;
      begin();
    }
    if (ghost) {
      ghost.style.left = `${ev.clientX + 12}px`;
      ghost.style.top = `${ev.clientY + 12}px`;
    }
    hover.update(ev.clientX, ev.clientY);
  }
  function onUp(ev: PointerEvent) {
    const wasStarted = started;
    if (wasStarted) hover.update(ev.clientX, ev.clientY);
    const target = hover.over;
    cleanup();
    if (!wasStarted) return;
    swallowClick();
    if (target && target !== currentGroupOf(sessionId)) {
      moveSession(sessionId, target).catch(() => {});
    }
  }
  function onCancel() {
    const was = started;
    cleanup();
    if (was) swallowClick();
  }
  function onKey(ev: KeyboardEvent) {
    if (ev.key === 'Escape') {
      ev.preventDefault();
      onCancel();
    }
  }

  window.addEventListener('pointermove', onMove, true);
  window.addEventListener('pointerup', onUp, true);
  window.addEventListener('pointercancel', onCancel, true);
  window.addEventListener('keydown', onKey, true);
  window.addEventListener('blur', onCancel);
}

/** Memantau drag header pane (dikelola layout); pane terminal bisa dijatuhkan ke tab grup. */
export function watchPaneDrag(): () => void {
  let sessionId: string | null = null;
  let hover: ReturnType<typeof createHoverTracker> | null = null;
  let detach: (() => void) | null = null;

  const stop = () => {
    detach?.();
    detach = null;
    hover?.dispose();
    hover = null;
    sessionId = null;
  };

  const unsub = dragState.subscribe((st) => {
    if (st && !sessionId) {
      const pane = findLeaf(get(layout), st.paneId);
      const tab = pane ? activeTab(pane) : null;
      if (tab?.kind !== 'terminal' || !tab.sessionId) return;
      const sid = tab.sessionId;
      sessionId = sid;
      const h = createHoverTracker(sid);
      hover = h;
      const onMove = (ev: PointerEvent) => h.update(ev.clientX, ev.clientY);
      const cancel = () => h.reset();
      const onKey = (ev: KeyboardEvent) => {
        if (ev.key === 'Escape') cancel();
      };
      window.addEventListener('pointermove', onMove, true);
      window.addEventListener('keydown', onKey, true);
      window.addEventListener('pointercancel', cancel, true);
      detach = () => {
        window.removeEventListener('pointermove', onMove, true);
        window.removeEventListener('keydown', onKey, true);
        window.removeEventListener('pointercancel', cancel, true);
      };
    } else if (!st && sessionId) {
      const sid = sessionId;
      const target = hover?.over ?? null;
      stop();
      if (target && target !== currentGroupOf(sid)) {
        moveSession(sid, target).catch(() => {});
      }
    }
  });

  return () => {
    unsub();
    stop();
  };
}
