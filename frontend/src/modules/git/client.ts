import { call } from '../../lib/ipc';

export type Provider = 'gitlab' | 'github';
export interface RepoProject { id: string; name: string }
export interface Repo {
  projectId: string;
  projectName: string;
  projects: RepoProject[];
  provider: Provider;
  host: string;
  path: string;
  webUrl: string;
}
export type GitError = { code: string; message: string };

export function parseGitError(x: unknown): GitError {
  if (x && typeof x === 'object' && 'code' in x && 'message' in x && !(x instanceof Error)) {
    const o = x as { code: unknown; message: unknown };
    if (typeof o.code === 'string' && typeof o.message === 'string') return { code: o.code, message: o.message };
  }
  const msg = x instanceof Error ? x.message : String(x);
  const m = /^\[([a-z_]+)\]\s*(.*?)(?:\s*\(kode -?\d+\))?$/s.exec(msg);
  if (m) return { code: m[1], message: m[2] || msg };
  return { code: 'unknown', message: msg };
}

export async function listRepos(workspaceId: string): Promise<Repo[]> {
  let r: Repo[] | null;
  try {
    r = await call<Repo[] | null>('git.repos.list', { workspaceId });
  } catch (e) {
    throw parseGitError(e);
  }
  return (Array.isArray(r) ? r : []).map((x) => ({
    ...x,
    projects:
      Array.isArray(x.projects) && x.projects.length > 0 ? x.projects : [{ id: x.projectId, name: x.projectName }],
  }));
}
