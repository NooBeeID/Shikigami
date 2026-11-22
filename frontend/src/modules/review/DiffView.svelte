<!--
  DiffView: tampilan unified diff satu file.
  Titik ekstensi (opsional, kosong bila tidak diberikan):
  - hunkActions: Snippet<[Hunk]> (nama slot publik `hunk-actions`), dirender di kanan header hunk.
  - footer: Snippet (nama slot publik `footer`), dirender di bawah semua hunk/pesan status.
  - onLineClick: callback klik baris.
  - selectedHunkId: id hunk yang disorot (kelas `selected`, atribut data-hunk-id pada header hunk).
  - sessionId: bila ada, klik nomor baris membuka CommentBox di bawah baris.
-->
<script lang="ts">
  import { untrack, type Snippet } from 'svelte';
  import CommentBox from './CommentBox.svelte';
  import { hunkSummary, type DiffLine, type DiffResult, type Hunk } from './diff';

  let { diff, path, onLineClick, hunkActions, footer, selectedHunkId, sessionId }: {
    diff: DiffResult;
    path: string;
    onLineClick?: (line: DiffLine, hunk: Hunk) => void;
    hunkActions?: Snippet<[Hunk]>;
    footer?: Snippet;
    selectedHunkId?: string;
    sessionId?: string;
  } = $props();

  const status = $derived(
    diff.binary ? 'File biner, tidak ada diff'
      : diff.tooLarge ? 'File terlalu besar (> 1 MB), diff tidak ditampilkan'
      : diff.hunks.length === 0 ? 'Tidak ada perubahan terhadap baseline'
      : null,
  );

  let commentTarget = $state<{ hunkId: string; lineIndex: number; line: number } | null>(null);
  let commentSending = $state(false);
  function openComment(e: MouseEvent, h: Hunk, li: number, ln: DiffLine) {
    if (!sessionId) return;
    e.stopPropagation();
    if (commentSending) return;
    const line = ln.newNo || ln.oldNo;
    if (!line || line < 1) return;
    commentTarget = { hunkId: h.id, lineIndex: li, line };
  }
  $effect(() => {
    diff;
    if (!untrack(() => commentSending)) commentTarget = null;
  });

  function onKey(e: KeyboardEvent, ln: DiffLine, h: Hunk) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onLineClick?.(ln, h);
    }
  }
  function marker(k: DiffLine['kind']): string {
    return k === 'add' ? '+' : k === 'del' ? '-' : ' ';
  }
</script>

<div class="wrap">
  {#if status}
    <div class="empty">{status}</div>
    {#if footer}{@render footer()}{/if}
  {:else}
    <div class="diff" aria-label="Diff {path}">
      {#each diff.hunks as h, i (h.id + ':' + i)}
        <div class="hunk-head" class:selected={h.id === selectedHunkId} data-hunk-id={h.id}>
          <span class="range">@@ -{h.oldStart},{h.oldLines} +{h.newStart},{h.newLines} @@</span>
          <span class="sum">{hunkSummary(h)}</span>
          {#if hunkActions}<span class="actions">{@render hunkActions(h)}</span>{/if}
        </div>
        {#each h.lines as ln, li}
          {#if onLineClick}
            <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
            <div
              class="line {ln.kind} clickable"
              role="button"
              tabindex="0"
              onclick={() => onLineClick?.(ln, h)}
              onkeydown={(e) => onKey(e, ln, h)}
            ><span class="no" class:cm={!!sessionId} title={sessionId ? 'Komentar ke agent' : undefined} onclick={sessionId ? (e) => openComment(e, h, li, ln) : undefined}>{ln.oldNo || ''}</span><span class="no" class:cm={!!sessionId} title={sessionId ? 'Komentar ke agent' : undefined} onclick={sessionId ? (e) => openComment(e, h, li, ln) : undefined}>{ln.newNo || ''}</span><span class="mk">{marker(ln.kind)}</span><span class="tx">{ln.text}</span></div>
          {:else}
            <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
            <div class="line {ln.kind}"><span class="no" class:cm={!!sessionId} title={sessionId ? 'Komentar ke agent' : undefined} onclick={sessionId ? (e) => openComment(e, h, li, ln) : undefined}>{ln.oldNo || ''}</span><span class="no" class:cm={!!sessionId} title={sessionId ? 'Komentar ke agent' : undefined} onclick={sessionId ? (e) => openComment(e, h, li, ln) : undefined}>{ln.newNo || ''}</span><span class="mk">{marker(ln.kind)}</span><span class="tx">{ln.text}</span></div>
          {/if}
          {#if sessionId && commentTarget && commentTarget.hunkId === h.id && commentTarget.lineIndex === li}
            <div class="comment-slot">
              <CommentBox {sessionId} {path} line={commentTarget.line} bind:sending={commentSending}
                          onclose={() => { commentTarget = null; commentSending = false; }} />
            </div>
          {/if}
        {/each}
      {/each}
      {#if footer}{@render footer()}{/if}
    </div>
  {/if}
</div>

<style>
  .wrap { display: flex; flex-direction: column; flex: 1; min-height: 0; }
  .empty { flex: 1; display: flex; align-items: center; justify-content: center; padding: 12px; text-align: center; color: var(--fg-muted, #888); font-size: 14px; }
  .diff { flex: 1; min-height: 0; overflow: auto; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
  .hunk-head { display: flex; align-items: center; gap: 12px; min-width: max-content; padding: 2px 8px; background: color-mix(in srgb, var(--accent, currentColor) 8%, transparent); border-bottom: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); color: var(--fg-muted, #888); }
  .hunk-head.selected { background: color-mix(in srgb, var(--accent, currentColor) 22%, transparent); box-shadow: inset 2px 0 0 var(--accent, currentColor); }
  .actions { margin-left: auto; display: inline-flex; gap: 4px; }
  .line { display: flex; min-width: max-content; white-space: pre; line-height: 18px; }
  .line.add { background: color-mix(in srgb, var(--ok, #16a34a) 15%, transparent); }
  .line.del { background: color-mix(in srgb, var(--danger, #dc2626) 15%, transparent); }
  .line.add .mk { color: var(--ok, #16a34a); }
  .line.del .mk { color: var(--danger, #dc2626); }
  .no { flex: none; width: 48px; padding-right: 8px; text-align: right; color: var(--fg-muted, #888); user-select: none; font-variant-numeric: tabular-nums; }
  .mk { flex: none; width: 16px; text-align: center; user-select: none; }
  .tx { flex: 1; padding-right: 8px; }
  .clickable { cursor: pointer; }
  .clickable:hover { filter: brightness(1.1); }
  .clickable:focus-visible { outline: 1px solid var(--accent, currentColor); outline-offset: -1px; }
  .no.cm { cursor: pointer; }
  .no.cm:hover { color: var(--accent, currentColor); text-decoration: underline; }
  .comment-slot { position: sticky; left: 0; width: min(100%, 640px); box-sizing: border-box; padding: 4px 8px 6px 64px; white-space: normal; }
</style>
