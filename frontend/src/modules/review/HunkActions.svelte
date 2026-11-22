<script lang="ts" module>
  export interface ReviewActionResult { removed: boolean; conflict?: boolean }
</script>

<script lang="ts">
  import { call } from '../../lib/ipc';
  import type { Hunk } from './diff';

  let { sessionId, path, currentHash, external, scope, hunk, onDone }: {
    sessionId: string;
    path: string;
    currentHash: string;
    external: boolean;
    scope: 'hunk' | 'file';
    hunk?: Hunk;
    onDone: (r: ReviewActionResult) => void;
  } = $props();

  let confirming = $state(false);
  let externalConfirm = $state(false);
  let busy = $state(false);
  let errMsg = $state<string | null>(null);

  const hunkIds = $derived(scope === 'hunk' && hunk ? [hunk.id] : undefined);
  const needsStrongWarn = $derived(external || externalConfirm);
  const noun = $derived(scope === 'file' ? 'file ini' : 'hunk ini');

  function handleError(e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    if (msg.includes('[CONFLICT]')) {
      confirming = false;
      externalConfirm = false;
      errMsg = 'File berubah sejak diff dihitung, dimuat ulang';
      onDone({ removed: false, conflict: true });
    } else if (msg.includes('[EXTERNAL]')) {
      externalConfirm = true;
      confirming = true;
      errMsg = null;
    } else {
      errMsg = msg.split('\n')[0].slice(0, 120);
      confirming = false;
    }
  }

  export async function accept(): Promise<void> {
    if (busy) return;
    busy = true;
    errMsg = null;
    try {
      const res = await call<{ ok: boolean; removed: boolean }>('review.accept', { sessionId, path, hunkIds });
      onDone({ removed: !!res?.removed });
    } catch (e) {
      handleError(e);
    } finally {
      busy = false;
    }
  }

  export function startRevert(): void {
    if (busy) return;
    confirming = true;
    errMsg = null;
  }

  export function cancel(): void {
    if (busy) return;
    confirming = false;
    externalConfirm = false;
    errMsg = null;
  }

  export async function confirmRevert(): Promise<void> {
    if (!confirming || busy) return;
    busy = true;
    errMsg = null;
    const force = external || externalConfirm;
    try {
      const res = await call<{ ok: boolean; removed: boolean }>('review.revert', {
        sessionId,
        path,
        hunkIds,
        expectHash: currentHash,
        force: force || undefined,
      });
      confirming = false;
      externalConfirm = false;
      onDone({ removed: !!res?.removed });
    } catch (e) {
      handleError(e);
    } finally {
      busy = false;
    }
  }

  // Svelte 5 tidak mengizinkan export getter; kontrak `get pending` diganti fungsi.
  export function isPending(): boolean { return confirming; }
  export function isBusy(): boolean { return busy; }
</script>

<span class="ha" class:file={scope === 'file'}>
  {#if !confirming}
    <button type="button" disabled={busy} title="Revert {noun}" aria-label="Revert {noun}" onclick={startRevert}>Revert</button>
    <button type="button" disabled={busy} title="Terima {noun}" aria-label="Terima {noun}" onclick={() => void accept()}>Terima</button>
  {:else}
    {#if needsStrongWarn}
      <span class="warn">File ini diubah di luar agent; revert akan menimpa perubahan itu</span>
    {/if}
    <button type="button" class="danger" disabled={busy} onclick={() => void confirmRevert()}>{needsStrongWarn ? 'Yakin, timpa?' : 'Yakin?'}</button>
    <button type="button" disabled={busy} onclick={cancel}>Batal</button>
  {/if}
  {#if errMsg}<span class="err" role="alert">{errMsg}</span>{/if}
</span>

<style>
  .ha { display: inline-flex; align-items: center; gap: 4px; font-size: 11px; }
  button { background: transparent; border: 1px solid var(--border, color-mix(in srgb, currentColor 20%, transparent)); color: inherit; font: inherit; font-size: 11px; line-height: 16px; padding: 1px 6px; border-radius: 4px; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent)); }
  button:focus-visible { outline: 1px solid var(--accent, currentColor); }
  button:disabled { opacity: 0.5; cursor: default; }
  button.danger { color: var(--danger, #dc2626); border-color: var(--danger, #dc2626); }
  .warn { color: var(--warn, #d97706); white-space: normal; max-width: 260px; }
  .err { color: var(--danger, #dc2626); }
</style>
