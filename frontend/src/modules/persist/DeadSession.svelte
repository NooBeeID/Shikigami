<script lang="ts">
  import type { Leaf, Node } from '../layout/model';
  import { call } from '../../lib/ipc';
  import { sessions } from '../../lib/stores/sessions';
  import { workspaces, activeWorkspaceId } from '../../lib/stores/workspaces';
  import type { Session } from '../../lib/stores/types';
  import { layout } from '../../lib/stores/layout';

  let { leaf, onRestarted }: { leaf: Leaf; onRestarted?: (newSessionId: string) => void } = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);

  const project = $derived(
    $workspaces.find((w) => w.id === $activeWorkspaceId)?.projects?.find((p) => p.id === leaf.projectId) ?? null,
  );
  const providerLabel = $derived(leaf.provider ?? 'tidak diketahui');
  const missingProject = $derived(!leaf.projectId || !project);
  const canRestart = $derived(!busy && !missingProject && !!leaf.provider && !!$activeWorkspaceId);

  // Ganti sessionId daun target; mengembalikan node baru dan apakah daun ditemukan.
  function replaceSessionId(node: Node | null, leafId: string, newId: string, found: { v: boolean }): Node | null {
    if (!node) return null;
    if (node.type === 'pane') {
      if (node.id === leafId) {
        found.v = true;
        return { ...node, sessionId: newId };
      }
      return node;
    }
    return {
      ...node,
      a: replaceSessionId(node.a, leafId, newId, found)!,
      b: replaceSessionId(node.b, leafId, newId, found)!,
    };
  }

  async function restart() {
    if (!canRestart) return;
    const wsId = $activeWorkspaceId;
    const p = project;
    const providerId = leaf.provider;
    if (!wsId || !p || !providerId) return;
    busy = true;
    error = null;
    try {
      const s = await call<Session>('session.create', {
        workspaceId: wsId,
        projectId: p.id,
        cwd: p.path,
        providerId,
      });
      sessions.update((l) => (l.some((x) => x.id === s.id) ? l : [...l, s]));
      const doc = layout.serialize();
      const found = { v: false };
      doc.root = replaceSessionId(doc.root, leaf.id, s.id, found);
      if (found.v) layout.hydrate(doc);
      onRestarted?.(s.id);
    } catch (e) {
      error = (e instanceof Error ? e.message : String(e)) || 'Gagal memulai sesi';
    } finally {
      busy = false;
    }
  }
</script>

{#if leaf.kind === 'terminal'}
  <div class="dead" role="alert">
    <div class="card">
      <div class="title">Sesi berakhir</div>
      <div class="info">
        <span>Project: {project ? project.name : 'tidak diketahui'}</span>
        <span>Provider: {providerLabel}</span>
      </div>
      {#if missingProject}
        <div class="note">Project tidak ditemukan</div>
      {/if}
      {#if error}
        <div class="err">{error}</div>
      {/if}
      <button type="button" disabled={!canRestart} onclick={restart}>
        {busy ? 'Memulai...' : error ? 'Coba lagi' : 'Mulai ulang'}
      </button>
    </div>
  </div>
{/if}

<style>
  .dead {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg, #1e1e1e);
    color: var(--fg, #ddd);
    z-index: 5;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px 20px;
    border: 1px solid var(--border, #444);
    background: var(--bg-elev, #252526);
    border-radius: 6px;
    text-align: center;
  }
  .title { font-weight: 600; }
  .info { display: flex; flex-direction: column; gap: 2px; font-size: 12px; color: var(--fg-muted, #999); }
  .note, .err { color: var(--danger, #f44); font-size: 12px; }
  button {
    border: 1px solid var(--accent, #4a9eff);
    background: var(--accent, #4a9eff);
    color: var(--bg, #1e1e1e);
    border-radius: 4px;
    padding: 6px 12px;
    cursor: pointer;
  }
  button:hover:not(:disabled) { background: var(--bg-hover, #333); color: var(--fg, #ddd); }
  button:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
