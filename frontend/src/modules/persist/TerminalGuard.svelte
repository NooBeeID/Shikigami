<script lang="ts">
  import type { Tab } from '../layout/model';
  import TerminalPane from '../layout/panes/TerminalPane.svelte';
  import DeadSession from './DeadSession.svelte';
  import { persistStatus } from '../../lib/stores/persist';
  import { sessions } from '../../lib/stores/sessions';

  let { pane, paneId, visible }: { pane: Tab; paneId: string; visible: boolean } = $props();

  const dead = $derived.by(() => {
    if ($persistStatus !== 'ready') return false;
    if (!pane.sessionId) return true;
    const s = $sessions.find((x) => x.id === pane.sessionId);
    return !s || s.status === 'exited';
  });
</script>

{#if dead}
  <div class="guard"><DeadSession tab={pane} {paneId} /></div>
{:else}
  <TerminalPane {pane} {paneId} {visible} />
{/if}

<style>
  .guard { position: absolute; inset: 0; }
</style>
