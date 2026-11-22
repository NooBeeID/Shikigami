<script lang="ts">
  import type { Component } from 'svelte';
  import type { Tab } from '../model';

  let { component, pane, paneId, visible }:
    { component: Component<{ pane: Tab; paneId: string; visible: boolean }>; pane: Tab; paneId: string; visible: boolean } = $props();

  let cur = $state<{ pane: Tab; paneId: string; visible: boolean }>({ pane, paneId, visible });
  const Comp = component;

  export function update(p: Tab, id: string, v: boolean) {
    cur = { pane: p, paneId: id, visible: v };
  }
</script>

<div class="pane-root"><Comp pane={cur.pane} paneId={cur.paneId} visible={cur.visible} /></div>

<style>
  .pane-root { flex: 1; min-height: 0; min-width: 0; display: flex; flex-direction: column; position: relative; }
  .pane-root > :global(*) { flex: 1; min-height: 0; }
</style>
