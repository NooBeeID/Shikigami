import { registerSettingsPage } from '../pages';
import AgentsPage from './AgentsPage.svelte';

// Halaman Agent; data baru dimuat saat halaman dibuka (onMount di AgentsPage), bukan saat startup.
registerSettingsPage({ id: 'agents', title: 'Agent', order: 15, component: AgentsPage });
