<script lang="ts">
  import { untrack } from 'svelte';
  import { onEvent } from '../../../lib/ipc';
  import { readAll, fileKind, BinaryFileError, FileNotFoundError, MARKDOWN_MAX_BYTES } from '../../files/viewer/fileApi';
  import MarkdownView from '../../files/viewer/MarkdownView.svelte';
  import BinaryNotice from '../../files/viewer/BinaryNotice.svelte';

  let { root, path, projectId }: { root: string; path: string; projectId?: string } = $props();

  let status = $state<'loading' | 'ready' | 'notfound' | 'binary' | 'error'>('loading');
  let content = $state('');
  let size = $state(0);
  let errorMsg = $state('');
  let gen = 0;

  async function load() {
    const my = ++gen;
    if (status !== 'ready') status = 'loading';
    try {
      const c = await readAll(path, root);
      if (my !== gen) return;
      content = c.content;
      size = c.size;
      status = 'ready';
    } catch (e) {
      if (my !== gen) return;
      if (e instanceof BinaryFileError) status = 'binary';
      else if (e instanceof FileNotFoundError) status = 'notfound';
      else {
        errorMsg = e instanceof Error ? e.message : String(e);
        status = 'error';
      }
    }
  }

  $effect(() => {
    void root;
    void path;
    untrack(() => {
      void load();
    });
  });

  $effect(() => {
    const pid = projectId;
    const off = onEvent('grimoire.changed', (_t, payload) => {
      const p = payload as { projectId?: string } | null | undefined;
      if (!pid || p?.projectId === pid) void load();
    });
    return off;
  });
</script>

<div class="wrap">
  {#if status === 'loading'}
    <div class="note">Memuat...</div>
  {:else if status === 'notfound'}
    <div class="note">Halaman tidak ditemukan. Bangun ulang grimoire.</div>
  {:else if status === 'binary'}
    <BinaryNotice {path} />
  {:else if status === 'error'}
    <div class="note">{errorMsg}</div>
  {:else if fileKind(path) === 'markdown' && size <= MARKDOWN_MAX_BYTES}
    <MarkdownView source={content} />
  {:else}
    <pre class="plain">{content}</pre>
  {/if}
</div>

<style>
  .wrap { overflow: auto; height: 100%; padding: 0.75rem 1rem; box-sizing: border-box; }
  .note { padding: 1.25rem; color: var(--fg-muted, #888); }
  .plain { margin: 0; white-space: pre-wrap; word-break: break-word; font-family: ui-monospace, monospace; font-size: 0.85rem; }
</style>
