import { writable } from 'svelte/store';
import {
  getConnectorStatus,
  listConnectors,
  type ConnectorInfo,
  type ConnectorStatus,
} from './client';

export const connectors = writable<ConnectorInfo[]>([]);
export const connectorsLoading = writable<boolean>(false);
export const connectorsError = writable<string | null>(null);

let loadSeq = 0;

export async function loadConnectors(): Promise<void> {
  const seq = ++loadSeq;
  connectorsLoading.set(true);
  connectorsError.set(null);
  try {
    const list = await listConnectors();
    if (seq === loadSeq) connectors.set(list);
  } catch (e) {
    if (seq === loadSeq) connectorsError.set(e instanceof Error ? e.message : String(e));
  } finally {
    if (seq === loadSeq) connectorsLoading.set(false);
  }
}

export async function refreshConnector(id: string): Promise<ConnectorStatus> {
  const status = await getConnectorStatus(id);
  connectors.update((list) => list.map((c) => (c.id === id ? { ...c, status } : c)));
  return status;
}
