import { registerTabType } from '../../lib/registry';
import FileTab from './editor/FileTab.svelte';
import { registerFilesPanel } from './tree/registerTree';

registerTabType({ type: 'file', component: FileTab });
registerFilesPanel();
