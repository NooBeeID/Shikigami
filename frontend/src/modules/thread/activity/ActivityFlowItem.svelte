<script lang="ts">
  import type { Thread } from '../types';
  import { participantName } from '../thread-store';
  import { avatarColor } from '../format';
  import ActivityCard from './ActivityCard.svelte';
  import { isRunning, activityRemainingMs, type ActivityGroup } from './groupActivity';

  let { thread, group }: { thread: Thread; group: ActivityGroup } = $props();

  const agentName = $derived(participantName(thread, group.from));
  const color = $derived(avatarColor(group.from));
  const closed = $derived(thread?.state === 'closed');

  let now = $state(Date.now());
  const running = $derived(isRunning(group, now, group.finished, closed));

  // Satu timeout per perubahan lastTs/finished/closed; tanpa polling.
  $effect(() => {
    const g = group;
    const c = closed;
    void g.lastTs;
    void g.finished;
    const t = Date.now();
    if (c || g.finished || !isRunning(g, t, g.finished, c)) {
      now = t;
      return;
    }
    const id = setTimeout(() => {
      now = Date.now();
    }, activityRemainingMs(g, t) + 50);
    return () => clearTimeout(id);
  });

  function storageKey(key: string): string {
    return `thread.activity.open.${key}`;
  }
  function readOpen(key: string): boolean {
    try {
      return localStorage.getItem(storageKey(key)) === '1';
    } catch {
      return false;
    }
  }
  function writeOpen(key: string, v: boolean) {
    try {
      localStorage.setItem(storageKey(key), v ? '1' : '0');
    } catch {
      /* abaikan */
    }
  }
</script>

<div class="flow">
  <ActivityCard
    {agentName}
    {color}
    items={group.items}
    count={group.count}
    {running}
    initialOpen={readOpen(group.key)}
    onToggle={(v) => writeOpen(group.key, v)}
  />
</div>

<style>
  .flow {
    min-width: 0;
    max-width: 100%;
  }
</style>
