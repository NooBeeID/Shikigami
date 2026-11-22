<script lang="ts">
  import { get } from 'svelte/store';
  import { activeWorkspaceId } from '../../../lib/stores/workspaces';
  import { layout } from '../../../lib/stores/layout';
  import { larkSources } from '../store';
  import type { LarkConfig, WikiSource } from '../client';
  import AddSourceDialog from '../AddSourceDialog.svelte';
  import SourceRow from './SourceRow.svelte';
  import WikiTree from './WikiTree.svelte';

  type PersonalSource = { id: 'personal'; name: 'Pribadi'; spaceId: 'my_library'; host: string };

  let cfg = $state<LarkConfig>({ tickets: [], wiki: [] });
  let ws = $state<string | null>(get(activeWorkspaceId));
  let addOpen = $state(false);
  let expandedId = $state<string | null>(null);

  $effect(() => larkSources.subscribe((v) => { cfg = v; }));
  $effect(() => activeWorkspaceId.subscribe((v) => { ws = v; }));

  let personal = $derived<PersonalSource>({
    id: 'personal',
    name: 'Pribadi',
    spaceId: 'my_library',
    host: cfg.wiki[0]?.host ?? cfg.tickets[0]?.host ?? '',
  });

  $effect(() => {
    if (expandedId && expandedId !== 'personal' && !cfg.wiki.some((w) => w.id === expandedId)) expandedId = null;
  });

  function open(source: WikiSource | PersonalSource) {
    layout.openPane({
      kind: 'lark-doc',
      sourceId: source.id,
      nodeToken: ('rootToken' in source ? source.rootToken : undefined) ?? '',
      title: source.name,
    });
    expandedId = source.id;
  }

  function toggle(id: string) {
    expandedId = expandedId === id ? null : id;
  }
</script>

<div class="sec">
  <div class="bar">
    <button type="button" class="add" disabled={!ws} onclick={() => (addOpen = true)}>Tambah wiki</button>
  </div>

  <div class="line">
    <button type="button" class="chev" aria-label="Ciutkan atau bentangkan Pribadi" aria-expanded={expandedId === 'personal'} onclick={() => toggle('personal')}>{expandedId === 'personal' ? '▾' : '▸'}</button>
    <button type="button" class="open" onclick={() => open(personal)}>
      <span class="name">Pribadi</span>
      <span class="auto">otomatis</span>
    </button>
  </div>
  {#if expandedId === 'personal'}
    <WikiTree source={personal} onOpen={(nodeToken) => layout.openPane({ kind: 'lark-doc', sourceId: 'personal', nodeToken, title: 'Pribadi' })} />
  {/if}

  {#each cfg.wiki as s (s.id)}
    <div class="line">
      <button type="button" class="chev" aria-label={`Ciutkan atau bentangkan ${s.name}`} aria-expanded={expandedId === s.id} onclick={() => toggle(s.id)}>{expandedId === s.id ? '▾' : '▸'}</button>
      <div class="grow"><SourceRow id={s.id} name={s.name} host={s.host} label="wiki" onopen={() => open(s)} /></div>
    </div>
    {#if expandedId === s.id}
      <WikiTree source={s} onOpen={(nodeToken) => layout.openPane({ kind: 'lark-doc', sourceId: s.id, nodeToken, title: s.name })} />
    {/if}
  {/each}

  <AddSourceDialog kind="wiki" open={addOpen} onclose={() => (addOpen = false)} />
</div>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    padding: 0 6px 6px 6px;
  }
  .bar {
    display: flex;
    justify-content: flex-end;
    padding-bottom: 2px;
  }
  .add {
    background: none;
    border: none;
    color: var(--fg-muted, #888);
    font-size: 14px;
    cursor: pointer;
  }
  .add:hover:not(:disabled) {
    color: var(--fg, #ddd);
  }
  .add:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .line {
    display: flex;
    align-items: center;
  }
  .grow {
    flex: 1;
    min-width: 0;
  }
  .chev {
    background: none;
    border: none;
    color: var(--fg-muted, #888);
    width: 18px;
    cursor: pointer;
    font-size: 13px;
    padding: 0;
  }
  .open {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 6px;
    background: none;
    border: none;
    color: var(--fg, #ddd);
    text-align: left;
    padding: 4px 6px;
    cursor: pointer;
    border-radius: 4px;
    font-size: 15px;
  }
  .open:hover {
    background: var(--bg-hover, #2c2c2c);
  }
  .auto {
    color: var(--fg-muted, #888);
    font-size: 12px;
  }
</style>
