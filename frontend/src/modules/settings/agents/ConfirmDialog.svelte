<script lang="ts">
  import Modal from '../Modal.svelte';

  let {
    open,
    onclose,
    title,
    message,
    confirmLabel,
    onconfirm,
  }: {
    open: boolean;
    onclose: () => void;
    title: string;
    message: string;
    confirmLabel: string;
    onconfirm: () => Promise<void>;
  } = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);

  async function confirm() {
    if (busy) return;
    busy = true;
    error = null;
    try {
      await onconfirm();
      onclose();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }
</script>

<Modal {open} {onclose} {title}>
  <div class="wrap">
    <p>{message}</p>
    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Batal</button>
      <button type="button" class="danger" disabled={busy} onclick={confirm}>
        {busy ? 'Memproses...' : confirmLabel}
      </button>
    </div>
  </div>
</Modal>

<style>
  .wrap { display: flex; flex-direction: column; gap: 12px; padding: 16px; overflow: auto; max-height: 100%; box-sizing: border-box; }
  p { margin: 0; font-size: 15px; }
  .error { color: #e49a9a; font-size: 15px; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
  button { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 4px 12px; font-size: 15px; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--bg-hover, #2c2c2c); }
  button:disabled { opacity: 0.6; cursor: default; }
  button:focus-visible { outline: 2px solid var(--fg-muted, #888); outline-offset: 2px; }
  .danger { border-color: rgba(200, 70, 70, 0.6); color: #e49a9a; }
</style>
