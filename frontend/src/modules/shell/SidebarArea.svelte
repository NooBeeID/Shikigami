<script lang="ts">
  import { panels } from '../../lib/registry';
  const sidebar = panels('sidebar');
</script>

<aside class="sidebar">
  {#each $sidebar as panel (panel.id)}
    <section class="panel"><panel.component /></section>
  {/each}
</aside>

<style>
  .sidebar {
    width: var(--sidebar-w);
    height: 100%;
    overflow: auto;
    background: var(--bg-elev);
    border-right: 1px solid var(--border);
  }
</style>
