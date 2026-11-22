<script lang="ts">
  import { threadSlots, type ThreadSlotName, type ThreadSlotSpec } from './slots';
  import type { Thread, ThreadMessage } from './types';

  let { name, thread, message }: { name: ThreadSlotName; thread: Thread; message?: ThreadMessage } = $props();

  let list = $state<ThreadSlotSpec[]>([]);
  $effect(() => {
    const unsub = threadSlots(name).subscribe((v) => (list = v));
    return unsub;
  });
</script>

{#each list as s, i (s.id ?? `${i}`)}
  {@const C = s.component}
  <C {thread} {message} />
{/each}
