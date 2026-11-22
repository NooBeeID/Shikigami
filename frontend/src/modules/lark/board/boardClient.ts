import { call } from '../../../lib/ipc';

export const FIELD_TYPE_SINGLE_SELECT = 'single_select';

export type TicketSource = { id: string; name: string; baseToken: string; host: string; tableId?: string; viewId?: string };
export type LarkError = { code: string; message: string };
export type BoardField = { id: string; name: string; type: string; isPrimary: boolean; options?: string[] };
export type BoardRecord = { id: string; title: string; values: Record<string, string> };
export type TicketsPage = { fields: BoardField[]; records: BoardRecord[]; nextOffset?: number };
export type TicketDetail = { fields: { name: string; text: string }[]; url?: string; recordLinkAvailable: boolean };
export type TableInfo = { id: string; name: string };
export type ViewInfo = { id: string; name: string; type?: string };

export function parseLarkError(e: unknown): LarkError {
  const raw = e instanceof Error ? e.message : String(e);
  const m = /^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$/s.exec(raw);
  if (m) return { code: m[1], message: m[2] };
  return { code: 'unknown', message: raw };
}

async function wrap<T>(method: string, params: unknown): Promise<T> {
  try {
    return await call<T>(method, params);
  } catch (e) {
    throw parseLarkError(e);
  }
}

export async function listTickets(p: { workspaceId: string; sourceId: string; tableId: string; viewId?: string; offset: number }): Promise<TicketsPage> {
  const r = await wrap<Partial<TicketsPage> | null>('lark.tickets.list', p);
  return {
    fields: r?.fields ?? [],
    records: (r?.records ?? []).map((x) => ({ ...x, values: x.values ?? {} })),
    nextOffset: r?.nextOffset ?? undefined,
  };
}

export async function getTicket(p: { workspaceId: string; sourceId: string; recordId: string; tableId: string; viewId?: string }): Promise<TicketDetail> {
  const r = await wrap<Partial<TicketDetail> | null>('lark.ticket.get', p);
  return { fields: r?.fields ?? [], url: r?.url || undefined, recordLinkAvailable: !!r?.recordLinkAvailable };
}

export async function listTables(p: { workspaceId: string; sourceId: string }): Promise<TableInfo[]> {
  const r = await wrap<{ tables?: TableInfo[] } | null>('lark.tables.list', p);
  return r?.tables ?? [];
}

export async function listViews(p: { workspaceId: string; sourceId: string; tableId: string }): Promise<ViewInfo[]> {
  const r = await wrap<{ views?: ViewInfo[] } | null>('lark.views.list', p);
  return r?.views ?? [];
}

// ---- Tambahan aditif lark-board-ui ----
export type ViewMode = 'grid' | 'kanban';
export const MAX_EXTRA_FIELDS = 2;
export const NO_VALUE_LABEL = 'Tanpa nilai';

// Normalisasi galat apa pun menjadi LarkError.
export function toLarkError(e: unknown): LarkError {
  if (e && typeof e === 'object') {
    const o = e as { code?: unknown; message?: unknown };
    if (typeof o.code === 'string' && typeof o.message === 'string') return { code: o.code, message: o.message };
  }
  return parseLarkError(e);
}

export function isSingleSelect(f: BoardField): boolean {
  return f.type === FIELD_TYPE_SINGLE_SELECT;
}

export function primaryField(fields: BoardField[]): BoardField | undefined {
  return fields.find((f) => f.isPrimary) ?? fields[0];
}

// Field group default untuk kanban: "Status" bila ada, jika tidak single-select pertama.
export function defaultGroupField(candidates: BoardField[]): BoardField | undefined {
  return candidates.find((f) => f.name.trim().toLowerCase() === 'status') ?? candidates[0];
}

export function singleSelectFields(fields: BoardField[]): BoardField[] {
  return fields.filter(isSingleSelect);
}

export function eligibleExtraFields(fields: BoardField[]): BoardField[] {
  const p = primaryField(fields);
  return fields.filter((f) => f !== p);
}

// Cari lokal terhadap title dan semua nilai.
export function filterRecords(records: BoardRecord[], q: string): BoardRecord[] {
  const needle = q.trim().toLowerCase();
  if (!needle) return records;
  return records.filter(
    (r) => r.title.toLowerCase().includes(needle) || Object.values(r.values).some((v) => (v ?? '').toLowerCase().includes(needle)),
  );
}

// Gabung halaman berikutnya; buang duplikat id.
export function mergeRecords(existing: BoardRecord[], incoming: BoardRecord[]): BoardRecord[] {
  const seen = new Set(existing.map((r) => r.id));
  const out = existing.slice();
  for (const r of incoming) {
    if (seen.has(r.id)) continue;
    seen.add(r.id);
    out.push(r);
  }
  return out;
}

// Link tingkat Base/view: https://<host>/base/<baseToken>?table=<tableId>&view=<viewId>
// Sesuai docs/lark/cli-findings.md: pola ?table= terbukti; view ikut ditambahkan bila ada.
export function buildBaseLink(source: Pick<TicketSource, 'host' | 'baseToken'>, tableId: string, viewId?: string): string {
  const host = (source.host ?? '').trim().replace(/^https?:\/\//i, '').replace(/\/+$/, '');
  const token = (source.baseToken ?? '').trim();
  if (!host || !token) return '';
  const qs: string[] = [];
  if (tableId) qs.push('table=' + encodeURIComponent(tableId));
  if (viewId) qs.push('view=' + encodeURIComponent(viewId));
  return `https://${host}/base/${encodeURIComponent(token)}` + (qs.length ? '?' + qs.join('&') : '');
}
