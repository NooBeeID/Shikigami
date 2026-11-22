<script lang="ts">
  // Dialog konfirmasi tutup tab file yang belum disimpan (Simpan / Buang / Batal).
  // Komponen hanya tampilan + status busy/error; logika menyimpan ada di index.ts.
  let { items, onSave, onDiscard, onCancel }: {
    items: { id: string; label: string }[];
    onSave: () => Promise<{ ok: true } | { ok: false; failedLabels: string[] }>;
    onDiscard: () => void;
    onCancel: () => void;
  } = $props();

  let busy = $state(false);
  let error = $state('');
  let overlay: HTMLDivElement | undefined = $state();
  let dlg: HTMLDivElement | undefined = $state();
  let primaryBtn: HTMLButtonElement | undefined = $state();

  const multi = $derived(items.length > 1);
  const titleId = 'confirm-close-title-' + Math.random().toString(36).slice(2);

  // Fokus awal ke Simpan (bukan Buang) agar Enter tidak menjalankan aksi destruktif.
  $effect(() => {
    void Promise.resolve().then(() => (primaryBtn ?? overlay)?.focus());
  });

  async function save() {
    if (busy) return;
    busy = true;
    error = '';
    try {
      const r = await onSave();
      if (!r.ok) {
        error = 'Gagal menyimpan: ' + r.failedLabels.join(', ') + '. Pilih Batal untuk tetap membuka tab, atau Buang untuk menutup tanpa menyimpan.';
      }
    } catch (e) {
      error = 'Gagal menyimpan: ' + (e instanceof Error ? e.message : String(e));
    } finally {
      busy = false;
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      onCancel();
      return;
    }
    if (e.key === 'Tab' && dlg) {
      const btns = Array.from(dlg.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'));
      if (btns.length === 0) return;
      const first = btns[0];
      const last = btns[btns.length - 1];
      const active = document.activeElement;
      if (e.shiftKey && (active === first || !dlg.contains(active))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && (active === last || !dlg.contains(active))) {
        e.preventDefault();
        first.focus();
      }
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="overlay" tabindex="-1" bind:this={overlay} onpointerdown={(e) => { if (e.target === e.currentTarget) onCancel(); }} {onkeydown}>
  <div class="dlg" role="dialog" aria-modal="true" aria-labelledby={titleId} bind:this={dlg}>
    {#if multi}
      <h3 id={titleId}>{items.length} file belum disimpan</h3>
      <ul>
        {#each items as it (it.id)}<li>{it.label}</li>{/each}
      </ul>
    {:else}
      <h3 id={titleId}>Simpan perubahan "{items[0]?.label}"?</h3>
    {/if}
    <p class="hint">Perubahan akan hilang bila ditutup tanpa disimpan.</p>
    {#if error}<div class="err" role="alert">{error}</div>{/if}
    <div class="btns">
      <button type="button" onclick={onCancel}>Batal</button>
      <button type="button" class="discard" onclick={onDiscard}>{multi ? 'Buang semua' : 'Buang'}</button>
      <button type="button" class="primary" bind:this={primaryBtn} onclick={save} disabled={busy}>{busy ? 'Menyimpan...' : multi ? 'Simpan semua' : 'Simpan'}</button>
    </div>
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; z-index: 100; background: #0006; display: flex; align-items: center; justify-content: center; outline: none; }
  .dlg { width: min(440px, 92vw); max-height: 90vh; background: var(--bg-elev, #fff); color: var(--fg, #222); border: 1px solid var(--border, #8884); border-radius: 8px; padding: 16px; display: flex; flex-direction: column; gap: 8px; box-shadow: 0 10px 30px #0006; }
  h3 { margin: 0; font-size: 16px; overflow-wrap: anywhere; }
  .hint { margin: 0; font-size: 14px; color: var(--fg-muted, #888); }
  ul { margin: 0; padding-left: 18px; max-height: 40vh; overflow-y: auto; font-size: 14px; overflow-wrap: anywhere; }
  .err { color: var(--danger, #e5484d); font-size: 14px; overflow-wrap: anywhere; }
  .btns { display: flex; justify-content: flex-end; gap: 8px; }
  button { font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 6px; padding: 6px 14px; cursor: pointer; }
  button.discard { color: var(--danger, #e5484d); border-color: var(--danger, #e5484d); }
  button.primary { background: var(--accent, #3b82f6); color: #fff; border-color: transparent; }
  button:disabled { opacity: 0.5; cursor: default; }
</style>
