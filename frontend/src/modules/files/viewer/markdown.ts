// Parser markdown murni (tanpa DOM, tanpa HTML). Menghasilkan pohon node; HTML mentah
// di sumber diperlakukan sebagai teks literal. Renderer (MarkdownView) memakai Svelte
// sehingga semua teks otomatis di-escape.

export type Inline =
  | { t: 'text'; v: string }
  | { t: 'code'; v: string }
  | { t: 'strong'; c: Inline[] }
  | { t: 'em'; c: Inline[] }
  | { t: 'del'; c: Inline[] }
  | { t: 'link'; href: string; c: Inline[] } // href SUDAH lolos isSafeHref
  | { t: 'br' };
export type Block =
  | { t: 'heading'; level: 1 | 2 | 3 | 4 | 5 | 6; c: Inline[] }
  | { t: 'para'; c: Inline[] }
  | { t: 'code'; lang: string; v: string }
  | { t: 'quote'; c: Block[] }
  | { t: 'list'; ordered: boolean; start: number; items: Block[][] }
  | { t: 'hr' }
  | { t: 'table'; head: Inline[][]; align: ('left' | 'right' | 'center' | null)[]; rows: Inline[][][] };

const MAX_BLOCK_DEPTH = 8;
const MAX_INLINE_DEPTH = 6;
const MAX_LINK_DEPTH = 4;
const MAX_SCAN = 2000;

const UNSAFE_CHARS = /[\u0000-\u001f\u007f\s]/g;

function cleanHref(raw: string): string {
  return raw.trim().replace(UNSAFE_CHARS, '');
}

export function isSafeHref(raw: string): boolean {
  return /^(https?:|mailto:)/i.test(cleanHref(raw));
}

// ---------------------------------------------------------------- blok

function isBlank(l: string): boolean {
  return l.trim() === '';
}

function leadingSpaces(l: string): number {
  let i = 0;
  while (i < l.length && l[i] === ' ') i++;
  return i;
}

function parseHeading(line: string): { level: 1 | 2 | 3 | 4 | 5 | 6; text: string } | null {
  const ind = leadingSpaces(line);
  if (ind > 3) return null;
  let h = 0;
  while (line[ind + h] === '#' && h <= 6) h++;
  if (h < 1 || h > 6) return null;
  const rest = line.slice(ind + h);
  if (rest.length > 0 && rest[0] !== ' ' && rest[0] !== '\t') return null;
  let t = rest.trim();
  let e = t.length;
  while (e > 0 && t[e - 1] === '#') e--;
  if (e < t.length && (e === 0 || t[e - 1] === ' ' || t[e - 1] === '\t')) t = t.slice(0, e).trimEnd();
  return { level: h as 1 | 2 | 3 | 4 | 5 | 6, text: t };
}

function isHr(line: string): boolean {
  if (leadingSpaces(line) > 3) return false;
  let ch = '';
  let count = 0;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (c === ' ' || c === '\t') continue;
    if (c !== '-' && c !== '*' && c !== '_') return false;
    if (ch === '') ch = c;
    else if (c !== ch) return false;
    count++;
  }
  return count >= 3;
}

interface Fence { ch: string; len: number; lang: string }
function parseFence(line: string): Fence | null {
  const ind = leadingSpaces(line);
  if (ind > 3) return null;
  const ch = line[ind];
  if (ch !== '`' && ch !== '~') return null;
  let k = ind;
  while (line[k] === ch) k++;
  const len = k - ind;
  if (len < 3) return null;
  const info = line.slice(k).trim();
  if (ch === '`' && info.includes('`')) return null;
  const sp = info.search(/\s/);
  return { ch, len, lang: sp < 0 ? info : info.slice(0, sp) };
}

function closesFence(line: string, f: Fence): boolean {
  const t = line.trim();
  if (t.length < f.len) return false;
  for (let i = 0; i < t.length; i++) if (t[i] !== f.ch) return false;
  return true;
}

function isQuote(line: string): boolean {
  const ind = leadingSpaces(line);
  return ind <= 3 && line[ind] === '>';
}

function stripQuote(line: string): string {
  const ind = leadingSpaces(line);
  let k = ind + 1;
  if (line[k] === ' ') k++;
  return line.slice(k);
}

