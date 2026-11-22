import { call } from '../../lib/ipc';
import { parseGitError } from './client';

export type MergeRequestState = 'open' | 'merged' | 'closed';

export interface MergeRequest {
  number: number;
  title: string;
  author: string;
  sourceBranch: string;
  targetBranch: string;
  state: MergeRequestState;
  draft: boolean;
  reviewState?: string;
  pipelineStatus?: string;
  updatedAt: string;
  url: string;
}

export async function listMergeRequests(params: {
  workspaceId: string;
  projectId: string;
  state: MergeRequestState;
}): Promise<MergeRequest[]> {
  let r: { mrs: MergeRequest[] | null } | null | undefined;
  try {
    r = await call<{ mrs: MergeRequest[] | null }>('git.mrs.list', params);
  } catch (e) {
    throw parseGitError(e);
  }
  return Array.isArray(r?.mrs) ? r!.mrs! : [];
}
