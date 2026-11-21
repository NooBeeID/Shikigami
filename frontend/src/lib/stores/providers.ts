import { writable, type Writable } from 'svelte/store';
import { call } from '../ipc';
import type { Provider } from './types';
import { normalizeList } from './util';

export const providers: Writable<Provider[]> = writable<Provider[]>([]);

// FINAL(shell-layout)
export async function loadProviders(): Promise<void> {
  const res = await call('provider.list');
  providers.set(normalizeList<Provider>(res, 'providers'));
}
