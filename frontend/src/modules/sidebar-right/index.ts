import { registerRightTab } from '../shell';
import FilesTab from './FilesTab.svelte';

registerRightTab({ id: 'files', title: 'Files', component: FilesTab, order: 10 });

export { FilesTab };
