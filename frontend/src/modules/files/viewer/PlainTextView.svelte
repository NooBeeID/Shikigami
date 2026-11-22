<script lang="ts">
  import { untrack } from 'svelte';
  import { basename } from '../shared/fsClient';
  import { BinaryFileError, FileNotFoundError } from './fileApi';
  import { LineReader } from './lineIndex';
  import BinaryNotice from './BinaryNotice.svelte';

  let { path, size }: { path: string; size: number } = $props();

  const ROW_HEIGHT = 18;
  const OVERSCAN = 20;
  const MAX_DOM_ROWS = 190;

  let viewport = $state<HTMLDivElement | null>(null);
  let scrollTop = $state(0);
  let viewH = $state(0);
  let reader = $state.raw<LineReader | null>(null);
  let winFirst = $state(0);
  let winLines = $state.raw<string[]>([]);
  let tick = $state(0);
  let error = $state.raw<unknown>(null);
  let gen = 0;

  function createReader() {
    gen++;
    error = null;
    scrollTop = 0;
    winFirst = 0;
    winLines = [];
    reader = new LineReader(path, size);
    tick++;
    if (viewport) viewport.scrollTop = 0;
  }

  // Buat ulang pembaca saat path/size berubah.
  $effect(() => {
    void path;
    void size;
    untrack(createReader);
    return () => {
      gen++;
    };
  });

  const totalRows = $derived.by(() => {
    void tick;
    return reader ? reader.totalLines : 0;
  });
  const first = $derived(Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN));
  const last = $derived.by(() => {
    const l = Math.min(totalRows, Math.ceil((scrollTop + viewH) / ROW_HEIGHT) + OVERSCAN);
    return Math.min(l, first + MAX_DOM_ROWS);
  });

  // Muat baris untuk jendela yang terlihat.
  $effect(() => {
    const rd = reader;
    const f = first;
    const l = last;
    if (!rd) return;
    const my = ++gen;
    (async () => {
      try {
        await rd.ensureIndexed(l);
        if (my !== gen) return;
        const lines = await rd.getLines(f, Math.min(l, rd.indexedLines));
        if (my !== gen) return;
        winFirst = f;
        winLines = lines ?? [];
        tick++;
      } catch (e) {
        if (my === gen) error = e;
      }
    })();
    return () => {
      gen++;
    };
  });

  const visibleRows = $derived.by(() => {
    void tick;
    const rd = reader;
    const out: { row: number; text: string | undefined }[] = [];
    if (!rd) return out;
    const end = Math.min(last, totalRows);
    for (let r = first; r < end; r++) {
      let text: string | undefined;
      const k = r - winFirst;
      if (k >= 0 && k < winLines.length) text = winLines[k];
      else text = rd.peekLines(r, r + 1)[0];
      out.push({ row: r, text });
    }
    return out;
  });

  const gutterCh = $derived(String(Math.max(1, totalRows)).length + 2);
  const cols = $derived.by(() => {
    void tick;
    return (reader?.maxCols ?? 0) + 1;
  });
  const empty = $derived.by(() => {
    void tick;
    return size === 0 || (!!reader && reader.complete && reader.indexedLines === 0);
  });
  const notes = $derived.by(() => {
    void tick;
    const rd = reader;
    const n: string[] = [];
    if (!rd) return n;
    if (rd.truncated) n.push('Hanya 500.000 baris pertama ditampilkan');
    if (rd.stale) n.push('File berubah saat dibaca; muat ulang tab untuk melihat isi terbaru');
    return n;
  });
  const position = $derived.by(() => {
    void tick;
    const rd = reader;
    if (!rd) return '';
    const cur = Math.min(totalRows, Math.floor(scrollTop / ROW_HEIGHT) + 1);
    return rd.complete ? `Baris ${cur} dari ${totalRows}` : `Baris ${cur} dari ~${totalRows}`;
  });

  function onScroll(e: Event) {
    scrollTop = (e.currentTarget as HTMLDivElement).scrollTop;
  }
</script>

{#if error instanceof BinaryFileError}
  <BinaryNotice {path} />
{:else if error instanceof FileNotFoundError}
  <div class="msg" role="alert">
    <div>File tidak ditemukan.</div>
    <div class="path">{path}</div>
  </div>
{:else if error}
  <div class="msg" role="alert">
    <div>Gagal membaca file: {error instanceof Error ? error.message : String(error)}</div>
    <button type="button" onclick={createReader}>Coba lagi</button>
  </div>
{:else if empty}
  <div class="msg">(file kosong)</div>
{:else}
  <div class="wrap">
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div
      class="viewport"
      role="region"
      aria-label={'Isi file ' + basename(path)}
      tabindex="0"
      bind:this={viewport}
      bind:clientHeight={viewH}
      onscroll={onScroll}
    >
      <div
        class="spacer"
        style:height={totalRows * ROW_HEIGHT + 'px'}
        style:width={`calc(${cols + gutterCh}ch + 16px)`}
      >
        {#each visibleRows as r (r.row)}
          <div class="row" style:top={r.row * ROW_HEIGHT + 'px'}>
            <span class="gutter" style:width={gutterCh + 'ch'}>{r.row + 1}</span><span class="text">{r.text ?? '…'}</span>
          </div>
        {/each}
      </div>
    </div>
    <div class="footer">
      <span>{position}</span>
      {#each notes as note}<span class="note">{note}</span>{/each}
    </div>
  </div>
{/if}

<style>
  .wrap { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .viewport {
    flex: 1;
    min-height: 0;
    overflow: auto;
    position: relative;
    font-family: ui-monospace, monospace;
    font-size: 14px;
    line-height: 18px;
    color: var(--fg, inherit);
    outline: none;
  }
  .spacer { position: relative; }
  .row { position: absolute; left: 0; width: 100%; height: 18px; white-space: pre; }
  .gutter {
    position: sticky;
    left: 0;
    display: inline-block;
    box-sizing: border-box;
    padding-right: 1ch;
    text-align: right;
    color: var(--fg-muted, inherit);
    opacity: 0.7;
    background: var(--bg, inherit);
    user-select: none;
  }
  .text { padding-left: 1ch; }
  .footer {
    display: flex;
    gap: 12px;
    padding: 2px 8px;
    font-size: 13px;
    color: var(--fg-muted, inherit);
    border-top: 1px solid var(--border, transparent);
  }
  .note { color: var(--danger, inherit); }
  .msg { margin: auto; padding: 16px; text-align: center; color: var(--fg-muted, inherit); }
  .path { margin-top: 6px; font-size: 14px; font-family: ui-monospace, monospace; word-break: break-all; }
</style>
