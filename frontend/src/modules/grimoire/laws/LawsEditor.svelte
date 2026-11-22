<script lang="ts">
  import { getLaws, setLaws, utf8Bytes, MAX_LAWS_BYTES } from './lawsStore';

  let { projectId, onback }: { projectId: string; onback: () => void } = $props();

  let text = $state('');
  let loaded = $state(false);
  let loadError = $state<string | null>(null);
  let saving = $state(false);
  let saved = $state(false);
  let saveError = $state<string | null>(null);

  const bytes = $derived(utf8Bytes(text));
  const over = $derived(bytes > MAX_LAWS_BYTES);
  const canSave = $derived(loaded && !saving && !over);

  // Editor sengaja tidak berlangganan grimoire.changed agar suntingan belum tersimpan tidak tertimpa.
  $effect(() => {
    const id = projectId;
    let cancelled = false;
    loaded = false;
    loadError = null;
    saved = false;
    saveError = null;
    text = '';
    getLaws(id)
      .then((c) => { if (!cancelled) { text = c; loaded = true; } })
      .catch((e) => { if (!cancelled) loadError = e instanceof Error ? e.message : String(e); });
    return () => { cancelled = true; };
  });

  async function save() {
    if (!canSave) return;
    const id = projectId;
    saving = true;
    saved = false;
    saveError = null;
    try {
      await setLaws(id, text);
      if (id === projectId) saved = true;
    } catch (e) {
      if (id === projectId) saveError = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }

  function onInput() {
    saved = false;
    saveError = null;
  }
</script>

<div class="laws-editor">
  <div class="head">
    <button class="btn" onclick={onback}>Kembali</button>
    <span class="title">Laws</span>
  </div>
  <p class="note">Perubahan berlaku untuk sesi baru; sesi yang sedang berjalan tidak berubah.</p>
  <textarea
    bind:value={text}
    oninput={onInput}
    disabled={!loaded}
    spellcheck="false"
    placeholder="Aturan khusus project ini, disuntik ke sesi agent baru."
    aria-label="Isi laws.md"
  ></textarea>
  <div class="status">
    <span class="count" class:over>{bytes} / {MAX_LAWS_BYTES} byte</span>
    <button class="btn save" onclick={save} disabled={!canSave}>{saving ? 'Menyimpan...' : 'Simpan'}</button>
    {#if saved}<span class="ok">Tersimpan</span>{/if}
    {#if saveError}<span class="err" role="alert">{saveError}</span>{/if}
    {#if loadError}<span class="err" role="alert">{loadError}</span>{/if}
  </div>
  {#if over}<div class="err over-msg">Melebihi batas {MAX_LAWS_BYTES} byte</div>{/if}
</div>

<style>
  .laws-editor { padding: 8px 12px; display: flex; flex-direction: column; gap: 8px; box-sizing: border-box; min-width: 0; }
  .head { display: flex; align-items: center; gap: 8px; }
  .title { font-weight: 600; color: var(--fg); }
  .note { margin: 0; font-size: 11px; color: var(--fg-muted); }
  textarea {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
    background: var(--bg-elev);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: 4px;
    resize: vertical;
    min-height: 200px;
    width: 100%;
    box-sizing: border-box;
  }
  textarea:focus { outline: none; border-color: var(--accent); }
  textarea:disabled { opacity: .5; }
  .status { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; font-size: 11px; }
  .count { color: var(--fg-muted); }
  .count.over { color: var(--danger); }
  .ok { color: var(--fg-muted); }
  .err { color: var(--danger); font-size: 11px; }
  .btn {
    background: transparent; color: var(--fg); border: 1px solid var(--border);
    border-radius: 4px; padding: 3px 10px; cursor: pointer; font-size: 12px;
  }
  .btn:hover:not(:disabled) { background: var(--bg-hover); }
  .btn.save { border-color: var(--accent); color: var(--accent); }
  .btn:disabled { opacity: .5; cursor: default; }
</style>
