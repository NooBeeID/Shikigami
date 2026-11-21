<script lang="ts">
  import { panels } from '../../lib/registry';
  const statusbar = panels('statusbar');
</script>

<footer class="statusbar">
  {#each $statusbar as panel (panel.id)}
    <section class="panel"><panel.component /></section>
  {/each}
</footer>

<style>
  .statusbar {
    height: var(--statusbar-h);
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 8px;
    background: var(--bg-elev);
    border-top: 1px solid var(--border);
    color: var(--fg-muted);
    overflow: hidden;
  }
</style>