interface Marker { indent: number; ordered: boolean; num: number; contentOffset: number; rest: string }
function listMarker(line: string): Marker | null {
  const indent = leadingSpaces(line);
  if (indent > 3) return null;
  const c = line[indent];
  let mEnd: number;
  let ordered = false;
  let num = 1;
  if (c === '-' || c === '*' || c === '+') {
    mEnd = indent + 1;
  } else if (c !== undefined && c >= '0' && c <= '9') {
    let j = indent;
    while (j < line.length && line[j] >= '0' && line[j] <= '9' && j - indent < 10) j++;
    if (j - indent > 9) return null;
    if (line[j] !== '.' && line[j] !== ')') return null;
    ordered = true;
    num = Number(line.slice(indent, j));
    mEnd = j + 1;
  } else {
    return null;
  }
  if (mEnd < line.length && line[mEnd] !== ' ' && line[mEnd] !== '\t') return null;
  let k = mEnd;
  while (k < line.length && (line[k] === ' ' || line[k] === '\t')) k++;
  if (k >= line.length) return { indent, ordered, num, contentOffset: mEnd + 1, rest: '' };
  if (k - mEnd > 4) return { indent, ordered, num, contentOffset: mEnd + 1, rest: line.slice(mEnd + 1) };
  return { indent, ordered, num, contentOffset: k, rest: line.slice(k) };
}

function splitRow(line: string): string[] {
  let t = line.trim();
  if (t.startsWith('|')) t = t.slice(1);
  if (t.endsWith('|') && !t.endsWith('\\|')) t = t.slice(0, -1);
  const cells: string[] = [];
  let cur = '';
  for (let i = 0; i < t.length; i++) {
    const c = t[i];
    if (c === '\\' && t[i + 1] === '|') {
      cur += '|';
      i++;
    } else if (c === '|') {
      cells.push(cur.trim());
      cur = '';
    } else {
      cur += c;
    }
  }
  cells.push(cur.trim());
  return cells;
}

function tableSeparator(line: string | undefined): ('left' | 'right' | 'center' | null)[] | null {
  if (line === undefined || !line.includes('-')) return null;
  const cells = splitRow(line);
  const align: ('left' | 'right' | 'center' | null)[] = [];
  for (const cell of cells) {
    if (!/^:?-+:?$/.test(cell)) return null;
    const l = cell.startsWith(':');
    const r = cell.endsWith(':');
    align.push(l && r ? 'center' : r ? 'right' : l ? 'left' : null);
  }
  return align;
}

function isTableStart(lines: string[], i: number): boolean {
  if (!lines[i].includes('|')) return false;
  const sep = tableSeparator(lines[i + 1]);
  return sep !== null && sep.length === splitRow(lines[i]).length;
}

function startsBlock(l: string): boolean {
  return parseFence(l) !== null || parseHeading(l) !== null || isHr(l) || isQuote(l);
}

function literalPara(lines: string[]): Block[] {
  return [{ t: 'para', c: [{ t: 'text', v: lines.join('\n') }] }];
}

