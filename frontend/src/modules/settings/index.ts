import { registerPanel } from '../../lib/registry';
import { registerSettingsPage } from './pages';
import ConnectorsPlaceholder from './ConnectorsPlaceholder.svelte';
import McpPage from './McpPage.svelte';
import SettingsModal from './SettingsModal.svelte';

// Tiket 02 menimpa halaman `connectors` lewat registerSettingsPage (id sama).
registerSettingsPage({ id: 'connectors', title: 'Connector', order: 10, component: ConnectorsPlaceholder });
registerSettingsPage({ id: 'mcp', title: 'MCP', order: 20, component: McpPage });
registerPanel({ id: 'settings-modal', area: 'statusbar', order: 95, component: SettingsModal });

export { openSettings, closeSettings, settingsOpen, activePage } from './store';
export { registerSettingsPage, settingsPages } from './pages';
export type { SettingsPageSpec } from './pages';
export { default as Modal } from './Modal.svelte';
export { default as SettingsModal } from './SettingsModal.svelte';
