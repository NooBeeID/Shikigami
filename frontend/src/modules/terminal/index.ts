import { registerTabType } from '../../lib/registry';
import TerminalTab from './TerminalTab.svelte';

registerTabType({ type: 'session', component: TerminalTab });
