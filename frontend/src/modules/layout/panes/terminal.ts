import { get } from 'svelte/store';
import { registerPaneKind } from '../registry';
import type { Leaf } from '../model';
import { killSession, sessions } from '../../../lib/stores/sessions';
import { providers } from '../../../lib/stores/providers';
import TerminalPane from './TerminalPane.svelte';

function providerIdOf(leaf: Leaf): string | undefined {
  return leaf.provider ?? get(sessions).find((s) => s.id === leaf.sessionId)?.providerId;
}

const BADGES: Record<string, string> = { claude: 'CL', codex: 'CX', shell: 'SH' };

registerPaneKind({
  kind: 'terminal',
  component: TerminalPane,
  titleFor: (leaf) => {
    const name = get(providers).find((p) => p.id === providerIdOf(leaf))?.name;
    return name || leaf.title || 'Terminal';
  },
  badgeFor: (leaf) => {
    const id = providerIdOf(leaf);
    if (!id) return null;
    return BADGES[id] ?? id.slice(0, 2).toUpperCase();
  },
  onEndSession: async (leaf) => {
    if (leaf.sessionId) await killSession(leaf.sessionId);
  },
});
