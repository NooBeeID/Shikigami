import { call } from '../../../lib/ipc';

export type ConnectorState = 'cli_missing' | 'not_logged_in' | 'connected';
export interface ConnectorStatus {
  state: ConnectorState;
  user?: string;
  message?: string;
}
export interface ConnectorGuide {
  steps: string[];
  commands: string[];
}
export interface ConnectorInfo {
  id: string;
  name: string;
  status: ConnectorStatus;
}

export function listConnectors(): Promise<ConnectorInfo[]> {
  return call<ConnectorInfo[] | null>('connector.list').then((r) => r ?? []);
}

export function getConnectorStatus(id: string): Promise<ConnectorStatus> {
  return call<ConnectorStatus>('connector.status', { id });
}

export function getConnectorGuide(id: string): Promise<ConnectorGuide> {
  return call<ConnectorGuide>('connector.guide', { id }).then((g) => ({
    steps: g?.steps ?? [],
    commands: g?.commands ?? [],
  }));
}
