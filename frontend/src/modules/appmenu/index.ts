import { on, emit } from './bus';
import type { MenuEventName, MenuPayloads } from './types';

export { getAppConfig, setAppConfig, pickDirectory } from './bridge';
export type { MenuEventName, MenuPayloads, AppConfig } from './types';

const ALL_EVENTS: MenuEventName[] = [
  'menu:zoom',
  'menu:open-folder',
  'menu:new-workspace-from-folder',
  'menu:toggle-sidebar-left',
  'menu:toggle-sidebar-right',
  'files:dropped',
];

type PayloadOf<K extends MenuEventName> = K extends keyof MenuPayloads ? MenuPayloads[K] : undefined;

export function onMenuEvent<K extends MenuEventName>(name: K, fn: (p: PayloadOf<K>) => void): () => void {
  return on(name, fn);
}

export function emitMenuEvent<K extends MenuEventName>(name: K, p?: PayloadOf<K>): void {
  emit(name, p);
}

const rt = (window as any).runtime;
if (rt?.EventsOn) {
  for (const name of ALL_EVENTS) {
    rt.EventsOn(name, (payload: unknown) => emit(name, payload));
  }
}
