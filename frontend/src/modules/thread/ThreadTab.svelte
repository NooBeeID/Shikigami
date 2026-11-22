<script lang="ts">
  import MessageList from './MessageList.svelte';
  import ThreadHeader from './ThreadHeader.svelte';
  import ThreadInput from './ThreadInput.svelte';
  import ThreadSlot from './ThreadSlot.svelte';
  import { hasThreadSlot } from './slots';
  import { errorText, getThread } from './thread-client';
  import { loadMessages, messagesByThread, sendMessage, threads, upsertThreadIfActiveWorkspace } from './thread-store';

  let { threadId }: { threadId: string; visible?: boolean } = $props();

  const thread = $derived($threads.find((t) => t.id === threadId));
  const messages = $derived($messagesByThread[threadId] ?? []);
  const sidePanel = hasThreadSlot('side-panel');

  let loading = $state(false);
  let loadError = $state('');
  let sendError = $state('');
  let attempt = $state(0);

  $effect(() => {
    void loadMessages(threadId);
  });

  $effect(() => {
    void attempt;
    if (thread) return;
    const id = threadId;
    loading = true;
    loadError = '';
    getThread(id)
      .then((t) => upsertThreadIfActiveWorkspace(t))
      .catch((e) => (loadError = errorText(e)))
      .finally(() => (loading = false));
  });

  async function handleSend(text: string) {
    try {
      sendError = '';
      await sendMessage(threadId, text);
    } catch (e) {
      sendError = errorText(e);
      throw e;
    }
  }
</script>

<div class="tab">
  {#if thread}
    <ThreadHeader {thread} />
    <div class="mid">
      <div class="main"><MessageList {thread} {messages} /></div>
      {#if $sidePanel}
        <aside><ThreadSlot name="side-panel" {thread} /></aside>
      {/if}
    </div>
    <ThreadSlot name="above-input" {thread} />
    <ThreadInput disabled={thread.state === 'closed'} error={sendError} onsend={handleSend} />
  {:else if loadError}
    <div class="state">
      <span class="err">{loadError}</span>
      <button type="button" onclick={() => attempt++}>Coba lagi</button>
    </div>
  {:else}
    <div class="state">{loading ? 'Memuat thread...' : 'Thread tidak tersedia'}</div>
  {/if}
</div>

<style>
  .tab { display: flex; flex-direction: column; height: 100%; min-height: 0; background: var(--bg, #fff); color: var(--fg, #222); }
  .mid { flex: 1; min-height: 0; display: flex; }
  .main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  aside { width: 260px; flex: none; border-left: 1px solid var(--border, #8884); overflow-y: auto; }
  .state { margin: auto; display: flex; flex-direction: column; gap: 8px; align-items: center; font-size: 12px; color: var(--fg-muted, #888); }
  .err { color: var(--danger, #e5484d); }
  button { font: inherit; font-size: 12px; color: var(--fg, #222); background: var(--bg-elev, #fff); border: 1px solid var(--border, #8884); border-radius: 6px; padding: 4px 10px; cursor: pointer; }
</style>
