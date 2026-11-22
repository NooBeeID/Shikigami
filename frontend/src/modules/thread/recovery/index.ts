import { registerThreadSlot } from '../slots';
import RecoveryPanel from './RecoveryPanel.svelte';
import RecoveryNotice from './RecoveryNotice.svelte';
import DeadBadge from './DeadBadge.svelte';

export { DeadBadge, RecoveryPanel, RecoveryNotice };
export { isRecoveryMessage, deadParticipants } from './util';

let stopper: (() => void) | null = null;

// Idempoten: panggilan kedua mengembalikan pembersih yang sama.
export function installRecovery(): () => void {
  if (stopper) return stopper;
  registerThreadSlot({ slot: 'side-panel', component: RecoveryPanel, order: 20, id: 'recovery-panel' });
  registerThreadSlot({ slot: 'message-extra', component: RecoveryNotice, order: 20, id: 'recovery-notice' });
  const stop = () => { if (stopper === stop) stopper = null; };
  stopper = stop;
  return stop;
}
