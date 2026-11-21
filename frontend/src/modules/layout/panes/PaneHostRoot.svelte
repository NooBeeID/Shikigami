<script lang="ts">
  import type { Component } from 'svelte';
  import type { Leaf } from '../model';

  let { component, pane, visible }:
    { component: Component<{ pane: Leaf; visible: boolean }>; pane: Leaf; visible: boolean } = $props();

  let cur = $state<{ pane: Leaf; visible: boolean }>({ pane, visible });
  const Comp = component;

  export function update(p: Leaf, v: boolean) {
    cur = { pane: p, visible: v };
  }
</script>

<div class="pane-root"><Comp pane={cur.pane} visible={cur.visible} /></div>

<style>
  .pane-root { flex: 1; min-height: 0; min-width: 0; display: flex; flex-direction: column; position: relative; }
  .pane-root > :global(*) { flex: 1; min-height: 0; }
</style>
