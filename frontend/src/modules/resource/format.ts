const MIB = 1024 * 1024;
const GIB = 1024 * MIB;

export function formatBytes(n: number): string {
  if (n >= GIB) return `${(n / GIB).toFixed(1).replace('.', ',')} GB`;
  return `${Math.round(n / MIB)} MB`;
}

export function formatCpu(p: number): string {
  if (p < 0.1) return '< 0,1%';
  if (p < 10) return `${p.toFixed(1).replace('.', ',')}%`;
  return `${Math.round(p)}%`;
}

// Nama baris dropdown. Baris sesi memakai judul dari UI (titleOf; undefined bila sesi tak dikenal
// -> pakai label daemon). Akhiran 4 karakter id ditambah hanya bila judul kembar.
export function rowNames(
  procs: { kind: string; sessionId: string; label: string; pid: number }[],
  titleOf: (sessionId: string) => string | undefined,
): Map<string, string> {
  const base = procs.map((p) => (p.kind === 'session' ? (titleOf(p.sessionId) ?? p.label) : p.label));
  const count = new Map<string, number>();
  procs.forEach((p, i) => {
    if (p.kind === 'session') count.set(base[i], (count.get(base[i]) ?? 0) + 1);
  });
  const out = new Map<string, string>();
  procs.forEach((p, i) => {
    const dup = p.kind === 'session' && (count.get(base[i]) ?? 0) > 1;
    out.set(rowKey(p), dup ? `${base[i]} · ${p.sessionId.slice(0, 4)}` : base[i]);
  });
  return out;
}

export function rowKey(p: { kind: string; sessionId: string; pid: number }): string {
  return `${p.kind}:${p.sessionId}:${p.pid}`;
}