function parseBlocks(lines: string[], depth: number): Block[] {
  if (depth > MAX_BLOCK_DEPTH) return literalPara(lines);
  const out: Block[] = [];
  const n = lines.length;
  let i = 0;
  while (i < n) {
    const line = lines[i];
    if (isBlank(line)) { i++; continue; }

    const fence = parseFence(line);
    if (fence) {
      const body: string[] = [];
      i++;
      while (i < n && !closesFence(lines[i], fence)) { body.push(lines[i]); i++; }
      if (i < n) i++;
      out.push({ t: 'code', lang: fence.lang, v: body.join('\n') });
      continue;
    }

    const h = parseHeading(line);
    if (h) {
      out.push({ t: 'heading', level: h.level, c: parseInline(h.text, 0) });
      i++;
      continue;
    }

    if (isHr(line)) { out.push({ t: 'hr' }); i++; continue; }

    if (isQuote(line)) {
      const body: string[] = [];
      while (i < n && isQuote(lines[i])) { body.push(stripQuote(lines[i])); i++; }
      out.push({ t: 'quote', c: parseBlocks(body, depth + 1) });
      continue;
    }

    const first = listMarker(line);
    if (first) {
      const ordered = first.ordered;
      const items: Block[][] = [];
      while (i < n) {
        const m = listMarker(lines[i]);
        if (!m || m.ordered !== ordered || isHr(lines[i])) break;
        const itemLines: string[] = [m.rest];
        i++;
        let blanks = 0;
        while (i < n) {
          const l = lines[i];
          if (isBlank(l)) { blanks++; i++; continue; }
          const ind = leadingSpaces(l);
          if (ind >= m.contentOffset) {
            for (let b = 0; b < blanks; b++) itemLines.push('');
            blanks = 0;
            itemLines.push(l.slice(m.contentOffset));
            i++;
            continue;
          }
          const lm = listMarker(l);
          if (lm && !isHr(l)) {
            if (lm.indent > m.indent) {
              for (let b = 0; b < blanks; b++) itemLines.push('');
              blanks = 0;
              itemLines.push(l.slice(Math.min(ind, m.contentOffset)));
              i++;
              continue;
            }
            break;
          }
          if (blanks === 0 && !startsBlock(l)) { itemLines.push(l.trim()); i++; continue; }
          break;
        }
        items.push(parseBlocks(itemLines, depth + 1));
      }
      out.push({ t: 'list', ordered, start: first.num, items });
      continue;
    }

    if (isTableStart(lines, i)) {
      const head = splitRow(line).map((c) => parseInline(c, 0));
      const align = tableSeparator(lines[i + 1])!;
      const cols = head.length;
      i += 2;
      const rows: Inline[][][] = [];
      while (i < n && !isBlank(lines[i]) && lines[i].includes('|')) {
        const cells = splitRow(lines[i]);
        const row: Inline[][] = [];
        for (let c = 0; c < cols; c++) row.push(parseInline(cells[c] ?? '', 0));
        rows.push(row);
        i++;
      }
      out.push({ t: 'table', head, align, rows });
      continue;
    }

    // paragraf
    const para: string[] = [line];
    i++;
    while (i < n) {
      const l = lines[i];
      if (isBlank(l) || startsBlock(l) || isTableStart(lines, i)) break;
      const lm = listMarker(l);
      if (lm && lm.rest !== '' && (!lm.ordered || lm.num === 1)) break;
      para.push(l);
      i++;
    }
    out.push({ t: 'para', c: parseInline(para.map((p, k) => (k === 0 ? p.trimStart() : p.trimStart())).join('\n').trimEnd(), 0) });
  }
  return out;
}

export function parseMarkdown(source: string): Block[] {
  const lines = source.replace(/\r\n?/g, '\n').split('\n');
  return parseBlocks(lines, 0);
}

// ---------------------------------------------------------------- inline

function isPunct(c: string): boolean {
  const k = c.charCodeAt(0);
  return (k >= 33 && k <= 47) || (k >= 58 && k <= 64) || (k >= 91 && k <= 96) || (k >= 123 && k <= 126);
}

function isWord(c: string | undefined): boolean {
  return c !== undefined && c !== '' && /[\p{L}\p{N}]/u.test(c);
}

function isSpace(c: string | undefined): boolean {
  return c === undefined || c === '' || /\s/.test(c);
}

// Cari penutup delimiter. Hanya bergantung pada validitas penutup (bukan pembuka),
// sehingga kegagalan pencarian dari `from` berlaku untuk semua `from` yang lebih besar.
function findClose(s: string, from: number, delim: string): number {
  const c = delim[0];
  const dbl = delim.length === 2;
  let pos = from;
  for (;;) {
    const j = s.indexOf(delim, pos);
    if (j < 0) return -1;
    pos = j + 1;
    if (j <= from) continue; // isi kosong
    // dilewati bila di-escape
    let bs = 0;
    while (j - 1 - bs >= 0 && s[j - 1 - bs] === '\\') bs++;
    if (bs % 2 === 1) continue;
    if (isSpace(s[j - 1])) continue;
    if (dbl) {
      let k = j;
      while (s[k + 2] === c) k++;
      if (c === '_' && isWord(s[k + 2])) { pos = k + 1; continue; }
      return k;
    }
    if (s[j - 1] === c || s[j + 1] === c) {
      // lewati seluruh barisan
      let k = j;
      while (s[k + 1] === c) k++;
      pos = k + 1;
      continue;
    }
    if (c === '_' && isWord(s[j + 1])) continue;
    return j;
  }
}

