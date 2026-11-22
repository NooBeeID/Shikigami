import { onMenuEvent } from '../appmenu';
import { get } from 'svelte/store';
import { toggleLeft, toggleRight } from '../../lib/stores/shell';
import { activeProjectId } from '../../lib/stores';
import { createAndOpenSession, openFolder } from './helpers';

const DEDUPE_MS = 250;

function isBlockedTarget(t: EventTarget | null): boolean {
  const el = t as HTMLElement | null;
  if (!el || !el.tagName) return false;
  const tag = el.tagName;
  if (tag === 'TEXTAREA') return !el.classList.contains('xterm-helper-textarea');
  return tag === 'INPUT' || tag === 'SELECT';
}

export function installShellShortcuts(): () => void {
  const last = { left: -Infinity, right: -Infinity };
  const fire = (side: 'left' | 'right') => {
    const now = performance.now();
    if (now - last[side] < DEDUPE_MS) return;
    last[side] = now;
    if (side === 'left') toggleLeft();
    else toggleRight();
  };

  const offLeft = onMenuEvent('menu:toggle-sidebar-left', () => fire('left'));
  const offRight = onMenuEvent('menu:toggle-sidebar-right', () => fire('right'));

  const onKey = (e: KeyboardEvent) => {
    if (!e.metaKey || e.code !== 'KeyB' || e.shiftKey || e.ctrlKey) return;
    if (isBlockedTarget(e.target)) return;
    e.preventDefault();
    e.stopPropagation();
    fire(e.altKey ? 'right' : 'left');
  };
  window.addEventListener('keydown', onKey, true);

  // ⌘T = sesi claude baru, ⇧⌘T = sesi shell baru (project aktif).
  let newInflight = false;
  const onNewSession = (e: KeyboardEvent) => {
    if (!e.metaKey || e.code !== 'KeyT' || e.ctrlKey || e.altKey) return;
    if (isBlockedTarget(e.target)) return;
    e.preventDefault();
    e.stopPropagation();
    if (e.repeat || newInflight) return;
    const projectId = get(activeProjectId);
    if (!projectId) {
      openFolder();
      return;
    }
    newInflight = true;
    createAndOpenSession(e.shiftKey ? 'shell' : 'claude', projectId)
      .catch((err) => console.error('[shell] gagal membuat sesi baru:', err))
      .finally(() => { newInflight = false; });
  };
  window.addEventListener('keydown', onNewSession, true);

  return () => {
    window.removeEventListener('keydown', onNewSession, true);
    offLeft();
    offRight();
    window.removeEventListener('keydown', onKey, true);
  };
}
