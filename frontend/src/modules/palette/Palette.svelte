<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { paletteOpen, closePalette, togglePalette } from './state';
  import { paletteActions, paletteSources, type PaletteItem, type PaletteSourceKind } from './registry';
  import { fuzzyFilter, GROUP_LIMIT } from './fuzzy';

  interface Group { kind: PaletteSourceKind; label: string; items: PaletteItem[] }

  const ORDER: { kind: PaletteSourceKind; label: string }[] = [
    { kind: 'action', label: 'Aksi' },
    { kind: 'session', label: 'Sesi' },
    { kind: 'project', label: 'Project' },
    { kind: 'file', label: 'File' },
  ];

  let query = $state('');
  let selected = $state(0);
  let groups = $state<Group[]>([]);
  let inputEl = $state<HTMLInputElement | null>(null);
  let listEl = $state<HTMLDivElement | null>(null);
  let previousFocus: HTMLElement | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let token = 0;
  let wasOpen = false;

  const flat = $derived(groups.flatMap((g) => g.items));

  function onGlobalKey(e: KeyboardEvent) {
    if (e.metaKey && !e.altKey && !e.shiftKey && !e.ctrlKey && e.code === 'KeyK') {
      e.preventDefault();
      e.stopPropagation();
      togglePalette();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', onGlobalKey, true);
    return () => {
      window.removeEventListener('keydown', onGlobalKey, true);
      clearTimer();
    };
  });

  function clearTimer() {
    if (timer !== null) { clearTimeout(timer); timer = null; }
  }

  async function search() {
    const my = ++token;
    const q = query;
    const buckets = new Map<PaletteSourceKind, PaletteItem[]>();
    const push = (kind: PaletteSourceKind, items: PaletteItem[]) => {
      buckets.set(kind, [...(buckets.get(kind) ?? []), ...items]);
    };
    const actionItems: PaletteItem[] = [];
    const extra = new Map<string, string>();
    for (const a of $paletteActions) {
      const id = 'action:' + a.id;
      actionItems.push({ id, kind: 'action', title: a.title, subtitle: a.hint, run: a.run });
      extra.set(id, (a.keywords ?? []).join(' '));
    }
    push('action', fuzzyFilter(actionItems, q, (it) => it.title + ' ' + (extra.get(it.id) ?? ''), GROUP_LIMIT));
    const sources = $paletteSources;
    const results = await Promise.allSettled(sources.map((s) => Promise.resolve(s.list(q))));
    if (my !== token) return;
    results.forEach((r, i) => {
      if (r.status === 'fulfilled') push(sources[i].kind, r.value);
    });
    const next: Group[] = [];
    for (const g of ORDER) {
      const items = (buckets.get(g.kind) ?? []).slice(0, GROUP_LIMIT);
      if (items.length) next.push({ ...g, items });
    }
    groups = next;
    const total = next.reduce((n, g) => n + g.items.length, 0);
    selected = Math.max(0, Math.min(selected, total - 1));
  }

  async function onOpened() {
    previousFocus = document.activeElement as HTMLElement | null;
    query = '';
    selected = 0;
    void search();
    await tick();
    inputEl?.focus();
    clearTimer();
    timer = setTimeout(() => { timer = null; void search(); }, 250);
  }

  function onClosed() {
    clearTimer();
    token++;
    if (previousFocus?.isConnected) previousFocus.focus();
    previousFocus = null;
  }

  $effect(() => {
    const open = $paletteOpen;
    if (open && !wasOpen) void onOpened();
    else if (!open && wasOpen) onClosed();
    wasOpen = open;
  });

  async function choose(item: PaletteItem | undefined) {
    if (!item) return;
    closePalette();
    await tick();
    try { await Promise.resolve(item.run()); } catch { /* galat ditelan */ }
  }

  function scrollSelected() {
    void tick().then(() => {
      listEl?.querySelector(`#palette-opt-${selected}`)?.scrollIntoView({ block: 'nearest' });
    });
  }

  function onKeydown(e: KeyboardEvent) {
    const n = flat.length;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      e.stopPropagation();
      if (n > 0) {
        selected = e.key === 'ArrowDown' ? (selected + 1) % n : (selected - 1 + n) % n;
        scrollSelected();
      }
    } else if (e.key === 'Enter') {
      e.preventDefault();
      e.stopPropagation();
      void choose(flat[selected]);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      closePalette();
    } else if (e.key === 'Tab') {
      e.preventDefault();
      e.stopPropagation();
    }
  }

  function onInput() {
    selected = 0;
    void search();
  }

  function onBackdropDown(e: PointerEvent) {
    if (e.target === e.currentTarget) closePalette();
  }
</script>

{#if $paletteOpen}
  <div class="backdrop" role="presentation" onpointerdown={onBackdropDown}>
    <div class="panel" role="dialog" aria-modal="true" aria-label="Palet perintah">
      <input
        class="input"
        bind:this={inputEl}
        bind:value={query}
        oninput={onInput}
        onkeydown={onKeydown}
        placeholder="Ketik perintah, sesi, project, atau file…"
        role="combobox"
        aria-expanded="true"
        aria-controls="palette-list"
        aria-activedescendant={flat.length ? `palette-opt-${selected}` : undefined}
        autocomplete="off"
        spellcheck="false"
      />
      <div class="list" id="palette-list" role="listbox" bind:this={listEl}>
        {#if flat.length === 0}
          <div class="empty">Tidak ada hasil</div>
        {:else}
          {#each groups as g, gi (g.kind)}
            <div class="group-label">{g.label}</div>
            {#each g.items as item, ii (item.id)}
              {@const idx = groups.slice(0, gi).reduce((n, x) => n + x.items.length, 0) + ii}
              <div
                class="row"
                class:selected={idx === selected}
                role="option"
                id={`palette-opt-${idx}`}
                aria-selected={idx === selected}
                tabindex="-1"
                onclick={() => choose(item)}
                onkeydown={() => {}}
                onmousemove={() => { if (selected !== idx) selected = idx; }}
              >
                <span class="title">{item.title}</span>
                {#if g.kind === 'action'}
                  {#if item.subtitle}<span class="badge">{item.subtitle}</span>{/if}
                {:else if item.subtitle}
                  <span class="subtitle">{item.subtitle}</span>
                {/if}
              </div>
            {/each}
          {/each}
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 1000;
    background: rgba(0, 0, 0, 0.35);
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 12vh;
  }
  .panel {
    width: min(560px, calc(100vw - 32px));
    max-height: 60vh;
    background: var(--bg-elev);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: 10px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.45);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .input {
    border: none;
    border-bottom: 1px solid var(--border);
    border-radius: 0;
    padding: 12px 14px;
    background: transparent;
    color: var(--fg);
    outline: none;
    font-size: 14px;
  }
  .list { overflow-y: auto; padding: 4px 0 6px; }
  .group-label {
    padding: 8px 14px 4px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--fg-muted);
  }
  .row {
    display: flex;
    align-items: baseline;
    gap: 10px;
    padding: 6px 14px;
    border-left: 2px solid transparent;
    cursor: pointer;
    min-width: 0;
  }
  .row.selected { background: var(--bg-hover); border-left-color: var(--accent); }
  .title { white-space: nowrap; flex-shrink: 0; max-width: 60%; overflow: hidden; text-overflow: ellipsis; }
  .subtitle {
    color: var(--fg-muted);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .badge {
    margin-left: auto;
    font-size: 11px;
    color: var(--fg-muted);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0 6px;
  }
  .empty { padding: 16px 14px; color: var(--fg-muted); }
</style>
