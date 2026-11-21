<script lang="ts">
  import type { Leaf } from '../layout/model';
  import TerminalPane from '../layout/panes/TerminalPane.svelte';
  import DeadSession from './DeadSession.svelte';
  import { persistStatus } from '../../lib/stores/persist';
  import { sessions } from '../../lib/stores/sessions';

  let { pane, visible }: { pane: Leaf; visible: boolean } = $props();

  const dead = $derived.by(() => {
    if ($persistStatus !== 'ready') return false;
    if (!pane.sessionId) return true;
    const s = $sessions.find((x) => x.id === pane.sessionId);
    return !s || s.status === 'exited';
  });
</script>

{#if dead}
  <div class="guard"><DeadSession leaf={pane} /></div>
{:else}
  <TerminalPane {pane} {visible} />
{/if}

<style>
  .guard { position: absolute; inset: 0; }
</style>
