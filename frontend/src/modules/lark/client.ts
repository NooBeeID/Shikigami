import { call } from '../../lib/ipc';

export type TicketSource = { id: string; name: string; baseToken: string; host: string; tableId?: string; viewId?: string };
export type WikiSource = { id: string; name: string; spaceId: string; rootToken?: string; host: string };
export type LarkConfig = { tickets: TicketSource[]; wiki: WikiSource[] };
export type LarkError = { code: string; message: string };
export type ParsedLink = {
  kind: 'base' | 'wiki';
  host: string;
  baseToken?: string;
  tableId?: string;
  viewId?: string;
  spaceId?: string;
  nodeToken?: string;
  defaultName?: string;
};
export type LarkTable = { id: string; name: string };
export type LarkView = { id: string; name: string; type?: string };
export type AddSourceParams =
  | { kind: 'ticket'; name: string; baseToken: string; host: string; tableId?: string; viewId?: string }
  | { kind: 'wiki'; name: string; spaceId: string; rootToken?: string; host: string };

export function parseLarkError(e: unknown): LarkError {
  if (e && typeof e === 'object' && 'code' in e && 'message' in e && !(e instanceof Error)) {
    const o = e as { code: unknown; message: unknown };
    if (typeof o.code === 'string' && typeof o.message === 'string') return { code: o.code, message: o.message };
  }
  const msg = e instanceof Error ? e.message : String(e);
  const m = /^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$/s.exec(msg);
  if (m) return { code: m[1], message: m[2] || msg };
  return { code: 'unknown', message: msg };
}

function wrap<T>(method: string, params: unknown): Promise<T> {
  return call<T>(method, params).catch((e) => {
    throw parseLarkError(e);
  });
}

export async function larkSourcesGet(workspaceId: string): Promise<LarkConfig> {
  const r = await wrap<Partial<LarkConfig> | null>('lark.sources.get', { workspaceId });
  return { tickets: r?.tickets ?? [], wiki: r?.wiki ?? [] };
}

export function larkSourceAdd(workspaceId: string, p: AddSourceParams): Promise<TicketSource | WikiSource> {
  return wrap<TicketSource | WikiSource>('lark.sources.add', { workspaceId, ...p });
}

export async function larkSourceRemove(workspaceId: string, sourceId: string): Promise<void> {
  await wrap('lark.sources.remove', { workspaceId, sourceId });
}

export async function larkSourceRename(workspaceId: string, sourceId: string, name: string): Promise<void> {
  await wrap('lark.sources.rename', { workspaceId, sourceId, name });
}

export function larkLinkParse(link: string): Promise<ParsedLink> {
  return wrap<ParsedLink>('lark.link.parse', { link });
}

export async function larkTablesList(workspaceId: string, sourceId: string): Promise<LarkTable[]> {
  const r = await wrap<{ tables?: LarkTable[] } | null>('lark.tables.list', { workspaceId, sourceId });
  return r?.tables ?? [];
}

export async function larkViewsList(workspaceId: string, sourceId: string, tableId: string): Promise<LarkView[]> {
  const r = await wrap<{ views?: LarkView[] } | null>('lark.views.list', { workspaceId, sourceId, tableId });
  return r?.views ?? [];
}
