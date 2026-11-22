<script lang="ts">
  import { errorText } from './thread-client';
  import { createThread } from './thread-store';

  let { open, onclose }: { open: boolean; onclose: () => void } = $props();

  let goal = $state('');
  let error = $state('');
  let busy = $state(false);
  let area: HTMLTextAreaElement | undefined = $state();

  $effect(() => {
    if (open) {
      goal = '';
      error = '';
      busy = false;
      void Promise.resolve().then(() => area?.focus());
    }
  });

  async function submit() {
    if (busy) return;
    if (!goal.trim()) {
      error = 'Goal tidak boleh kosong';
      return;
    }
    busy = true;
    error = '';
    try {
      await createThread(goal);
      goal = '';
      onclose();
    } catch (e) {
      error = e instanceof Error && /^(Goal tidak|Pilih workspace)/.test(e.message) ? e.message : errorText(e);
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      if (!busy) onclose();
    } else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      void submit();
    }
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="overlay" onpointerdown={(e) => { if (e.target === e.currentTarget && !busy) onclose(); }} {onkeydown}>
    <div class="dlg" role="dialog" aria-modal="true" aria-label="Thread baru">
      <h3>Thread baru</h3>
      <label for="thread-goal">Goal</label>
      <textarea id="thread-goal" bind:this={area} bind:value={goal} rows="6" placeholder="Jelaskan tujuan yang melibatkan beberapa project..."></textarea>
      {#if error}<div class="err" role="alert">{error}</div>{/if}
      <div class="btns">
        <button type="button" onclick={onclose} disabled={busy}>Batal</button>
        <button type="button" class="primary" onclick={submit} disabled={busy || !goal.trim()}>{busy ? 'Membuat...' : 'Buat'}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; z-index: 100; background: #0006; display: flex; align-items: center; justify-content: center; }
  .dlg { width: min(520px, 92vw); background: var(--bg-elev, #fff); color: var(--fg, #222); border: 1px solid var(--border, #8884); border-radius: 8px; padding: 16px; display: flex; flex-direction: column; gap: 8px; box-shadow: 0 10px 30px #0006; }
  h3 { margin: 0; font-size: 14px; }
  label { font-size: 12px; color: var(--fg-muted, #888); }
  textarea { font: inherit; font-size: 13px; color: var(--fg, #222); background: var(--bg, #fff); border: 1px solid var(--border, #8884); border-radius: 6px; padding: 8px; resize: vertical; }
  .err { color: var(--danger, #e5484d); font-size: 12px; }
  .btns { display: flex; justify-content: flex-end; gap: 8px; }
  button { font: inherit; font-size: 12px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 6px; padding: 6px 14px; cursor: pointer; }
  button.primary { background: var(--accent, #3b82f6); color: #fff; border-color: transparent; }
  button:disabled { opacity: 0.5; cursor: default; }
</style>
