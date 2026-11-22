import { get } from 'svelte/store';
import { registerPaneKind } from '../registry';
import type { Tab } from '../model';
import { killSession, sessions } from '../../../lib/stores/sessions';
import { providers } from '../../../lib/stores/providers';
import TerminalPane from './TerminalPane.svelte';

function providerIdOf(tab: Tab): string | undefined {
  return tab.provider ?? get(sessions).find((s) => s.id === tab.sessionId)?.providerId;
}

const BADGES: Record<string, string> = { claude: 'CL', codex: 'CX', shell: 'SH' };

registerPaneKind({
  kind: 'terminal',
  component: TerminalPane,
  titleFor: (tab) => {
    const name = get(providers).find((p) => p.id === providerIdOf(tab))?.name;
    return name || tab.title || 'Terminal';
  },
  badgeFor: (tab) => {
    const id = providerIdOf(tab);
    if (!id) return null;
    return BADGES[id] ?? id.slice(0, 2).toUpperCase();
  },
  onEndSession: async (tab) => {
    if (tab.sessionId) await killSession(tab.sessionId);
  },
});
