<script lang="ts">
  import ThreadSlot from './ThreadSlot.svelte';
  import { participantName } from './thread-store';
  import { avatarColor, formatTime, initials } from './format';
  import type { Thread, ThreadMessage } from './types';

  let { thread, message }: { thread: Thread; message: ThreadMessage } = $props();

  const sender = $derived(participantName(thread, message.from));
  const receiver = $derived(participantName(thread, message.to));
  const time = $derived(formatTime(message.ts));
  const plain = $derived(message.kind === 'system' || message.kind === 'activity');
</script>

{#if plain}
  <div class="sys">{message.text}{#if time} <span class="t">{time}</span>{/if}</div>
{:else}
  <div class="msg" class:user={message.from === 'user'}>
    <span class="avatar" style:background={avatarColor(message.from)} aria-hidden="true">{initials(sender)}</span>
    <div class="body">
      <div class="meta"><span class="who">{sender} -> {receiver}</span>{#if time}<span class="t">{time}</span>{/if}</div>
      <div class="text">{message.text}</div>
    </div>
  </div>
{/if}
<ThreadSlot name="message-extra" {thread} {message} />

<style>
  .sys { text-align: center; font-size: 13px; color: var(--fg-muted, #888); padding: 2px 0; white-space: pre-wrap; overflow-wrap: anywhere; }
  .msg { display: flex; gap: 8px; padding: 6px 8px; border-radius: 6px; }
  .msg.user { background: var(--bg-elev, #fff); }
  .avatar { flex: none; width: 28px; height: 28px; border-radius: 50%; color: #fff; font-size: 13px; font-weight: 600; display: flex; align-items: center; justify-content: center; }
  .body { min-width: 0; flex: 1; }
  .meta { display: flex; gap: 8px; align-items: baseline; font-size: 13px; color: var(--fg-muted, #888); }
  .who { font-weight: 600; color: var(--fg, #222); }
  .t { color: var(--fg-muted, #888); }
  .text { font-size: 15px; white-space: pre-wrap; overflow-wrap: anywhere; color: var(--fg, #222); }
</style>
