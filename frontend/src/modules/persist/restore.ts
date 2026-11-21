import { call } from '../../lib/ipc';
import { layout } from '../../lib/stores/layout';
import {
  leftSidebar,
  rightSidebar,
  LEFT_MIN,
  LEFT_MAX,
  LEFT_DEFAULT,
  RIGHT_MIN,
  RIGHT_MAX,
  RIGHT_DEFAULT,
} from '../../lib/stores/shell';
import { loadSessions } from '../../lib/stores/sessions';
import { normalizeList } from '../../lib/stores/util';
import { setBaseline } from './autosave';

function clamp(v: unknown, min: number, max: number, def: number): number {
  const n = typeof v === 'number' ? v : Number(v);
  if (!Number.isFinite(n) || n === 0) return def;
  return Math.round(Math.min(max, Math.max(min, n)));
}

export function applyUi(ui: unknown): void {
  if (!ui || typeof ui !== 'object') return;
  const o = ui as Record<string, any>;
  const l = o.leftSidebar && typeof o.leftSidebar === 'object' ? o.leftSidebar : {};
  const r = o.rightSidebar && typeof o.rightSidebar === 'object' ? o.rightSidebar : {};
  leftSidebar.set({
    visible: typeof l.visible === 'boolean' ? l.visible : true,
    width: clamp(l.width, LEFT_MIN, LEFT_MAX, LEFT_DEFAULT),
  });
  rightSidebar.set({
    visible: typeof r.visible === 'boolean' ? r.visible : true,
    width: clamp(r.width, RIGHT_MIN, RIGHT_MAX, RIGHT_DEFAULT),
    activeTab: typeof r.activeTab === 'string' && r.activeTab ? r.activeTab : 'files',
  });
}

// Tipis: hanya kloning + penjagaan bentuk. Tidak ada daun yang dibuang
// (sesi mati / berkas hilang ditangani overlay dan viewer masing-masing).
export function sanitizeDoc(raw: unknown, _wsId: string, _liveIds: Set<string>): unknown {
  if (!raw || typeof raw !== 'object') return raw;
  try {
    return JSON.parse(JSON.stringify(raw));
  } catch {
    return raw;
  }
}

export async function restoreWorkspace(wsId: string | null): Promise<void> {
  if (!wsId) {
    layout.hydrate(null);
    setBaseline(null, '');
    return;
  }
  const res = await call<{ workspaceId: string; groupId: string; layout: unknown | null }>('workspace.layout.get', {
    workspaceId: wsId,
  });
  const liveIds = new Set<string>();
  try {
    const sl = await call('session.list');
    for (const s of normalizeList<{ id: string; status: string }>(sl, 'sessions')) {
      if (s && s.status === 'running') liveIds.add(s.id);
    }
  } catch {
    // diabaikan
  }
  try {
    await loadSessions();
  } catch {
    // diabaikan
  }
  const doc = sanitizeDoc(res?.layout ?? null, wsId, liveIds);
  layout.hydrate(doc);
  setBaseline(wsId, res?.groupId ?? '');
}
