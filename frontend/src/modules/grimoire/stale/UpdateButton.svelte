<script lang="ts">
  import { runUpdate } from './staleStore';

  let { projectId, building, staleCount, onupdate }: {
    projectId: string;
    building: boolean;
    staleCount: number;
    onupdate?: () => void;
  } = $props();

  async function click() {
    if (building) return;
    onupdate?.();
    await runUpdate(projectId);
  }
</script>

<button type="button" class="update-btn" disabled={building} onclick={click}>
  {building ? 'Membangun...' : 'Perbarui'}{!building && staleCount > 0 ? ` (${staleCount} stale)` : ''}
</button>

<style>
  .update-btn {
    border: 1px solid var(--border, currentColor);
    background: var(--bg-elev, transparent);
    color: var(--fg, inherit);
    border-radius: 4px;
    font-size: 13px;
    padding: 2px 8px;
    cursor: pointer;
  }
  .update-btn:hover:not(:disabled) { background: var(--bg-hover, var(--bg-elev, transparent)); }
  .update-btn:focus-visible { outline: 1px solid var(--accent, currentColor); outline-offset: 1px; }
  .update-btn:disabled { opacity: 0.6; cursor: default; }
</style>
