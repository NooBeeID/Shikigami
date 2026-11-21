import { registerTabType } from '../../lib/registry';
import TerminalTab from './TerminalTab.svelte';

registerTabType({ type: 'session', component: TerminalTab });
export { setTerminalFontScale, currentTerminalFontSize, registerTerminalInstance } from './font-scale';
