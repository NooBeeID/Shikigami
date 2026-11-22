<script lang="ts">
  import { larkSources, larkState, refreshSources, sectionOpen, type LarkState } from './store';
  import { openSettings } from '../settings';
  import TicketSection from './sections/TicketSection.svelte';
  import WikiSection from './sections/WikiSection.svelte';

  let st = $state<LarkState>({ status: 'loading' });
  let hasContent = $state(false);
  let open = $state({ tickets: true, wiki: true });

  $effect(() => larkState.subscribe((v) => { st = v; }));
  $effect(() => larkSources.subscribe((v) => { hasContent = v.tickets.length + v.wiki.length > 0; }));
  $effect(() => sectionOpen.subscribe((v) => { open = v; }));

  function reload() {
    larkState.set({ status: 'loading' });
    void refreshSources();
  }

  function toggle(k: 'tickets' | 'wiki') {
    sectionOpen.update((v) => ({ ...v, [k]: !v[k] }));
  }

  let errTitle = $derived.by(() => {
    if (st.status !== 'error') return '';
    switch (st.code) {
      case 'cli_missing': return 'lark-cli belum terpasang';
      case 'not_logged_in': return 'Belum login Lark';
      case 'permission': return `Izin Lark kurang${st.message ? `: ${st.message}` : ''}`;
      case 'timeout': return 'Waktu habis';
      default: return st.message ?? 'Terjadi galat';
    }
  });
  let needSettings = $derived(st.status === 'error' && (st.code === 'cli_missing' || st.code === 'not_logged_in'));
  let showSections = $derived(st.status === 'ready' || (st.status === 'error' && hasContent));
</script>

<section class="lark" aria-label="Lark">
  <div class="header">
    <span class="title">LARK</span>
    <button class="icon" type="button" title="Segarkan" aria-label="Segarkan" onclick={reload}>&#8635;</button>
  </div>
  <div class="body">
    {#if st.status === 'loading' && !hasContent}
      <p class="muted">Memuat...</p>
    {/if}
    {#if st.status === 'error'}
      <div class="errbox" role="alert">
        <div>{errTitle}</div>
        <div class="acts">
          {#if needSettings}
            <button type="button" onclick={() => openSettings('connectors')}>Buka Pengaturan &gt; Connector</button>
          {/if}
          <button type="button" onclick={reload}>Coba lagi</button>
        </div>
      </div>
    {/if}
    {#if showSections || (st.status === 'loading' && hasContent)}
      <div class="group">
        <button type="button" class="gh" aria-expanded={open.tickets} onclick={() => toggle('tickets')}>
          <span class="chev">{open.tickets ? '▾' : '▸'}</span> Tiket
        </button>
        {#if open.tickets}<TicketSection />{/if}
      </div>
      <div class="group">
        <button type="button" class="gh" aria-expanded={open.wiki} onclick={() => toggle('wiki')}>
          <span class="chev">{open.wiki ? '▾' : '▸'}</span> Wiki
        </button>
        {#if open.wiki}<WikiSection />{/if}
      </div>
    {/if}
  </div>
</section>

<style>
  .lark {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    color: var(--fg, #ddd);
  }
  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px;
  }
  .title {
    font-size: 13px;
    letter-spacing: 0.08em;
    color: var(--fg-muted, #888);
  }
  .icon {
    background: none;
    border: none;
    color: var(--fg-muted, #888);
    cursor: pointer;
    font-size: 16px;
  }
  .icon:hover {
    color: var(--fg, #ddd);
  }
  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 0 4px 8px;
  }
  .muted {
    margin: 0;
    padding: 6px 10px;
    color: var(--fg-muted, #888);
    font-size: 14px;
  }
  .errbox {
    margin: 4px 6px 8px;
    padding: 8px 10px;
    background: var(--bg-elev, #222);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    color: var(--danger, #e49a9a);
    font-size: 14px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .acts {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
  .acts button {
    background: var(--bg-elev, #222);
    color: var(--fg, #ddd);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    padding: 2px 8px;
    font-size: 14px;
    cursor: pointer;
  }
  .acts button:hover {
    background: var(--bg-hover, #2c2c2c);
  }
  .gh {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 100%;
    background: none;
    border: none;
    color: var(--fg, #ddd);
    font-size: 14px;
    font-weight: 600;
    text-align: left;
    padding: 5px 8px;
    cursor: pointer;
  }
  .gh:hover {
    background: var(--bg-hover, #2c2c2c);
  }
  .chev {
    color: var(--fg-muted, #888);
    width: 10px;
  }
</style>
