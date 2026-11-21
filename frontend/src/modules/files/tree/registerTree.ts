import { registerPanel } from '../../../lib/registry';
import FilesPanel from './FilesPanel.svelte';

export function registerFilesPanel(): void {
  registerPanel({ id: 'files', area: 'sidebar', component: FilesPanel, order: 20 });
}
