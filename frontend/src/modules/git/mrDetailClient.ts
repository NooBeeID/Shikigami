import { call } from '../../lib/ipc';
import { parseGitError } from './client';
import type { MergeRequest } from './mrClient';

export type MergeableState = 'mergeable' | 'conflict' | 'unknown';
export type ReviewerState = 'approved' | 'changes_requested' | 'pending';
export interface MrReviewer { name: string; state: string }
export interface MrLastPipeline { id: string; status: string; url?: string }
export interface MergeRequestDetail extends MergeRequest {
  description: string;
  labels: string[];
  reviewers: MrReviewer[];
  mergeable?: MergeableState;
  lastPipeline?: MrLastPipeline;
}

export async function getMrDetail(workspaceId: string, projectId: string, number: number): Promise<MergeRequestDetail> {
  let r: MergeRequestDetail | null;
  try {
    r = await call<MergeRequestDetail | null>('git.mr.get', { workspaceId, projectId, number });
  } catch (e) {
    throw parseGitError(e);
  }
  if (!r || typeof r !== 'object') {
    throw { code: 'not_found', message: 'MR tidak ditemukan' };
  }
  return {
    ...r,
    description: typeof r.description === 'string' ? r.description : '',
    labels: Array.isArray(r.labels) ? r.labels : [],
    reviewers: Array.isArray(r.reviewers) ? r.reviewers : [],
  };
}
