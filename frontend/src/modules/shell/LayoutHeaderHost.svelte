<script lang="ts">
  import { layoutHeaders } from './registry';

  const headers = layoutHeaders();
</script>

{#each $headers as h (h.id)}
  <h.component />
{/each}
