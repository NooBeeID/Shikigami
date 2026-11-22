import { registerThreadSlot } from '../slots';
import { registerSessionFilter } from '../../sidebar-left';
import ParticipantsPanel from './ParticipantsPanel.svelte';
import MessageExtra from './MessageExtra.svelte';
import { startSessionSync, threadSessionPredicate } from './session-filter';

export { isThreadSession, threadSessionIds, threadSessionPredicate } from './session-filter';
export { openParticipantTerminal } from './terminal';

let stopper: (() => void) | null = null;

// Idempoten: panggilan kedua mengembalikan pembersih yang sama.
export function installParticipants(): () => void {
  if (stopper) return stopper;
  registerThreadSlot({ slot: 'side-panel', component: ParticipantsPanel, order: 0, id: 'participants' });
  registerThreadSlot({ slot: 'message-extra', component: MessageExtra, id: 'participants-msg' });
  registerSessionFilter({ id: 'thread-sessions', predicate: threadSessionPredicate });
  const stopSync = startSessionSync();
  const stop = () => {
    stopSync();
    if (stopper === stop) stopper = null;
  };
  stopper = stop;
  return stop;
}
