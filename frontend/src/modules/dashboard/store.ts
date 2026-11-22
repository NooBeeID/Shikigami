import { derived, type Readable } from 'svelte/store';
import { sessions, providers, activeWorkspaceId, activeWorkspace } from '../../lib/stores';
import { statusStore } from '../status/store';
import { activityEntries } from '../activity/store';
import { sessionTitle, basename } from '../shell/helpers';
import { STATUS_ORDER, type AgentRow, type AgentStatus } from './types';

const STATUS_RANK = Object.fromEntries(STATUS_ORDER.map((s, i) => [s, i])) as Record<AgentStatus, number>;

// Keempat kunci selalu ada, urutan input dipertahankan; fungsi murni.
export function groupRows(rows: AgentRow[]): Record<AgentStatus, AgentRow[]> {
  const out = {} as Record<AgentStatus, AgentRow[]>;
  for (const s of STATUS_ORDER) out[s] = [];
  for (const r of rows) out[r.status].push(r);
  return out;
}

export const agentRows: Readable<AgentRow[]> = derived(
  [sessions, statusStore, activityEntries, activeWorkspace, providers, activeWorkspaceId],
  ([$sessions, $status, $entries, $ws, $providers, $wsId]) => {
    if (!$wsId) return [];
    // Entri aktivitas terbaru lebih dulu: ambil yang pertama per sesi dalam satu lintasan.
    const lastBySession = new Map<string, string>();
    for (const e of $entries) if (!lastBySession.has(e.sessionId)) lastBySession.set(e.sessionId, e.summary);
    const projectNames = new Map<string, string>();
    for (const p of $ws?.projects ?? []) projectNames.set(p.id, p.name);
    const providerNames = new Map<string, string>();
    for (const p of $providers) providerNames.set(p.id, p.name);

    const rows: AgentRow[] = [];
    for (const s of $sessions) {
      if (s.providerId === 'shell' || s.workspaceId !== $wsId) continue;
      const st = $status[s.id];
      const status: AgentStatus = st ? st.status : s.status === 'exited' ? 'done' : 'idle';
      rows.push({
        sessionId: s.id,
        title: sessionTitle(s.id) || providerNames.get(s.providerId) || s.providerId,
        providerId: s.providerId,
        projectName: projectNames.get(s.projectId) ?? basename(s.cwd),
        status,
        since: st?.since ?? (Date.parse(s.createdAt) || 0),
        reason: st?.reason || undefined,
        lastActivity: lastBySession.get(s.id) || undefined,
      });
    }
    // waiting naik (terlama menunggu dulu); kelompok lain menurun (terbaru dulu).
    rows.sort((a, b) => STATUS_RANK[a.status] - STATUS_RANK[b.status] || (a.status === 'waiting' ? a.since - b.since : b.since - a.since));
    return rows;
  },
);

export const waitingCount: Readable<number> = derived(agentRows, ($r) => $r.filter((r) => r.status === 'waiting').length);
