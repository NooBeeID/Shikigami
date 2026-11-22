import { call } from '../../lib/ipc';

export type LarkError = { code: string; message: string };
export type WikiNode = { token: string; title: string; objType: string; objToken: string; hasChild: boolean };
export type WikiListResult = { nodes: WikiNode[]; hasMore?: boolean; hidden?: boolean; hiddenReason?: string };

export function parseLarkError(e: unknown): LarkError {
  if (e && typeof e === 'object' && 'code' in e && 'message' in e && typeof (e as any).code === 'string' && !(e instanceof Error)) {
    return e as LarkError;
  }
  const raw = e instanceof Error ? e.message : String(e);
  const m = /^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$/s.exec(raw);
  if (m) return { code: m[1], message: m[2] || m[1] };
  return { code: 'unknown', message: raw };
}

export async function listWikiNodes(workspaceId: string, sourceId: string, parentToken?: string): Promise<WikiListResult> {
  try {
    const r = await call<WikiListResult>('lark.wiki.list', { workspaceId, sourceId, parentToken: parentToken || undefined });
    return { ...r, nodes: r?.nodes ?? [] };
  } catch (e) {
    throw parseLarkError(e);
  }
}
