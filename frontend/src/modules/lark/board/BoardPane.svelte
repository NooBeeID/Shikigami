<script lang="ts">
  import { untrack } from 'svelte';
  import { get } from 'svelte/store';
  import type { Leaf, Node } from '../../layout/model';
  import { layout } from '../../../lib/stores/layout';
  import { activeWorkspaceId } from '../../../lib/stores/workspaces';
  import { larkSources, larkState, refreshSources } from '../store';
  import { copyLink, sendToAgent } from '../actions';
  import {
    buildBaseLink,
    eligibleExtraFields,
    filterRecords,
    listTables,
    listTickets,
    listViews,
    MAX_EXTRA_FIELDS,
    mergeRecords,
    singleSelectFields,
    toLarkError,
    type BoardField,
    type BoardRecord,
    type LarkError,
    type TableInfo,
    type ViewInfo,
    type ViewMode,
  } from './boardClient';
  import BoardHeader from './BoardHeader.svelte';
  import Grid from './Grid.svelte';
  import Kanban from './Kanban.svelte';
  import TicketDrawer from './TicketDrawer.svelte';

  let { pane, visible }: { pane: Leaf; visible: boolean } = $props();

  type Phase = 'loading' | 'ready' | 'error';
  let phase = $state<Phase>('loading');
  let error = $state<LarkError | null>(null);
  let tables = $state<TableInfo[]>([]);
  let views = $state<ViewInfo[]>([]);
  let tableId = $state('');
  let viewId = $state('');
  let fields = $state<BoardField[]>([]);
  let records = $state<BoardRecord[]>([]);
  let nextOffset = $state<number | undefined>(undefined);
  let loadingMore = $state(false);
  let moreError = $state('');
  let search = $state('');
  let mode = $state<ViewMode>('grid');
  let groupFieldId = $state('');
  let extraFieldIds = $state<string[]>([]);
  let selectedRecordId = $state<string | null>(null); // WIRING: dipakai lark-tickets-wiring
  let seq = 0;
  let metaSeq = 0;
  let viewsSeq = 0;
  let extraTouched = false;
  let lastBootKey = '';

  const source = $derived($larkSources.tickets?.find((s) => s.id === pane.sourceId));
  const sourceKey = $derived(source ? source.id + '|' + source.baseToken : '');
  const wsId = $derived($activeWorkspaceId ?? '');
  const kanbanFields = $derived(singleSelectFields(fields));
  const canKanban = $derived(kanbanFields.length > 0);
  const effectiveMode = $derived<ViewMode>(mode === 'kanban' && canKanban ? 'kanban' : 'grid');
  const visibleRecords = $derived(filterRecords(records, search));
  const baseLink = $derived(source ? buildBaseLink(source, tableId, viewId) : '');

  function setError(e: unknown) {
    error = toLarkError(e);
    phase = 'error';
  }

  // Status sumber: hanya membaca larkState, source, wsId.
  $effect(() => {
    const st = $larkState;
    const src = source;
    const key = sourceKey + '|' + wsId;
    const ws = wsId;
    untrack(() => {
      if (!ws) {
        lastBootKey = '';
        error = { code: 'invalid_params', message: 'Workspace belum dipilih' };
        phase = 'error';
        return;
      }
      if (!src) {
        lastBootKey = '';
        if (st.status === 'loading') {
          phase = 'loading';
        } else if (st.status === 'error') {
          error = { code: st.code ?? 'unknown', message: st.message ?? 'Gagal memuat sumber' };
          phase = 'error';
        } else {
          error = { code: 'not_found', message: 'Sumber tidak ada di workspace ini' };
          phase = 'error';
        }
        return;
      }
      if (key !== lastBootKey) {
        lastBootKey = key;
        void bootstrap();
      }
    });
  });

  async function bootstrap(keep?: { tableId: string; viewId: string }) {
    const src = source;
    if (!src) return;
    const my = ++metaSeq;
    ++seq;
    phase = 'loading';
    error = null;
    moreError = '';
    loadingMore = false;
    if (!keep) {
      records = [];
      fields = [];
      nextOffset = undefined;
    }
    selectedRecordId = null;
    try {
      const list = await listTables({ workspaceId: wsId, sourceId: src.id });
      if (my !== metaSeq) return;
      tables = list;
      if (list.length === 0) {
        tableId = '';
        viewId = '';
        views = [];
        records = [];
        fields = [];
        nextOffset = undefined;
        phase = 'ready';
        return;
      }
      const has = (id: string | undefined) => !!id && list.some((t) => t.id === id);
      let nextTable: string;
      let nextView = '';
      if (keep && has(keep.tableId)) {
        nextTable = keep.tableId;
        nextView = keep.viewId;
      } else if (has(pane.tableId)) {
        nextTable = pane.tableId!;
        nextView = pane.viewId ?? '';
      } else if (has(src.tableId)) {
        nextTable = src.tableId!;
        nextView = src.viewId ?? '';
      } else {
        nextTable = list[0].id;
      }
      if (!keep || !has(keep.tableId)) extraTouched = false;
      tableId = nextTable;
      viewId = nextView;
      await loadViews(nextTable);
      if (my !== metaSeq) return;
      persistPane();
      await loadTickets(true);
    } catch (e) {
      if (my === metaSeq) setError(e);
    }
  }

  async function loadViews(tid: string) {
    const src = source;
    if (!src) return;
    const my = ++viewsSeq;
    try {
      const list = await listViews({ workspaceId: wsId, sourceId: src.id, tableId: tid });
      if (my !== viewsSeq || tid !== tableId) return;
      views = list;
      if (viewId && !list.some((v) => v.id === viewId)) {
        viewId = '';
        persistPane();
      }
    } catch {
      if (my !== viewsSeq || tid !== tableId) return;
      views = [];
    }
  }

  async function loadTickets(reset: boolean) {
    const src = source;
    if (!src || !tableId) return;
    let my = seq;
    if (reset) {
      my = ++seq;
      phase = 'loading';
      error = null;
      moreError = '';
      loadingMore = false;
    }
    try {
      const page = await listTickets({
        workspaceId: wsId,
        sourceId: src.id,
        tableId,
        viewId: viewId || undefined,
        offset: 0,
      });
      if (my !== seq) return;
      fields = page.fields;
      records = page.records;
      nextOffset = page.nextOffset;
      phase = 'ready';
      syncFieldSelections();
    } catch (e) {
      if (my === seq) setError(e);
    }
  }

  async function loadMore() {
    const src = source;
    if (!src || nextOffset === undefined || loadingMore || phase !== 'ready') return;
    const my = seq;
    loadingMore = true;
    moreError = '';
    try {
      const page = await listTickets({
        workspaceId: wsId,
        sourceId: src.id,
        tableId,
        viewId: viewId || undefined,
        offset: nextOffset,
      });
      if (my !== seq) return;
      records = mergeRecords(records, page.records);
      nextOffset = page.nextOffset;
      if (fields.length === 0 && page.fields.length > 0) {
        fields = page.fields;
        syncFieldSelections();
      }
    } catch (e) {
      if (my === seq) moreError = toLarkError(e).message;
    } finally {
      if (my === seq) loadingMore = false;
    }
  }

  function syncFieldSelections() {
    const ks = singleSelectFields(fields);
    if (!ks.some((f) => f.id === groupFieldId)) groupFieldId = ks[0]?.id ?? '';
    const eligible = eligibleExtraFields(fields);
    let ids = extraFieldIds.filter((id) => eligible.some((f) => f.id === id));
    if (ids.length === 0 && !extraTouched) {
      ids = eligible.filter((f) => f.id !== groupFieldId).slice(0, MAX_EXTRA_FIELDS).map((f) => f.id);
    }
    extraFieldIds = ids;
  }

  function onTable(id: string) {
    if (id === tableId) return;
    tableId = id;
    viewId = '';
    views = [];
    records = [];
    fields = [];
    nextOffset = undefined;
    search = '';
    selectedRecordId = null;
    extraTouched = false;
    extraFieldIds = [];
    persistPane();
    void loadViews(id);
    void loadTickets(true);
  }

  function onView(id: string) {
    viewId = id;
    records = [];
    nextOffset = undefined;
    selectedRecordId = null;
    persistPane();
    void loadTickets(true);
  }

  function onGroup(id: string) {
    groupFieldId = id;
    extraFieldIds = extraFieldIds.filter((x) => x !== id);
  }

  function onExtra(ids: string[]) {
    extraTouched = true;
    extraFieldIds = ids.slice(0, MAX_EXTRA_FIELDS);
  }

  function onRefresh() {
    void bootstrap({ tableId, viewId });
  }

  function onCopy() {
    if (baseLink) void copyLink(baseLink);
  }
  function onSend() {
    if (baseLink) void sendToAgent(baseLink);
  }

  // Titik sambung wiring.
  function onOpenRecord(recordId: string) {
    selectedRecordId = recordId;
  }

  function retry() {
    if (!source) {
      if (!wsId) return;
      void refreshSources();
      return;
    }
    if (tables.length === 0 || tableId === '') void bootstrap();
    else void loadTickets(true);
  }

  // Layout store tidak punya setter props leaf; solusi lokal lewat renamePane yang memancarkan store.
  function persistPane() {
    try {
      const find = (n: Node | null): Leaf | null => {
        if (!n) return null;
        if (n.type === 'pane') return n.id === pane.id ? n : null;
        return find(n.a) ?? find(n.b);
      };
      const leaf = find(get(layout).root);
      if (!leaf) return;
      if (leaf.tableId === tableId && (leaf.viewId ?? '') === viewId) return;
      leaf.tableId = tableId;
      if (viewId) leaf.viewId = viewId;
      else delete leaf.viewId;
      layout.renamePane(pane.id, leaf.title || 'Tiket Lark');
    } catch {
      /* persist gagal tidak boleh mengganggu pane */
    }
  }
