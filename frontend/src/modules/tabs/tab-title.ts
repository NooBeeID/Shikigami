import type { Tab, Session, Provider, Workspace } from '../../lib/stores';

function basename(p: string): string {
  const trimmed = p.replace(/\/+$/, '');
  const seg = trimmed.split('/').pop() ?? '';
  return seg || p;
}

export function tabLabel(
  tab: Tab,
  sessions: Session[],
  providers: Provider[],
  workspace: Workspace | null,
): { title: string; subtitle: string; exited: boolean } {
  const fallback = { title: tab.title || tab.refId, subtitle: '', exited: false };
  if (tab.type !== 'session') return fallback;
  const s = sessions.find((x) => x.id === tab.refId);
  if (!s) return fallback;
  const title = providers.find((p) => p.id === s.providerId)?.name ?? s.providerId;
  const project = workspace?.projects.find((p) => p.id === s.projectId);
  return { title, subtitle: project?.name ?? basename(s.cwd), exited: s.status === 'exited' };
}
