import { registerPanel, registerTabType } from '../../lib/registry';
import TerminalTab from './TerminalTab.svelte';
import QuickTerminal from './QuickTerminal.svelte';

registerTabType({ type: 'session', component: TerminalTab });
registerPanel({ id: 'terminal-quick', area: 'main-tab', component: QuickTerminal, order: 50 });
