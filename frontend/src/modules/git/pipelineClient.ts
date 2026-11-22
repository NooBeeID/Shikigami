import { call } from '../../lib/ipc';
import { parseGitError } from './client';

export type PipelineStatus = 'pending' | 'running' | 'success' | 'failed' | 'canceled' | 'skipped' | 'unknown';

export interface Pipeline {
  id: string;
  ref: string;
  sha: string;
  status: PipelineStatus;
  author?: string;
  durationSec?: number;
  updatedAt: string;
  url: string;
}

export interface PipelineJob {
  name: string;
  status: PipelineStatus;
  durationSec?: number;
  url?: string;
}

export interface PipelineStage {
  name: string;
  jobs: PipelineJob[];
}

export interface PipelineDetail extends Pipeline {
  stages: PipelineStage[];
}

export async function listPipelines(params: { workspaceId: string; projectId: string }): Promise<Pipeline[]> {
  let r: Pipeline[] | null | undefined;
  try {
    r = await call<Pipeline[] | null>('git.pipelines.list', params);
  } catch (e) {
    throw parseGitError(e);
  }
  return Array.isArray(r) ? r : [];
}

export async function getPipeline(params: { workspaceId: string; projectId: string; id: string }): Promise<PipelineDetail> {
  let r: PipelineDetail;
  try {
    r = await call<PipelineDetail>('git.pipeline.get', params);
  } catch (e) {
    throw parseGitError(e);
  }
  return { ...r, stages: Array.isArray(r.stages) ? r.stages : [] };
}
