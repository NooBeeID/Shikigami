<script lang="ts">
  let { disabled, busy = false, error = '', onsend }: {
    disabled: boolean; busy?: boolean; error?: string; onsend: (text: string) => Promise<void> | void;
  } = $props();

  let text = $state('');
  let el: HTMLTextAreaElement | undefined = $state();
  let sending = $state(false);

  function resize() {
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 8 * 20) + 'px';
  }

  $effect(() => {
    void text;
    resize();
  });

  async function submit() {
    const v = text.trim();
    if (!v || disabled || busy || sending) return;
    sending = true;
    try {
      await onsend(v);
      text = '';
    } catch {
      // Galat ditampilkan pemanggil lewat prop error; teks dipertahankan.
    } finally {
      sending = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      void submit();
    }
  }
</script>

<div class="wrap">
  {#if error}<div class="err" role="alert">{error}</div>{/if}
  <div class="row">
    <textarea
      bind:this={el}
      bind:value={text}
      {onkeydown}
      {disabled}
      rows="1"
      placeholder={disabled ? 'Thread sudah ditutup' : 'Tulis pesan ke master... (Enter kirim, Shift+Enter baris baru)'}
    ></textarea>
    <button type="button" onclick={submit} disabled={disabled || busy || sending || !text.trim()}>Kirim</button>
  </div>
</div>

<style>
  .wrap { padding: 8px 12px; border-top: 1px solid var(--border, #8884); }
  .err { color: var(--danger, #e5484d); font-size: 14px; margin-bottom: 4px; }
  .row { display: flex; gap: 8px; align-items: flex-end; }
  textarea { flex: 1; resize: none; font: inherit; font-size: 15px; line-height: 20px; color: var(--fg, #222); background: var(--bg-elev, #fff); border: 1px solid var(--border, #8884); border-radius: 6px; padding: 6px 8px; min-height: 34px; box-sizing: border-box; }
  textarea:disabled { opacity: 0.6; }
  button { font: inherit; font-size: 14px; color: var(--fg, #222); background: var(--bg-elev, #fff); border: 1px solid var(--border, #8884); border-radius: 6px; padding: 7px 12px; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--bg-hover, #0001); }
  button:disabled { opacity: 0.5; cursor: default; }
</style>
