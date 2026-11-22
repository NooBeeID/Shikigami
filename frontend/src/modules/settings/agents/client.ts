import { call } from '../../../lib/ipc';

export interface Category {
  id: string;
  name: string;
  role: string;
  builtin: boolean;
}
export interface Agent {
  id: string;
  name: string;
  categoryId: string;
  providerId: string;
  model: string;
  isDefault: boolean;
}
export interface AgentsSnapshot {
  categories: Category[];
  agents: Agent[];
}
export interface CategoryUpsertInput {
  id?: string;
  name: string;
  role: string;
}
export interface AgentUpsertInput {
  id?: string;
  name: string;
  categoryId: string;
  providerId: string;
  model: string;
  isDefault: boolean;
}
export interface CategoryDeleteInput {
  id: string;
  agents?: 'move' | 'delete';
  moveTo?: string;
}

// Galat backend (bahasa Indonesia) diteruskan apa adanya.
export function listAgents(): Promise<AgentsSnapshot> {
  return call<AgentsSnapshot | null>('agents.list').then((r) => ({
    categories: r?.categories ?? [],
    agents: r?.agents ?? [],
  }));
}

export function upsertCategory(input: CategoryUpsertInput): Promise<Category> {
  return call<Category>('agents.categoryUpsert', input);
}

export function deleteCategory(input: CategoryDeleteInput): Promise<void> {
  return call('agents.categoryDelete', input).then(() => undefined);
}

export function upsertAgent(input: AgentUpsertInput): Promise<Agent> {
  return call<Agent>('agents.upsert', input);
}

export function deleteAgent(id: string): Promise<void> {
  return call('agents.delete', { id }).then(() => undefined);
}

export function setDefaultAgent(id: string): Promise<void> {
  return call('agents.setDefault', { id }).then(() => undefined);
}
