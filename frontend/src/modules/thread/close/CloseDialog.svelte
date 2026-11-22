<script lang="ts">
  import { untrack } from 'svelte';
  import type { Thread } from '../types';
  import { errorText } from '../thread-client';
  import { closeThread, fetchClosePreview, type ClosePreview } from './closeApi';
  import { closeCloseDialog } from './closeDialogStore';

  let { thread, open }: { thread: Thread; open: boolean } = $props();

  let status = $state<'idle' | 'loading' | 'ready' | 'error'>('idle');
  let preview = $state<ClosePreview | null>(null);
  let loadError = $state('');
  let busy = $state(false);
  let closeError = $state('');
  let overlay: HTMLDivElement | undefined = $state();
  let gen = 0;

  async function load() {
    const my = ++gen;
    status = 'loading';
    loadError = '';
    try {
      const p = await fetchClosePreview(thread.id);
      if (my !== gen) return;
      preview = p;
      status = 'ready';
    } catch (e) {
      if (my !== gen) return;
      loadError = errorText(e);
      status = 'error';
    }
  }

  // Bergantung hanya pada `open`: satu panggilan preview per pembukaan.
  $effect(() => {
    if (open) {
      untrack(() => {
        preview = null;
        closeError = '';
        busy = false;
        void load();
        void Promise.resolve().then(() => overlay?.focus());
      });
    } else {
      gen++;
      status = 'idle';
    }
  });

  async function confirmClose() {
    if (busy || status !== 'ready') return;
    busy = true;
    closeError = '';
    try {
      await closeThread(thread.id);
      closeCloseDialog();
    } catch (e) {
      closeError = errorText(e);
    } finally {
      busy = false;
    }
  }

  function cancel() {
    if (busy) return;
    closeCloseDialog();
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') cancel();
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="overlay" tabindex="-1" bind:this={overlay} onpointerdown={(e) => { if (e.target === e.currentTarget) cancel(); }} {onkeydown}>
    <div class="dlg" role="dialog" aria-modal="true" aria-label="Tutup thread">
      <h3>Tutup thread</h3>
      <p class="hint">Periksa pekerjaan yang belum dipindahkan sebelum menutup.</p>
      <div class="danger" role="alert">
        Semua worktree dan branch thread ini akan dihapus PERMANEN dan semua agent thread dimatikan. Perubahan yang belum di-commit dan commit yang belum di-merge akan hilang.
      </div>
      <div class="content">
        {#if status === 'loading' || status === 'idle'}
          <div class="muted">Memeriksa worktree...</div>
        {:else if status === 'error'}
          <div class="loaderr">{loadError}</div>
          <button type="button" onclick={() => void load()}>Coba lagi</button>
        {:else if preview}
          {#if preview.worktrees.length === 0}
            <div class="muted">Tidak ada worktree peserta; tidak ada yang akan hilang.</div>
          {:else}
            {#each preview.worktrees as w (w.path || w.participant.sessionId)}
              <div class="card">
                <div class="head">
                  <strong>{w.participant.name || w.participant.sessionId}</strong>
                  {#if w.participant.projectId}<span class="muted">{w.participant.projectId}</span>{/if}
                </div>
                <div class="meta">
                  branch <code>{w.branch}</code>{#if w.baseBranch} dari <code>{w.baseBranch}</code>{/if}
                </div>
                <div class="meta"><code>{w.path}</code></div>
                {#if w.error}
                  <div class="loaderr">Tidak bisa diperiksa: {w.error}</div>
                {:else if w.uncommitted.length === 0 && w.unmergedCommits.length === 0}
                  <div class="clean">Bersih: tidak ada yang akan hilang.</div>
                {:else}
                  {#if w.uncommitted.length > 0}
                    <div class="sub">Perubahan belum di-commit ({w.uncommitted.length})</div>
                    <ul>
                      {#each w.uncommitted as u}
                        <li><code class="tag">{u.status}</code> <span>{u.path}</span></li>
                      {/each}
                    </ul>
                  {/if}
                  {#if w.unmergedCommits.length > 0}
                    <div class="sub">Commit belum di-merge ({w.unmergedCommits.length})</div>
                    <ul>
                      {#each w.unmergedCommits as c}
                        <li><code class="tag">{c.sha}</code> <span>{c.subject}</span></li>
                      {/each}
                    </ul>
                  {/if}
                {/if}
              </div>
            {/each}
          {/if}
        {/if}
      </div>
      {#if closeError}<div class="err" role="alert">{closeError}</div>{/if}
      <div class="btns">
        <button type="button" onclick={cancel} disabled={busy}>Batal</button>
        <button type="button" class="danger-btn" onclick={confirmClose} disabled={busy || status !== 'ready'}>{busy ? 'Menutup...' : 'Lanjut tutup'}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; z-index: 100; background: #0006; display: flex; align-items: center; justify-content: center; outline: none; }
  .dlg { width: min(560px, 92vw); max-height: 90vh; background: var(--bg-elev, #fff); color: var(--fg, #222); border: 1px solid var(--border, #8884); border-radius: 8px; padding: 16px; display: flex; flex-direction: column; gap: 8px; box-shadow: 0 10px 30px #0006; }
  h3 { margin: 0; font-size: 16px; }
  .hint { margin: 0; font-size: 14px; color: var(--fg-muted, #888); }
  .danger { color: var(--danger, #e5484d); border: 1px solid var(--danger, #e5484d); background: color-mix(in srgb, var(--danger, #e5484d) 10%, transparent); border-radius: 6px; padding: 8px; font-size: 14px; overflow-wrap: anywhere; }
  .content { max-height: 50vh; overflow-y: auto; overflow-x: hidden; display: flex; flex-direction: column; gap: 8px; font-size: 14px; overflow-wrap: anywhere; }
  .muted { color: var(--fg-muted, #888); }
  .loaderr { color: var(--danger, #e5484d); }
  .clean { color: var(--ok, #30a46c); }
  .card { border: 1px solid var(--border, #8884); border-radius: 6px; padding: 8px; display: flex; flex-direction: column; gap: 4px; }
  .head { display: flex; gap: 8px; align-items: baseline; }
  .meta { color: var(--fg-muted, #888); }
  .sub { font-weight: bold; margin-top: 4px; }
  ul { margin: 0; padding-left: 16px; display: flex; flex-direction: column; gap: 2px; }
  code { font-family: ui-monospace, monospace; font-size: 13px; overflow-wrap: anywhere; }
  .err { color: var(--danger, #e5484d); font-size: 14px; overflow-wrap: anywhere; }
  .btns { display: flex; justify-content: flex-end; gap: 8px; }
  button { font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 6px; padding: 6px 14px; cursor: pointer; align-self: flex-start; }
  .btns button { align-self: auto; }
  button.danger-btn { background: var(--danger, #e5484d); color: #fff; border-color: transparent; }
  button:disabled { opacity: 0.5; cursor: default; }
</style>
