<script lang="ts">
  import { untrack } from 'svelte';
  import type { Tab } from '../../layout/model';
  import { activeWorkspaceId } from '../../../lib/stores/workspaces';
  import { onEvent } from '../../../lib/ipc';
  import { copyLink, sendToAgent } from '../actions';
  import { openSettings } from '../../settings';
  import { getWikiNode, parseLarkError, type WikiGetResult } from './docClient';
  import MarkdownView from './MarkdownView.svelte';
  import SheetView from './SheetView.svelte';

  let { pane, visible }: { pane: Tab; visible: boolean } = $props();

  type View =
    | { status: 'loading' }
    | { status: 'error'; code: string; message: string }
    | { status: 'ready'; data: WikiGetResult };

  let view = $state<View>({ status: 'loading' });
  let refreshing = $state(false);
  let seq = 0;

  const wsId = $derived($activeWorkspaceId ?? '');
  const sourceId = $derived(pane.sourceId ?? '');
  const nodeToken = $derived(pane.nodeToken ?? '');

  const NO_HOST = 'Link tidak bisa dibuat (host belum diketahui). Tambah satu Base atau wiki dari link untuk menentukan host.';

  async function load(full = false): Promise<void> {
    const my = ++seq;
    try {
      if (!wsId) {
        refreshing = false;
        view = { status: 'error', code: 'no_workspace', message: 'Tidak ada workspace aktif' };
        return;
      }
      if (!sourceId || !nodeToken) {
        refreshing = false;
        view = { status: 'error', code: 'invalid_params', message: 'Dokumen ini belum punya referensi node' };
        return;
      }
      if (full || untrack(() => view.status) !== 'ready') {
        refreshing = false;
        view = { status: 'loading' };
      } else {
        refreshing = true;
      }
      try {
        const data = await getWikiNode(wsId, sourceId, nodeToken);
        if (my !== seq) return;
        refreshing = false;
        view = { status: 'ready', data };
      } catch (x) {
        if (my !== seq) return;
        const e = parseLarkError(x);
        refreshing = false;
        view = { status: 'error', code: e.code, message: e.message };
      }
    } catch (x) {
      if (my !== seq) return;
      refreshing = false;
      view = { status: 'error', code: 'unknown', message: String(x) };
    }
  }

  $effect(() => {
    wsId; sourceId; nodeToken;
    untrack(() => { void load(true); });
  });

  $effect(() => {
    const off = onEvent('workspace.changed', (_t, p) => {
      const s = view;
      if ((p as any)?.kind === 'configChanged' && s.status === 'error' && (s.code === 'source_not_found' || s.code === 'invalid_params')) {
        void load(true);
      }
    });
    return off;
  });

  const title = $derived(view.status === 'ready' ? view.data.title : (pane.title || 'Dokumen Lark'));
  const url = $derived(view.status === 'ready' ? view.data.url : '');
  const actionsDisabled = $derived(view.status !== 'ready' || url === '');
  const safeUrl = $derived(/^https?:\/\//i.test(url) ? url : '');

  function errText(code: string): string {
    switch (code) {
      case 'source_not_found': return 'Sumber tidak ada di workspace ini';
      case 'cli_missing': return 'lark-cli belum terpasang.';
      case 'not_logged_in': return 'Belum masuk ke Lark.';
      case 'permission': return 'Izin Lark kurang';
      case 'timeout': return 'Permintaan ke Lark melewati batas waktu.';
      case 'no_workspace':
      case 'invalid_params': return '';
      default: return 'Gagal memuat dokumen.';
    }
  }

  function openExternal(e: MouseEvent, href: string) {
    const open = (window as any).runtime?.BrowserOpenURL;
    if (typeof open === 'function') {
      e.preventDefault();
      open(href);
    }
  }
</script>

<div class="doc-pane" hidden={!visible}>
  <div class="header" role="toolbar" aria-label="Aksi dokumen">
    <span class="title" title={title}>{title}</span>
    {#if refreshing}<span class="muted" role="status">Memuat...</span>{/if}
    <button type="button" class="btn" aria-label="Segarkan" disabled={view.status === 'loading'} onclick={() => void load()}>Segarkan</button>
    <button type="button" class="btn" disabled={actionsDisabled} title={url === '' && view.status === 'ready' ? NO_HOST : undefined} onclick={() => void copyLink(url).catch(() => {})}>Salin link</button>
    <button type="button" class="btn" disabled={actionsDisabled} title={url === '' && view.status === 'ready' ? NO_HOST : undefined} onclick={() => void sendToAgent(url).catch(() => {})}>Kirim ke agen</button>
  </div>
  {#if view.status === 'ready' && view.data.url === ''}
    <p class="muted hint">{NO_HOST}</p>
  {/if}

  <div class="body">
    {#if view.status === 'loading'}
      <p class="muted pad" role="status">Memuat dokumen...</p>
    {:else if view.status === 'error'}
      {@const base = errText(view.code)}
      <div class="error" role="alert">
        {#if base}<p>{base}{#if view.code === 'permission' && view.message} {view.message}{/if}</p>{/if}
        {#if view.message && (!base || (view.code !== 'permission' && view.message !== base))}
          <p class="small">{view.message}</p>
        {/if}
        <div class="row">
          {#if view.code === 'cli_missing' || view.code === 'not_logged_in'}
            <button type="button" class="btn" onclick={() => openSettings('connectors')}>Buka Pengaturan</button>
          {/if}
          <button type="button" class="btn" onclick={() => void load(true)}>Coba lagi</button>
        </div>
      </div>
    {:else if view.data.objType === 'docx'}
      {#if (view.data.markdown ?? '').trim() === ''}
        <p class="muted pad">Dokumen kosong</p>
      {:else}
        <MarkdownView source={view.data.markdown ?? ''} />
      {/if}
    {:else if view.data.objType === 'sheet'}
      <SheetView cells={view.data.cells ?? []} truncated={!!view.data.truncated} />
    {:else}
      <div class="center">
        <h3>{view.data.title}</h3>
        <p class="muted">Tipe dokumen ini ({view.data.objType}) belum bisa ditampilkan di sini.</p>
        {#if safeUrl}
          <a href={safeUrl} target="_blank" rel="noopener noreferrer" onclick={(e) => openExternal(e, safeUrl)}>Buka di Lark</a>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .doc-pane { flex: 1; min-height: 0; min-width: 0; display: flex; flex-direction: column; color: var(--fg, inherit); background: var(--bg, transparent); }
  .doc-pane[hidden] { display: none; }
  .header { display: flex; align-items: center; gap: 6px; padding: 6px 10px; border-bottom: 1px solid var(--border, currentColor); }
  .title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
  .btn { background: transparent; color: inherit; border: 1px solid var(--border, currentColor); border-radius: 4px; padding: 2px 8px; font-size: 14px; cursor: pointer; }
  .btn:hover:not(:disabled) { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  .btn:focus-visible { outline: 1px solid var(--accent, currentColor); }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .body { flex: 1; min-height: 0; display: flex; flex-direction: column; }
  .muted { color: var(--fg-muted, inherit); }
  .hint { margin: 0; padding: 4px 10px; font-size: 14px; }
  .pad { padding: 10px; margin: 0; }
  .error { padding: 10px; color: var(--danger, inherit); }
  .error p { margin: 0 0 6px; }
  .error .small { font-size: 14px; color: var(--fg-muted, inherit); }
  .row { display: flex; gap: 6px; }
  .center { margin: auto; text-align: center; padding: 16px; }
  .center a { color: var(--accent, inherit); }
</style>
