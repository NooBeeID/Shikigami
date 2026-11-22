<script lang="ts">
  import { onDestroy } from 'svelte';
  import { decodeFileTab, setTabDirty } from '../shared/shellBridge';
  import { basename, watchFile } from '../shared/fsClient';
  import type { FileChangedEvent } from '../shared/types';
  import {
    fileKind, readChunk, readAll, BinaryFileError, FileNotFoundError,
  } from '../viewer/fileApi';
  import MarkdownView from '../viewer/MarkdownView.svelte';
  import PlainTextView from '../viewer/PlainTextView.svelte';
  import BinaryNotice from '../viewer/BinaryNotice.svelte';
  import { ConflictError, writeFile } from './writeApi';

  const EDIT_WARN_BYTES = 5 * 1024 * 1024;
  const MARKDOWN_MAX_BYTES = 2 * 1024 * 1024;

  let props: Record<string, unknown> = $props();
  const data = $derived(decodeFileTab(props));
  const path = $derived(data.path);
  const kind = $derived(fileKind(path));

  type Phase = 'loading' | 'ready' | 'binary' | 'notfound' | 'error';
  type Banner = null | { type: 'conflict' | 'external' | 'removed'; currentMtime?: number };

  let phase = $state<Phase>('loading');
  let errorMessage = $state('');
  let saveError = $state('');
  let mode = $state<'view' | 'edit'>('view');
  let content = $state('');
  let savedContent = $state('');
  let baseMtime = $state(0);
  let size = $state(0);
  let saving = $state(false);
  let justSaved = $state(false);
  let banner = $state<Banner>(null);
  let confirmOverwrite = $state(false);
  let largeWarn = $state(false);
  let reloadKey = $state(0);
  let intersecting = $state(false);
  let docVisible = $state(typeof document === 'undefined' ? true : document.visibilityState !== 'hidden');
  const visible = $derived(intersecting && docVisible);

  let rootEl: HTMLDivElement | undefined = $state();
  let unwatch: (() => void) | null = null;
  let savedTimer: ReturnType<typeof setTimeout> | undefined;
  let lastPath = '';

  const dirty = $derived(phase === 'ready' && content !== savedContent);
  const isMarkdownKind = $derived(kind === 'markdown');
  const largeMarkdown = $derived(isMarkdownKind && size > MARKDOWN_MAX_BYTES);

  function mib(n: number): string {
    return (n / (1024 * 1024)).toFixed(1);
  }

  function handleLoadError(e: unknown) {
    if (e instanceof BinaryFileError) phase = 'binary';
    else if (e instanceof FileNotFoundError) phase = 'notfound';
    else {
      phase = 'error';
      errorMessage = e instanceof Error ? e.message : String(e);
    }
  }

  async function load() {
    phase = 'loading';
    const p = path;
    try {
      const probe = await readChunk(p, 0, 1);
      size = probe.size;
      baseMtime = probe.mtime;
      if (kind === 'markdown' && size <= MARKDOWN_MAX_BYTES) {
        const all = await readAll(p);
        content = savedContent = all.content;
        baseMtime = all.mtime;
        size = all.size;
      } else {
        content = savedContent = '';
      }
      phase = 'ready';
      banner = null;
    } catch (e) {
      handleLoadError(e);
    }
  }

  async function reload() {
    confirmOverwrite = false;
    banner = null;
    saveError = '';
    const wasEdit = mode === 'edit';
    await load();
    if (wasEdit && phase === 'ready') {
      try {
        const all = await readAll(path);
        content = savedContent = all.content;
        baseMtime = all.mtime;
        size = all.size;
      } catch (e) {
        handleLoadError(e);
      }
    }
    reloadKey++;
  }

  async function enterEdit(confirmed = false) {
    if (mode === 'edit') return;
    if (!confirmed && size > EDIT_WARN_BYTES) {
      largeWarn = true;
      return;
    }
    largeWarn = false;
    try {
      const all = await readAll(path);
      content = savedContent = all.content;
      baseMtime = all.mtime;
      size = all.size;
      mode = 'edit';
    } catch (e) {
      handleLoadError(e);
    }
  }

  function enterView() {
    if (mode === 'view') return;
    if (kind === 'text' && dirty) return;
    mode = 'view';
    if (kind === 'text') reloadKey++;
  }

  async function doWrite(): Promise<void> {
    saving = true;
    saveError = '';
    try {
      const r = await writeFile(path, content, baseMtime);
      baseMtime = r.mtime;
      savedContent = content;
      size = new TextEncoder().encode(content).length;
      banner = null;
      confirmOverwrite = false;
      justSaved = true;
      clearTimeout(savedTimer);
      savedTimer = setTimeout(() => { justSaved = false; }, 2000);
    } catch (e) {
      if (e instanceof ConflictError) {
        banner = { type: 'conflict', currentMtime: e.currentMtime };
      } else if (e instanceof FileNotFoundError) {
        banner = { type: 'removed' };
      } else {
        saveError = e instanceof Error ? e.message : String(e);
      }
    } finally {
      saving = false;
    }
  }

  async function save() {
    if (saving || mode !== 'edit' || !dirty) return;
    await doWrite();
  }

  async function overwrite() {
    if (saving) return;
    confirmOverwrite = false;
    try {
      const fresh = await readChunk(path, 0, 1);
      baseMtime = fresh.mtime;
    } catch (e) {
      if (!(e instanceof FileNotFoundError)) {
        saveError = e instanceof Error ? e.message : String(e);
        return;
      }
      // File dihapus: tulis ulang dengan mtime 0 (file baru).
      baseMtime = 0;
    }
    await doWrite();
  }

  function onKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
      e.preventDefault();
      void save();
    }
  }

  function onFsChanged(ev: FileChangedEvent) {
    if (saving) return;
    if (ev.op === 'remove') {
      banner = { type: 'removed' };
      return;
    }
    if (ev.mtime === baseMtime) return;
    banner = { type: 'external', currentMtime: ev.mtime };
  }

  function startWatch() {
    if (unwatch || phase !== 'ready') return;
    unwatch = watchFile(path, onFsChanged);
  }

  function stopWatch() {
    unwatch?.();
    unwatch = null;
  }

  let wasVisible = false;
  async function checkOnReturn() {
    try {
      const fresh = await readChunk(path, 0, 1);
      if (fresh.mtime !== baseMtime && !saving && !banner) {
        banner = { type: 'external', currentMtime: fresh.mtime };
      }
    } catch {
      // diabaikan; galat akan tampil saat simpan atau muat ulang
    }
  }

  // Muat awal; ulangi bila path tab berubah.
  $effect(() => {
    const p = path;
    if (p === lastPath) return;
    lastPath = p;
    stopWatch();
    mode = 'view';
    banner = null;
    void load();
  });

  $effect(() => {
    setTabDirty(path, dirty);
  });

  $effect(() => {
    if (visible && phase === 'ready') {
      startWatch();
      if (!wasVisible) {
        wasVisible = true;
        if (savedContent !== undefined && lastPath) void checkOnReturn();
      }
    } else {
      stopWatch();
      if (!visible) wasVisible = false;
    }
  });

  $effect(() => {
    if (!rootEl || typeof IntersectionObserver === 'undefined') {
      intersecting = true;
      return;
    }
    const obs = new IntersectionObserver((entries) => {
      for (const en of entries) intersecting = en.isIntersecting;
    });
    obs.observe(rootEl);
    return () => obs.disconnect();
  });

  $effect(() => {
    const h = () => { docVisible = document.visibilityState !== 'hidden'; };
    document.addEventListener('visibilitychange', h);
    return () => document.removeEventListener('visibilitychange', h);
  });

  onDestroy(() => {
    stopWatch();
    clearTimeout(savedTimer);
    setTabDirty(path, false);
  });
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="file-tab" bind:this={rootEl} tabindex="-1" role="group" onkeydown={onKeydown}>
  <header class="bar">
    <span class="name" title={path}>
      {basename(path)}
      {#if dirty}<span class="dot" title="Belum disimpan" aria-hidden="true">●</span><span class="sr">Belum disimpan</span>{/if}
    </span>
    {#if justSaved}<span class="saved">Tersimpan</span>{/if}
    {#if saveError}<span class="save-error">Gagal menyimpan: {saveError}</span>{/if}
    <span class="spacer"></span>
    {#if phase === 'ready'}
      {#if mode === 'edit'}
        <button type="button" onclick={save} disabled={saving || !dirty}>Simpan</button>
      {/if}
      <span class="group">
        <button
          type="button"
          aria-pressed={mode === 'view'}
          disabled={mode === 'edit' && kind === 'text' && dirty}
          title={mode === 'edit' && kind === 'text' && dirty ? 'Simpan dulu sebelum kembali ke tampilan' : undefined}
          onclick={enterView}
        >Tampilan</button>
        <button type="button" aria-pressed={mode === 'edit'} onclick={() => enterEdit()}>Edit</button>
      </span>
    {/if}
  </header>

  {#if banner}
    <div class="banner" class:warn={banner.type !== 'removed'} role="alert">
      {#if banner.type === 'removed'}
        <span>File dihapus di disk</span>
        <button type="button" onclick={() => (banner = null)}>Tutup banner</button>
      {:else}
        <span>File berubah di disk{dirty ? '. Suntingan Anda belum hilang' : ''}</span>
        {#if confirmOverwrite}
          <span>Timpa file di disk dengan suntingan Anda?</span>
          <button type="button" onclick={overwrite}>Ya, timpa</button>
          <button type="button" onclick={() => (confirmOverwrite = false)}>Batal</button>
        {:else}
          <button type="button" onclick={reload}>Muat ulang</button>
          {#if mode === 'edit' && dirty}
            <button type="button" onclick={() => (confirmOverwrite = true)}>Timpa</button>
          {/if}
          <button type="button" onclick={() => { banner = null; confirmOverwrite = false; }}>
            {banner.type === 'conflict' ? 'Batal' : 'Abaikan'}
          </button>
        {/if}
      {/if}
    </div>
  {/if}

  {#if largeWarn}
    <div class="banner warn" role="alert">
      <span>File besar ({mib(size)} MiB). Menyunting memuat seluruh isi ke memori. Lanjutkan?</span>
      <button type="button" onclick={() => enterEdit(true)}>Tetap sunting</button>
      <button type="button" onclick={() => (largeWarn = false)}>Batal</button>
    </div>
  {/if}

  <div class="content">
    {#if phase === 'loading'}
      <div class="msg">Memuat...</div>
    {:else if phase === 'binary'}
      <BinaryNotice {path} />
    {:else if phase === 'notfound'}
      <div class="msg">File tidak ditemukan: {path}. Anda bisa menutup tab ini.</div>
    {:else if phase === 'error'}
      <div class="msg">
        <div>{errorMessage}</div>
        <button type="button" onclick={load}>Coba lagi</button>
      </div>
    {:else if mode === 'edit'}
      <textarea
        bind:value={content}
        spellcheck="false"
        wrap="off"
        readonly={saving}
        aria-label="Isi file"
      ></textarea>
    {:else if kind === 'markdown' && !largeMarkdown}
      <MarkdownView source={content} />
    {:else}
      {#if largeMarkdown}
        <div class="banner" role="status"><span>File besar, ditampilkan sebagai teks</span></div>
      {/if}
      {#key reloadKey}
        <PlainTextView {path} {size} />
      {/key}
    {/if}
  </div>
</div>

<style>
  .file-tab {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    outline: none;
    color: var(--fg, inherit);
    background: var(--bg, transparent);
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 8px;
    border-bottom: 1px solid var(--border, currentColor);
    background: var(--bg-elev, transparent);
    font-size: 14px;
  }
  .name { font-weight: 600; }
  .dot { color: var(--accent, currentColor); margin-left: 4px; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  .spacer { flex: 1; }
  .saved { color: var(--ok, inherit); }
  .save-error { color: var(--danger, inherit); }
  .group { display: inline-flex; gap: 2px; }
  button {
    font: inherit;
    padding: 2px 8px;
    color: inherit;
    background: var(--bg, transparent);
    border: 1px solid var(--border, currentColor);
    border-radius: 4px;
    cursor: pointer;
  }
  button:hover:not(:disabled) { background: var(--bg-hover, transparent); }
  button:disabled { opacity: 0.5; cursor: default; }
  button[aria-pressed='true'] { background: var(--bg-hover, transparent); font-weight: 600; }
  .banner {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    font-size: 14px;
    border-bottom: 1px solid var(--border, currentColor);
    background: var(--bg-elev, transparent);
  }
  .banner.warn { border-left: 3px solid var(--danger, currentColor); }
  .content { flex: 1; min-height: 0; overflow: auto; display: flex; flex-direction: column; }
  .msg { padding: 12px; display: flex; flex-direction: column; gap: 8px; align-items: flex-start; }
  textarea {
    flex: 1;
    min-height: 0;
    width: 100%;
    box-sizing: border-box;
    resize: none;
    border: none;
    outline: none;
    padding: 8px;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 15px;
    line-height: 1.5;
    tab-size: 4;
    white-space: pre;
    overflow: auto;
    color: inherit;
    background: transparent;
  }
</style>
