<script lang="ts">
  import WorkspaceHeader from './WorkspaceHeader.svelte';
  import type { Session } from '../../lib/stores';
  import SessionGroups from './SessionGroups.svelte';
  import NewSessionMenu from './NewSessionMenu.svelte';
  import { sessionListWrapper } from './registry';

  let err = $state('');
  const onerror = (m: string) => (err = m);
</script>

{#snippet listSnippet(filter: (s: Session) => boolean)}
  <SessionGroups {onerror} {filter} />
{/snippet}

<aside class="side">
  <div class="label">AGENT</div>
  <WorkspaceHeader {onerror} />
  {#if $sessionListWrapper}
    {@const W = $sessionListWrapper}
    <W.component {onerror} list={listSnippet} />
  {:else}
    <SessionGroups {onerror} />
  {/if}
  {#if err}
    <div class="err" role="alert">
      <span class="msg">{err}</span>
      <button type="button" aria-label="Tutup pesan galat" onclick={() => (err = '')}>×</button>
    </div>
  {/if}
  <NewSessionMenu {onerror} />
</aside>

<style>
  .side { height: 100%; min-height: 0; min-width: 0; overflow: hidden; display: flex; flex-direction: column; background: var(--bg-elev, transparent); border-right: 1px solid var(--border, #8884); }
  .label { font-size: 10px; letter-spacing: 0.08em; text-transform: uppercase; color: var(--fg-muted, #888); padding: 12px 16px 4px; }
  .err { display: flex; align-items: flex-start; gap: 6px; margin: 0 8px 4px; padding: 4px 8px; font-size: 11px; color: var(--danger, #e5484d); border: 1px solid var(--danger, #e5484d); border-radius: 4px; }
  .msg { flex: 1; min-width: 0; overflow-wrap: anywhere; }
  .err button { font: inherit; color: inherit; background: transparent; border: 0; cursor: pointer; padding: 0 2px; }
</style>
