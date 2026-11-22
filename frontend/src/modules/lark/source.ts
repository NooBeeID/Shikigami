import { get } from 'svelte/store';
import { call } from '../../lib/ipc';
import { activeWorkspaceId } from '../../lib/stores/workspaces';

export type SourceStatus = 'ok' | 'missing' | 'error';
type SourceRef = { id: string };
type SourcesResp = { tickets?: SourceRef[]; wiki?: SourceRef[] };

export async function resolveSource(sourceId: string): Promise<SourceStatus> {
  if (sourceId === 'personal') return 'ok';
  const workspaceId = get(activeWorkspaceId);
  if (!workspaceId) return 'error';
  try {
    const r = await call<SourcesResp>('lark.sources.get', { workspaceId });
    const found = (r?.tickets ?? []).some((s) => s.id === sourceId) || (r?.wiki ?? []).some((s) => s.id === sourceId);
    return found ? 'ok' : 'missing';
  } catch {
    return 'error';
  }
}

// Memeriksa sumber saat dipasang dan setiap workspace aktif berganti; hasil basi dibuang.
export function watchSource(
  sourceId: () => string | undefined,
  onChange: (s: SourceStatus | 'checking') => void,
): () => void {
  let seq = 0;
  const run = () => {
    const id = sourceId();
    const mine = ++seq;
    if (!id) {
      onChange('missing');
      return;
    }
    onChange('checking');
    void resolveSource(id).then((st) => {
      if (mine === seq) onChange(st);
    });
  };
  const unsub = activeWorkspaceId.subscribe(run);
  return () => {
    seq++;
    unsub();
  };
}
