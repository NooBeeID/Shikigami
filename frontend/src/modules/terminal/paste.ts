// Jembatan untuk menempelkan teks ke terminal sebuah sesi dari luar komponen
// (mis. saat file dijatuhkan dari Finder ke pane). Memakai term.paste agar
// bracketed paste dihormati oleh program di dalamnya (mis. Claude Code).
const pasters = new Map<string, (text: string) => void>();

export function registerTerminalPaste(sessionId: string, fn: (text: string) => void): () => void {
  pasters.set(sessionId, fn);
  return () => {
    if (pasters.get(sessionId) === fn) pasters.delete(sessionId);
  };
}

// Mengembalikan false bila terminal sesi itu tidak sedang terpasang.
export function pasteToSession(sessionId: string, text: string): boolean {
  const fn = pasters.get(sessionId);
  if (!fn) return false;
  fn(text);
  return true;
}

// Escape ala Terminal.app: karakter selain yang aman diberi backslash.
export function shellEscapePath(p: string): string {
  return p.replace(/[^A-Za-z0-9_\-.,:@%+=\/~\u0080-\uFFFF]/g, '\\$&');
}

// Beberapa path dipisah spasi, diakhiri satu spasi agar pengguna bisa lanjut mengetik.
export function formatDroppedPaths(paths: string[]): string {
  return paths.map(shellEscapePath).join(' ') + ' ';
}
