import { connection } from '../../lib/stores/init';
import { getPaneKind, registerPaneKind } from '../layout/registry';
import '../layout/panes/terminal';
import TerminalGuard from './TerminalGuard.svelte';
import { startPersist } from './index';

// Bungkus jenis pane terminal dengan guard sesi mati.
const orig = getPaneKind('terminal');
if (orig) registerPaneKind({ ...orig, component: TerminalGuard });

// Mulai pemulihan sekali saat koneksi pertama kali ok.
let started = false;
let unsub: (() => void) | null = null;
unsub = connection.subscribe((c) => {
  if (started || c.state !== 'ok') return;
  started = true;
  queueMicrotask(() => {
    unsub?.();
    try {
      void startPersist().catch(() => {});
    } catch {
      // tidak pernah melempar
    }
  });
});
