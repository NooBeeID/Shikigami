<!--
  CommentBox: kotak komentar untuk satu baris diff. Mengirim lewat IPC `review.comment`
  (ditempel ke input prompt agent tanpa Enter).
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { call } from '../../lib/ipc';

  let { sessionId, path, line, onclose, onsent, sending = $bindable(false) }: {
    sessionId: string;
    path: string;
    line: number;
    onclose: () => void;
    onsent?: (sent: string) => void;
    sending?: boolean;
  } = $props();

  let text = $state('');
  let error = $state<string | null>(null);
  let busy = $state(false);
  let done = $state(false);
  let ta: HTMLTextAreaElement | undefined = $state();
  let timer: ReturnType<typeof setTimeout> | undefined;

  const canSend = $derived(text.trim().length > 0 && !sending && !done);

  onMount(() => ta?.focus());
  onDestroy(() => clearTimeout(timer));

  async function send() {
    if (!canSend) return;
    sending = true;
    error = null;
    busy = false;
    try {
      const res = await call<{ ok: boolean; sent: string; enter: boolean }>('review.comment', { sessionId, path, line, text });
      onsent?.(res.sent);
      done = true;
      timer = setTimeout(() => onclose(), 2000);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      busy = msg.includes('[BUSY]');
      error = busy ? 'Agent sedang bekerja, tunggu selesai lalu kirim lagi' : msg;
    } finally {
      sending = false;
    }
  }

  function close() {
    clearTimeout(timer);
    onclose();
  }

  function onKeydown(e: KeyboardEvent) {
    e.stopPropagation();
    if (e.key === 'Escape') {
      e.preventDefault();
      close();
    } else if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      void send();
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
<div class="cbox" onkeydown={(e) => e.stopPropagation()} onclick={(e) => e.stopPropagation()}>
  <div class="lbl">{path}:{line}</div>
  <textarea
    bind:this={ta}
    bind:value={text}
    maxlength="2000"
    rows="3"
    placeholder="Komentar untuk agent..."
    disabled={sending || done}
    aria-label="Komentar untuk {path}:{line}"
    onkeydown={onKeydown}
  ></textarea>
  <div class="row">
    <button type="button" class="primary" disabled={!canSend} onclick={() => void send()}>{sending ? 'Mengirim...' : 'Kirim ke agent'}</button>
    <button type="button" class="secondary" disabled={sending} onclick={close}>Batal</button>
    <span class="count">{text.length}/2000</span>
  </div>
  <div class="msg" role="status" aria-live="polite">
    {#if done}
      <span class="ok">Ditempel ke input agent, tekan Enter di sesi</span>
    {:else if error}
      <span class={busy ? 'warn' : 'err'}>{error}</span>
    {/if}
  </div>
</div>

<style>
  .cbox { border: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); background: var(--bg, canvas); color: var(--fg, inherit); padding: 8px; border-radius: 4px; font-family: system-ui, sans-serif; font-size: 12px; white-space: normal; box-sizing: border-box; }
  .lbl { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--fg-muted, #888); margin-bottom: 4px; overflow-wrap: anywhere; }
  textarea { width: 100%; box-sizing: border-box; resize: vertical; background: transparent; color: var(--fg, inherit); border: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); border-radius: 3px; padding: 4px 6px; font: inherit; }
  textarea:focus-visible { outline: 1px solid var(--accent, currentColor); outline-offset: 0; }
  .row { display: flex; align-items: center; gap: 6px; margin-top: 6px; }
  button { font: inherit; padding: 3px 10px; border-radius: 3px; cursor: pointer; color: var(--fg, inherit); }
  button.primary { background: var(--accent, #2563eb); border: 1px solid var(--accent, #2563eb); color: var(--bg, #fff); }
  button.secondary { background: transparent; border: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); }
  button:disabled { opacity: 0.5; cursor: default; }
  .count { margin-left: auto; color: var(--fg-muted, #888); font-variant-numeric: tabular-nums; }
  .msg { min-height: 0; margin-top: 4px; }
  .msg:empty { margin-top: 0; }
  .ok { color: var(--ok, #16a34a); }
  .warn { color: var(--warn, #d97706); }
  .err { color: var(--danger, #dc2626); }
</style>
