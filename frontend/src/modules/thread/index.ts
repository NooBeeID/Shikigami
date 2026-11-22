import { registerPaneKind } from '../layout/registry';
import { registerSidebarSection } from '../sidebar-left/registry';
import { registerTabType } from '../../lib/registry';
import ThreadPane from './ThreadPane.svelte';
import ThreadSection from './ThreadSection.svelte';
import ThreadTab from './ThreadTab.svelte';
import { subscribeThreadEvents } from './thread-store';
import { installParticipants } from './participants';

registerPaneKind({ kind: 'thread', component: ThreadPane, titleFor: (leaf) => leaf.title || 'Thread' });
registerSidebarSection({ id: 'thread-sidebar', component: ThreadSection, order: 0 });
registerTabType({ type: 'thread', component: ThreadTab }); // jalur legacy; tidak dirender siapa pun saat ini
subscribeThreadEvents();
installParticipants();

export { registerThreadSlot } from './slots';
export type { ThreadSlotName } from './slots';
export { threads, messagesByThread, openThreadPane, participantName } from './thread-store';
export * from './types';
export { avatarColor, initials, formatTime } from './format';
