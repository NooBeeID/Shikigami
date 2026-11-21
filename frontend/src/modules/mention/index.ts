import { registerPanel } from '../../lib/registry';
import MentionComposer from './MentionComposer.svelte';
import QueueBadge from './QueueBadge.svelte';

registerPanel({ id: 'mention-composer', area: 'sidebar', component: MentionComposer, order: 30 });
registerPanel({ id: 'mention-queue', area: 'statusbar', component: QueueBadge, order: 20 });
