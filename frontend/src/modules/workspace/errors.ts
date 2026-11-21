export function errMessage(e: unknown): string {
  let m: string;
  if (e instanceof Error) m = e.message;
  else if (typeof e === 'string') m = e;
  else m = 'terjadi kesalahan';
  if (!m) m = 'terjadi kesalahan';
  return m.length > 120 ? m.slice(0, 117) + '...' : m;
}

function nativeFn(): { kind: 'bridge' | 'runtime'; fn: Function } | null {
  try {
    const w = window as any;
    const b = w.go?.main?.Bridge?.OpenDirectoryDialog;
    if (typeof b === 'function') return { kind: 'bridge', fn: b };
    const r = w.runtime?.OpenDirectoryDialog;
    if (typeof r === 'function') return { kind: 'runtime', fn: r };
  } catch {
    /* abaikan */
  }
  return null;
}

export function hasNativeFolderDialog(): boolean {
  return nativeFn() !== null;
}

export async function pickFolder(): Promise<string | null> {
  try {
    const n = nativeFn();
    if (!n) return null;
    const res = n.kind === 'runtime' ? await n.fn({ Title: 'Pilih folder project' }) : await n.fn();
    return typeof res === 'string' && res ? res : null;
  } catch {
    return null;
  }
}
