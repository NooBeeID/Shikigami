import { registerPaneKind } from '../layout/registry';
import { registerRailItem } from '../shell/registry';
import { registerTabType } from '../../lib/registry';
import ThreadPane from './ThreadPane.svelte';
import ThreadSection from './ThreadSection.svelte';
import ThreadTab from './ThreadTab.svelte';
import { ThreadRailIcon } from './rail-icon';
import { subscribeThreadEvents } from './thread-store';
import { installParticipants } from './participants';
import { installClose } from './close';
import { installRecovery } from './recovery';

registerPaneKind({ kind: 'thread', component: ThreadPane, titleFor: (leaf) => leaf.title || 'Thread' });
// Slot order rail (dicadangkan): agents=0, lark=10, git-github=20, git-gitlab=21, threads=30.
registerRailItem({ id: 'threads', title: 'Thread', icon: ThreadRailIcon, component: ThreadSection, order: 30 });
registerTabType({ type: 'thread', component: ThreadTab }); // jalur legacy; tidak dirender siapa pun saat ini
subscribeThreadEvents();
installParticipants();
installClose();
installRecovery();

export { registerThreadSlot } from './slots';
export type { ThreadSlotName } from './slots';
export { threads, messagesByThread, openThreadPane, participantName } from './thread-store';
export * from './types';
export { avatarColor, initials, formatTime } from './format';