</script>

<div class="lark-pane" hidden={!visible}>
  {#if phase !== 'error' && source}
    <BoardHeader
      {tables}
      {tableId}
      {views}
      {viewId}
      {mode}
      {canKanban}
      {search}
      {kanbanFields}
      {groupFieldId}
      extraCandidates={eligibleExtraFields(fields)}
      {extraFieldIds}
      loading={phase === 'loading'}
      {loadingMore}
      hasMore={nextOffset !== undefined}
      loadedCount={records.length}
      shownCount={visibleRecords.length}
      linkAvailable={baseLink !== ''}
      {onTable}
      {onView}
      onMode={(m) => (mode = m)}
      onSearch={(q) => (search = q)}
      {onGroup}
      {onExtra}
      onLoadMore={() => void loadMore()}
      {onRefresh}
      {onCopy}
      {onSend}
    />
  {/if}
  {#if moreError}
    <div class="more-error">Gagal memuat halaman berikutnya: {moreError}</div>
  {/if}
  <div class="body">
    {#if phase === 'error'}
      <div class="center" role="alert">
        <p>{error?.message ?? 'Terjadi galat'}</p>
        <button type="button" class:secondary={error?.code === 'not_found'} onclick={retry}>Coba lagi</button>
      </div>
    {:else if phase === 'loading' && records.length === 0}
      <div class="center" role="status">Memuat tiket…</div>
    {:else if phase === 'ready' && tables.length === 0}
      <div class="center">Base ini belum punya tabel</div>
    {:else if phase === 'ready' && records.length === 0}
      <div class="center">Tidak ada tiket</div>
    {:else if phase === 'ready' && visibleRecords.length === 0}
      <div class="center">
        <p>Tidak ada tiket yang cocok dengan pencarian</p>
        {#if nextOffset !== undefined}
          <p class="note">Pencarian hanya menyaring tiket yang sudah dimuat</p>
        {/if}
      </div>
    {:else if effectiveMode === 'kanban'}
      <Kanban {fields} records={visibleRecords} {groupFieldId} {extraFieldIds} onOpen={onOpenRecord} {selectedRecordId} />
    {:else}
      <Grid {fields} records={visibleRecords} {extraFieldIds} onOpen={onOpenRecord} {selectedRecordId} />
    {/if}
  </div>
  {#if selectedRecordId && source && tableId}
    <TicketDrawer
      {source}
      {tableId}
      viewId={viewId || undefined}
      recordId={selectedRecordId}
      onClose={() => (selectedRecordId = null)}
    />
  {/if}
</div>

<style>
  .lark-pane { flex: 1; min-height: 0; min-width: 0; display: flex; flex-direction: column; position: relative; background: var(--bg, transparent); color: var(--fg, inherit); }
  .lark-pane[hidden] { display: none; }
  .body { flex: 1; min-height: 0; overflow: hidden; display: flex; }
  .center { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; font-size: 12px; color: var(--fg-muted, #888); text-align: center; padding: 12px; }
  .center p { margin: 0; }
  .note { font-size: 11px; }
  .more-error { padding: 4px 8px; font-size: 11px; color: var(--fg-muted, #888); border-bottom: 1px solid var(--border, #8884); }
  button { font: inherit; font-size: 11px; border: 1px solid var(--border, #8884); border-radius: 4px; padding: 2px 8px; background: transparent; color: var(--fg, inherit); cursor: pointer; }
  button:hover { background: var(--bg-hover, #0001); }
  button.secondary { color: var(--fg-muted, #888); }
</style>
