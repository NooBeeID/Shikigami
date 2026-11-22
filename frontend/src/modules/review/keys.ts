// Navigasi keyboard panel review: j/k pindah hunk, a terima, r revert, Enter/Esc saat konfirmasi.
// Listener dipasang pada elemen panel (bukan window) agar tidak bentrok dengan shortcut global.
export interface ReviewKeyHandlers {
  target: HTMLElement;
  next(): void;
  prev(): void;
  accept(): void;
  revert(): void;
  confirm(): void;
  cancel(): void;
  isConfirming?: () => boolean;
}

export function attachReviewKeys(handlers: ReviewKeyHandlers): () => void {
  const onKey = (e: KeyboardEvent) => {
    if (e.defaultPrevented) return;
    if (e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return;
    const t = e.target as HTMLElement | null;
    if (t) {
      const tag = t.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || t.isContentEditable) return;
    }
    let fn: (() => void) | null = null;
    switch (e.key) {
      case 'j': fn = handlers.next; break;
      case 'k': fn = handlers.prev; break;
      case 'a': fn = handlers.accept; break;
      case 'r': fn = handlers.revert; break;
      case 'Enter': if (handlers.isConfirming?.() === true) fn = handlers.confirm; break;
      case 'Escape': if (handlers.isConfirming?.() === true) fn = handlers.cancel; break;
    }
    if (!fn) return;
    e.preventDefault();
    e.stopPropagation();
    fn();
  };
  handlers.target.addEventListener('keydown', onKey);
  return () => handlers.target.removeEventListener('keydown', onKey);
}
