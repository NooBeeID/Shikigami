import type { Session, Project } from '../../lib/stores';

export interface SessionGroup {
  projectId: string;
  name: string;
  sessions: Session[];
}

export function groupSessionsByProject(
  sessions: Session[],
  projects: Project[],
  workspaceId: string | null,
): SessionGroup[] {
  const mine = sessions
    .filter((s) => s.workspaceId === workspaceId)
    .sort((a, b) => (a.createdAt < b.createdAt ? -1 : a.createdAt > b.createdAt ? 1 : 0));
  const known = new Set(projects.map((p) => p.id));
  const groups: SessionGroup[] = projects.map((p) => ({
    projectId: p.id,
    name: p.name,
    sessions: mine.filter((s) => s.projectId === p.id),
  }));
  const orphans = new Map<string, Session[]>();
  for (const s of mine) {
    if (known.has(s.projectId)) continue;
    const arr = orphans.get(s.projectId) ?? [];
    arr.push(s);
    orphans.set(s.projectId, arr);
  }
  for (const [projectId, list] of orphans) {
    groups.push({ projectId, name: 'Project lain', sessions: list });
  }
  return groups;
}
