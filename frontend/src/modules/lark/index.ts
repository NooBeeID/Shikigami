import { registerPaneKind } from '../layout/registry';
import BoardPane from './board/BoardPane.svelte';
import DocPane from './doc/DocPane.svelte';

// --- lark-pane-foundation: registrasi pane dan ekspor ---
registerPaneKind({ kind: 'lark-board', component: BoardPane, titleFor: (leaf) => leaf.title || 'Tiket Lark' });
registerPaneKind({ kind: 'lark-doc', component: DocPane, titleFor: (leaf) => leaf.title || 'Dokumen Lark' });
export { resolveSource } from './source';
export type { SourceStatus } from './source';
// --- akhir lark-pane-foundation ---

// --- lark-sources-wiring: item rail Lark ---
import { derived, type Readable } from 'svelte/store';
import { registerRailItem } from '../shell/registry';
import { connectors, loadConnectors } from '../settings/connectors/store';
import LarkPanel from './LarkPanel.svelte';
import LarkIcon from './LarkIcon.svelte';

// true bila connector lark ada dan state bukan cli_missing; true bila daftar belum dimuat.
const larkRailVisible: Readable<boolean> = derived(connectors, ($list) => {
  if ($list.length === 0) return true;
  const lark = $list.find((c) => c.id === 'lark');
  if (!lark) return false;
  return lark.status?.state !== 'cli_missing';
});

registerRailItem({ id: 'lark', title: 'Lark', icon: LarkIcon, component: LarkPanel, order: 10, visible: larkRailVisible });
void loadConnectors();
// --- akhir lark-sources-wiring ---
