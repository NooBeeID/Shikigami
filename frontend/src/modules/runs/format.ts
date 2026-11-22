import type { Run, Progress, Usage } from './types';

// Lama berjalan sejak `startedAt` (ms) sampai `ref` (ms), gaya ringkas.
export function formatElapsed(startedAt: number, ref: number): string {
  const s = Math.max(0, Math.floor((ref - startedAt) / 1000));
  if (s < 60) return `${s} dtk`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} mnt`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  if (h < 24) return rm ? `${h} jam ${rm} mnt` : `${h} jam`;
  return `${Math.floor(h / 24)} hari`;
}

// Judul baris: "name · identity"; tanpa pemisah bila identity kosong.
export function runTitle(r: Pick<Run, 'name' | 'identity'>): string {
  return r.identity ? `${r.name} · ${r.identity}` : r.name;
}

// Teks progres workflow, mis. "Probe · 1/2 agent". Kosong bila tak ada progres.
export function progressText(p?: Progress): string {
  if (!p) return '';
  const hasCount = p.total !== undefined && p.total > 0;
  const count = hasCount ? `${p.done ?? 0}/${p.total} agent` : '';
  if (p.phaseTitle && count) return `${p.phaseTitle} · ${count}`;
  return p.phaseTitle || count;
}

// Penanda ringkas untuk sub agent mandiri, mis. "☑ 1/3"; kosong bila total tak ada.
export function subagentProgress(p?: Progress): string {
  if (!p || !p.total) return '';
  return `☑ ${p.done ?? 0}/${p.total}`;
}

// Hitungan agent milik workflow, mis. "1/2".
export function agentsCount(p?: Progress): string {
  const a = p?.agents ?? [];
  return `${a.filter((x) => x.done).length}/${a.length}`;
}

// Jumlah token ringkas: 842, 1,2rb, 38rb, 1,4jt.
export function formatTokens(n: number): string {
  if (n < 1000) return String(n);
  const f = (v: number, unit: string) => `${(v >= 10 ? Math.round(v) : Math.round(v * 10) / 10).toString().replace('.', ',')}${unit}`;
  if (n < 1_000_000) return f(n / 1000, 'rb');
  return f(n / 1_000_000, 'jt');
}

// Ringkasan usage, mis. "38rb in · 2,1rb out"; kosong bila tak ada.
export function usageText(u?: Usage): string {
  if (!u || (!u.input && !u.output && !u.cacheRead)) return '';
  return `${formatTokens(u.input)} in · ${formatTokens(u.output)} out`;
}

// Tooltip usage lengkap, termasuk token yang dibaca dari cache.
export function usageTitle(u?: Usage): string {
  if (!u) return '';
  return `Input baru: ${u.input.toLocaleString('id-ID')}\nDibaca dari cache: ${u.cacheRead.toLocaleString('id-ID')}\nOutput: ${u.output.toLocaleString('id-ID')}`;
}

// "claude-opus-5-5" -> "opus 5.5"; "claude-haiku-4-5-20251001" -> "haiku 4.5".
// ID yang tak dikenali dikembalikan apa adanya.
export function shortModel(id?: string): string {
  if (!id) return '';
  const m = /^claude-([a-z]+)-(\d+)(?:-(\d{1,2}))?(?:-\d{8})?$/.exec(id);
  if (!m) return id;
  return m[3] ? `${m[1]} ${m[2]}.${m[3]}` : `${m[1]} ${m[2]}`;
}

// Model unik dari agent workflow, mis. "sonnet 5.5 ×2, haiku 5.5 ×1". Kosong bila tak ada.
export function workflowModels(p?: Progress): string {
  const counts = new Map<string, number>();
  for (const a of p?.agents ?? []) {
    const m = shortModel(a.model);
    if (m) counts.set(m, (counts.get(m) ?? 0) + 1);
  }
  const parts = [...counts].map(([m, c]) => (counts.size > 1 || c > 1 ? `${m} ×${c}` : m));
  return parts.join(', ');
}
