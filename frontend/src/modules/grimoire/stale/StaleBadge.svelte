<script lang="ts">
  let { label = 'stale', title = 'Bab ini belum diperbarui' }: { label?: string; title?: string } = $props();
</script>

<span class="stale-badge" {title}><span aria-hidden="true">&#9679;</span> {label}</span>

<style>
  .stale-badge {
    color: var(--danger, inherit);
    font-size: 10px;
    flex: none;
    white-space: nowrap;
  }
</style>
