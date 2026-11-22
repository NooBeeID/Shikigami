<script lang="ts">
  import { tick } from 'svelte';
  import MessageItem from './MessageItem.svelte';
  import { ActivityFlowItem, groupActivity } from './activity';
  import { participantName } from './thread-store';
  import type { Thread, ThreadMessage } from './types';

  let { thread, messages }: { thread: Thread; messages: ThreadMessage[] } = $props();

  let el: HTMLDivElement | undefined = $state();
  let stick = $state(true);
  let unseen = $state(false);
  let lastCount = 0;

  const flow = $derived(groupActivity(messages, (id) => participantName(thread, id)));
  const marker = $derived.by(() => {
    const lastItem = flow[flow.length - 1];
    return flow.length + (lastItem && lastItem.type === 'activity' ? lastItem.count : 0);
  });

  function onscroll() {
    if (!el) return;
    stick = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
    if (stick) unseen = false;
  }

  function toBottom() {
    if (!el) return;
    el.scrollTop = el.scrollHeight;
    stick = true;
    unseen = false;
  }

  $effect(() => {
    const n = marker;
    const grew = n > lastCount;
    const first = lastCount === 0;
    lastCount = n;
    if (!grew) return;
    if (stick || first) {
      void tick().then(() => {
        if (el) el.scrollTop = el.scrollHeight;
      });
    } else {
      unseen = true;
    }
  });
</script>

<div class="list" bind:this={el} {onscroll}>
  {#each flow as item (item.key)}
    {#if item.type === 'activity'}
      <ActivityFlowItem {thread} group={item} />
    {:else}
      <MessageItem {thread} message={item.message} />
    {/if}
  {:else}
    <div class="empty">Belum ada pesan</div>
  {/each}
  {#if !stick && unseen}
    <button type="button" class="jump" onclick={toBottom}>Ke pesan terbaru</button>
  {/if}
</div>

<style>
  .list { flex: 1; min-height: 0; overflow-y: auto; padding: 8px 12px; display: flex; flex-direction: column; gap: 4px; position: relative; }
  .empty { margin: auto; font-size: 12px; color: var(--fg-muted, #888); }
  .jump { position: sticky; bottom: 4px; align-self: center; font: inherit; font-size: 11px; color: var(--fg, #222); background: var(--bg-elev, #fff); border: 1px solid var(--border, #8884); border-radius: 12px; padding: 3px 10px; cursor: pointer; }
</style>