interface LinkParse { text: string; url: string; end: number }
// i menunjuk ke '['. Mengembalikan null bila bukan sintaks tautan yang valid.
function parseLinkAt(s: string, i: number): LinkParse | null {
  let depth = 0;
  let k = i;
  const limit = Math.min(s.length, i + MAX_SCAN);
  for (; k < limit; k++) {
    const c = s[k];
    if (c === '\\') { k++; continue; }
    if (c === '[') depth++;
    else if (c === ']') { depth--; if (depth === 0) break; }
  }
  if (k >= limit || s[k] !== ']' || s[k + 1] !== '(') return null;
  const text = s.slice(i + 1, k);
  let p = k + 2;
  const lim2 = Math.min(s.length, p + MAX_SCAN);
  while (p < lim2 && (s[p] === ' ' || s[p] === '\t' || s[p] === '\n')) p++;
  let url = '';
  if (s[p] === '<') {
    const e = s.indexOf('>', p + 1);
    if (e < 0 || e > lim2) return null;
    url = s.slice(p + 1, e);
    p = e + 1;
  } else {
    let pd = 0;
    const st = p;
    while (p < lim2) {
      const c = s[p];
      if (c === '\\' && p + 1 < s.length) { p += 2; continue; }
      if (c === ' ' || c === '\t' || c === '\n') break;
      if (c === '(') pd++;
      else if (c === ')') { if (pd === 0) break; pd--; }
      p++;
    }
    url = s.slice(st, p);
  }
  while (p < lim2 && (s[p] === ' ' || s[p] === '\t' || s[p] === '\n')) p++;
  const q = s[p];
  if (q === '"' || q === "'" || q === '(') {
    const closer = q === '(' ? ')' : q;
    let e = p + 1;
    while (e < lim2 && s[e] !== closer) { if (s[e] === '\\') e++; e++; }
    if (e >= lim2 || s[e] !== closer) return null;
    p = e + 1;
    while (p < lim2 && (s[p] === ' ' || s[p] === '\t' || s[p] === '\n')) p++;
  }
  if (s[p] !== ')') return null;
  return { text, url, end: p + 1 };
}

function plainText(nodes: Inline[]): string {
  let r = '';
  for (const n of nodes) {
    if (n.t === 'text' || n.t === 'code') r += n.v;
    else if (n.t === 'br') r += ' ';
    else r += plainText(n.c);
  }
  return r;
}

function trimTrailingSpaces(s: string): { s: string; n: number } {
  let e = s.length;
  while (e > 0 && s[e - 1] === ' ') e--;
  return { s: s.slice(0, e), n: s.length - e };
}

