import { call } from '../../../lib/ipc';

export type LarkError = { code: string; message: string };
export type WikiGetResult = {
  title: string; objType: string; url: string;
  markdown?: string; cells?: string[][]; truncated?: boolean;
};

export function parseLarkError(e: unknown): LarkError {
  if (e && typeof e === 'object' && !(e instanceof Error)) {
    const o = e as { code?: unknown; message?: unknown };
    if (typeof o.code === 'string' && typeof o.message === 'string') return { code: o.code, message: o.message };
  }
  const msg = e instanceof Error ? e.message : String(e);
  const m = /^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$/s.exec(msg);
  if (m) return { code: m[1], message: m[2] || msg };
  return { code: 'unknown', message: msg };
}

export async function getWikiNode(workspaceId: string, sourceId: string, nodeToken: string): Promise<WikiGetResult> {
  let r: WikiGetResult;
  try {
    r = await call<WikiGetResult>('lark.wiki.get', { workspaceId, sourceId, nodeToken });
  } catch (e) {
    throw parseLarkError(e);
  }
  const objType = r?.objType ?? '';
  return {
    ...r,
    title: r?.title ?? '',
    objType,
    url: r?.url ?? '',
    cells: objType === 'sheet' ? (r?.cells ?? []) : r?.cells,
    truncated: !!r?.truncated,
  };
}
