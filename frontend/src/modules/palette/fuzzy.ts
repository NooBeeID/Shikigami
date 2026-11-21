export interface FuzzyResult { score: number; indices: number[] }

export const GROUP_LIMIT = 8;

const BOUNDARY = new Set([' ', '/', '-', '_', '.']);

function isBoundary(text: string, i: number): boolean {
  return i === 0 || BOUNDARY.has(text[i - 1]);
}

function matchWord(word: string, text: string): FuzzyResult | null {
  const at = text.indexOf(word);
  if (at >= 0) {
    let score = 1000 - at;
    if (isBoundary(text, at)) score += 200;
    if (at === 0) score += 100;
    const indices = Array.from({ length: word.length }, (_, k) => at + k);
    return { score, indices };
  }
  const indices: number[] = [];
  let from = 0;
  let bonus = 0;
  for (const ch of word) {
    const idx = text.indexOf(ch, from);
    if (idx < 0) return null;
    if (isBoundary(text, idx)) bonus += 10;
    indices.push(idx);
    from = idx + 1;
  }
  const span = indices[indices.length - 1] - indices[0] + 1;
  return { score: 500 - span - indices[0] + bonus, indices };
}

export function fuzzyMatch(query: string, text: string): FuzzyResult | null {
  const q = query.toLowerCase().trim();
  if (!q) return { score: 0, indices: [] };
  const t = text.toLowerCase();
  const words = q.split(/\s+/);
  let score = 0;
  const indices: number[] = [];
  for (const w of words) {
    const r = matchWord(w, t);
    if (!r) return null;
    score += r.score;
    indices.push(...r.indices);
  }
  return { score, indices };
}

export function fuzzyFilter<T>(items: T[], query: string, text: (t: T) => string, limit?: number): T[] {
  if (!query.trim()) return limit === undefined ? items : items.slice(0, limit);
  const scored: { item: T; score: number; order: number }[] = [];
  items.forEach((item, order) => {
    const r = fuzzyMatch(query, text(item));
    if (r) scored.push({ item, score: r.score, order });
  });
  scored.sort((a, b) => b.score - a.score || a.order - b.order);
  const out = scored.map((s) => s.item);
  return limit === undefined ? out : out.slice(0, limit);
}