function parseInline(text: string, depth: number, inLink = false): Inline[] {
  const out: Inline[] = [];
  const n = text.length;
  let buf = '';
  let i = 0;
  const flush = () => { if (buf) { out.push({ t: 'text', v: buf }); buf = ''; } };
  const canFormat = depth < MAX_INLINE_DEPTH;

  while (i < n) {
    const c = text[i];

    if (c === '\\') {
      const nx = text[i + 1];
      if (nx === '\n') { flush(); out.push({ t: 'br' }); i += 2; continue; }
      if (nx !== undefined && isPunct(nx)) { buf += nx; i += 2; continue; }
      buf += c; i++;
      continue;
    }

    if (c === '\n') {
      const t = trimTrailingSpaces(buf);
      if (t.n >= 2) { buf = t.s; flush(); out.push({ t: 'br' }); }
      else { buf = t.s + ' '; }
      i++;
      continue;
    }

    if (c === '`') {
      let k = i;
      while (text[k] === '`') k++;
      const len = k - i;
      const fence = '`'.repeat(len);
      let pos = k;
      let found = -1;
      for (;;) {
        const j = text.indexOf(fence, pos);
        if (j < 0) break;
        if (text[j - 1] === '`' && j > k) { pos = j + 1; continue; }
        if (text[j + len] === '`') { let e = j + len; while (text[e] === '`') e++; pos = e; continue; }
        found = j;
        break;
      }
      if (found < 0) { buf += fence; i = k; continue; }
      let code = text.slice(k, found).replace(/\n/g, ' ');
      if (code.length > 2 && code[0] === ' ' && code[code.length - 1] === ' ' && code.trim() !== '') code = code.slice(1, -1);
      flush();
      out.push({ t: 'code', v: code });
      i = found + len;
      continue;
    }

    if ((c === '*' || c === '_' || c === '~') && canFormat) {
      const dbl = text[i + 1] === c;
      if (c === '~' && !dbl) { buf += c; i++; continue; }
      if (dbl) {
        const okOpen = c !== '_' || !isWord(text[i - 1]);
        if (okOpen && !isSpace(text[i + 2])) {
          const j = findClose(text, i + 2, c + c);
          if (j > 0) {
            flush();
            const inner = parseInline(text.slice(i + 2, j), depth + 1, inLink);
            out.push({ t: c === '~' ? 'del' : 'strong', c: inner });
            i = j + 2;
            continue;
          }
        }
        if (c === '~') { buf += '~~'; i += 2; continue; }
        buf += c; i++;
        continue;
      }
      const okOpen = c !== '_' || !isWord(text[i - 1]);
      if (okOpen && !isSpace(text[i + 1])) {
        const j = findClose(text, i + 1, c);
        if (j > 0) {
          flush();
          out.push({ t: 'em', c: parseInline(text.slice(i + 1, j), depth + 1, inLink) });
          i = j + 1;
          continue;
        }
      }
      buf += c; i++;
      continue;
    }

    if (c === '!' && text[i + 1] === '[') {
      const lp = parseLinkAt(text, i + 1);
      if (lp) {
        flush();
        const alt = lp.text.trim() || '[gambar]';
        if (!inLink && isSafeHref(lp.url)) out.push({ t: 'link', href: cleanHref(lp.url), c: [{ t: 'text', v: alt }] });
        else out.push({ t: 'text', v: alt });
        i = lp.end;
        continue;
      }
      buf += c; i++;
      continue;
    }

    if (c === '[') {
      const lp = parseLinkAt(text, i);
      if (lp) {
        flush();
        const inner = depth < MAX_LINK_DEPTH ? parseInline(lp.text, depth + 1, true) : [{ t: 'text', v: lp.text } as Inline];
        if (!inLink && isSafeHref(lp.url)) out.push({ t: 'link', href: cleanHref(lp.url), c: inner });
        else out.push(...inner);
        i = lp.end;
        continue;
      }
      buf += c; i++;
      continue;
    }

    if (c === '<' && !inLink) {
      const lim = Math.min(n, i + MAX_SCAN);
      let e = i + 1;
      while (e < lim && text[e] !== '>' && !/\s/.test(text[e]) && text[e] !== '<') e++;
      if (e < lim && text[e] === '>') {
        const url = text.slice(i + 1, e);
        if (/^(https?:\/\/|mailto:)/i.test(url) && isSafeHref(url)) {
          flush();
          out.push({ t: 'link', href: cleanHref(url), c: [{ t: 'text', v: url }] });
          i = e + 1;
          continue;
        }
      }
      buf += c; i++;
      continue;
    }

    if ((c === 'h' || c === 'H') && !inLink && !isWord(text[i - 1]) && /^https?:\/\//i.test(text.slice(i, i + 8))) {
      let e = i;
      while (e < n && !/\s/.test(text[e]) && text[e] !== '<') e++;
      let url = text.slice(i, e);
      for (;;) {
        const last = url[url.length - 1];
        if (last !== undefined && '.,;:!?\'"*_~]'.includes(last)) { url = url.slice(0, -1); continue; }
        if (last === ')') {
          const opens = url.split('(').length - 1;
          const closes = url.split(')').length - 1;
          if (closes > opens) { url = url.slice(0, -1); continue; }
        }
        break;
      }
      if (url.length > 8 && isSafeHref(url)) {
        flush();
        out.push({ t: 'link', href: cleanHref(url), c: [{ t: 'text', v: url }] });
        i += url.length;
        continue;
      }
      buf += c; i++;
      continue;
    }

    // teks biasa: majukan sampai karakter khusus berikutnya
    let j = i + 1;
    while (j < n && !'\\`*_~[!<\nhH'.includes(text[j])) j++;
    buf += text.slice(i, j);
    i = j;
  }
  flush();
  return out;
}

// dipakai agar plainText tidak dianggap tak terpakai oleh linter ketat
export function inlineToText(nodes: Inline[]): string {
  return plainText(nodes);
}
