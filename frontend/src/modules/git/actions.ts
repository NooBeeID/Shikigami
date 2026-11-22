import { get } from 'svelte/store';
import { layout } from '../../lib/stores/layout';
import { showToast } from '../openfolder/toast-store';
import { pasteToSession } from '../terminal';

export async function copyLink(url: string): Promise<void> {
  let ok = false;
  try {
    await navigator.clipboard.writeText(url);
    ok = true;
  } catch {
    try {
      const ta = document.createElement('textarea');
      ta.value = url;
      ta.style.cssText = 'position:fixed;opacity:0';
      document.body.appendChild(ta);
      ta.select();
      ok = document.execCommand('copy');
      ta.remove();
    } catch {
      ok = false;
    }
  }
  if (ok) showToast('Link disalin');
  else showToast('Gagal menyalin link', 'error');
}

export async function sendToAgent(url: string): Promise<void> {
  const doc = get(layout);
  const all = get(layout.leaves);
  const focused = all.find((l) => l.id === doc.focusId);
  const target =
    focused && focused.kind === 'terminal' && focused.sessionId
      ? focused
      : all.find((l) => l.kind === 'terminal' && l.sessionId);
  if (!target?.sessionId || !pasteToSession(target.sessionId, url + ' ')) {
    showToast('Tidak ada sesi aktif', 'error');
  }
}

export function openInBrowser(url: string): void {
  if (!/^https?:\/\//i.test(url)) {
    showToast('Link tidak valid', 'error');
    return;
  }
  const open = (window as any).runtime?.BrowserOpenURL;
  if (typeof open === 'function') open(url);
  else window.open(url, '_blank', 'noopener,noreferrer');
}
