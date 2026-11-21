import './drop.css';
import { registerPanel } from '../../lib/registry';
import { onMenuEvent } from '../appmenu';
import Toast from './Toast.svelte';
import { handleOpenFolder, handleNewWorkspaceFromFolder, handleDropped } from './logic';

registerPanel({ id: 'openfolder-toast', area: 'statusbar', component: Toast, order: 90 });
onMenuEvent('menu:open-folder', () => { void handleOpenFolder(); });
onMenuEvent('menu:new-workspace-from-folder', () => { void handleNewWorkspaceFromFolder(); });
onMenuEvent('files:dropped', (p) => { void handleDropped(p); });
