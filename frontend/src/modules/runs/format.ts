import type { Run, Progress } from './types';

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
