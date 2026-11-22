import ResourceMeter from './ResourceMeter.svelte';
import { registerTopbarItem } from '../shell';
import { startResourceSync } from './store';
import { inWails } from '../../lib/ipc';
export { ResourceMeter };
export * from './store';
if (inWails) {
  registerTopbarItem({ id: 'resource', component: ResourceMeter, align: 'right', order: 20 });
  startResourceSync();
}
