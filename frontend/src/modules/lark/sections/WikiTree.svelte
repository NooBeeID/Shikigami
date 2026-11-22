<script lang="ts">
  // Placeholder; ditimpa lark-wiki. Jaga props { source, onOpen }.
  import type { WikiSource } from '../client';

  type PersonalSource = { id: 'personal'; name: 'Pribadi'; spaceId: 'my_library'; host: string };

  let { source, onOpen }: { source: WikiSource | PersonalSource; onOpen: (nodeToken: string) => void } = $props();
</script>

<div class="tree">
  <button type="button" class="row" onclick={() => onOpen(('rootToken' in source ? source.rootToken : undefined) ?? '')}>
    Akar: {source.name}
  </button>
  <span class="hint">Pohon halaman segera hadir</span>
</div>

<style>
  .tree {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-left: 18px;
  }
  .row {
    background: none;
    border: none;
    color: var(--fg, #ddd);
    text-align: left;
    padding: 3px 6px;
    font-size: 12px;
    cursor: pointer;
    border-radius: 4px;
  }
  .row:hover {
    background: var(--bg-hover, #2c2c2c);
  }
  .hint {
    color: var(--fg-muted, #888);
    font-size: 11px;
    padding: 0 6px;
  }
</style>
