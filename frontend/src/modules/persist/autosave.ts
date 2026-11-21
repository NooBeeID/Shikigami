import { get } from 'svelte/store';
import { call } from '../../lib/ipc';
import { layout } from '../../lib/stores/layout';
import { leftSidebar, rightSidebar } from '../../lib/stores/shell';
import { activeWorkspaceId } from '../../lib/stores/workspaces';
import { flushNow, markSaved, persistStatus, setFlushHandler, setPersistStatus } from '../../lib/stores/persist';
import { saveWindowNow, startWindowWatch } from './windowState';

let currentWsId: string | null = null;
let currentGroupId = '';
let lastLayoutJSON = '';
let lastUiJSON = '';
let writesEnabled = false;
let timer: ReturnType<typeof setTimeout> | null = null;
let retryTimer: ReturnType<typeof setTimeout> | null = null;
let writeChain: Promise<void> = Promise.resolve();

export function getCurrentWsId(): string | null {
  return currentWsId;
}

export function getCurrentGroupId(): string {
  return currentGroupId;
}

// Ganti layout aktif ke grup lain. Pemanggil sudah menjalankan flushNow() sebelumnya.
// Sinkron: hydrate lalu baseline, sehingga autosave berikutnya menulis ke grup baru dan
// tidak menganggap hasil hydrate sebagai perubahan. Timer retry sengaja tidak dibersihkan:
// doWrite menangkap groupId per tulis, jadi tulis ulang grup lama tetap menuju grup lama.
export function switchGroup(groupId: string, doc: unknown): void {
  layout.hydrate(doc);
  setBaseline(currentWsId, groupId);
}

export function uiPayload() {
  return {
    leftSidebar: { ...get(leftSidebar) },
    rightSidebar: { ...get(rightSidebar) },
    activeWorkspaceId: currentWsId ?? get(activeWorkspaceId) ?? '',
  };
}

export function setBaseline(wsId: string | null, groupId: string): void {
  currentWsId = wsId;
  currentGroupId = groupId;
  lastLayoutJSON = JSON.stringify(layout.serialize());
  lastUiJSON = JSON.stringify(uiPayload());
}

function clearTimers(): void {
  if (timer) clearTimeout(timer);
  if (retryTimer) clearTimeout(retryTimer);
  timer = null;
  retryTimer = null;
}

export function enableWrites(on: boolean): void {
  writesEnabled = on;
  if (!on) clearTimers();
}

export function scheduleSave(): void {
  if (!writesEnabled || get(persistStatus) === 'restoring') return;
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => {
    timer = null;
    void flush();
  }, 500);
}

type Ui = ReturnType<typeof uiPayload>;

async function doWrite(
  wsId: string | null,
  groupId: string,
  layoutJSON: string,
  ui: Ui,
  uiJSON: string,
  attempt = 0,
): Promise<void> {
  try {
    if (wsId && layoutJSON !== lastLayoutJSON) {
      await call('workspace.layout.set', {
        workspaceId: wsId,
        ...(groupId ? { groupId } : {}),
        layout: JSON.parse(layoutJSON),
      });
      if (wsId === currentWsId) lastLayoutJSON = layoutJSON;
      markSaved();
    }
    if (uiJSON !== lastUiJSON) {
      await call('workspace.ui.set', ui);
      lastUiJSON = uiJSON;
      markSaved();
    }
    if (get(persistStatus) === 'error') setPersistStatus('ready');
  } catch (e) {
    if (attempt === 0) {
      retryTimer = setTimeout(() => {
        retryTimer = null;
        writeChain = writeChain.then(() => doWrite(wsId, groupId, layoutJSON, ui, uiJSON, 1)).catch(() => {});
      }, 2000);
    } else {
      setPersistStatus('error');
      console.warn('[persist] simpan gagal', e);
    }
  }
}

export async function flush(): Promise<void> {
  if (timer) clearTimeout(timer);
  if (retryTimer) clearTimeout(retryTimer);
  timer = null;
  retryTimer = null;
  if (!writesEnabled || get(persistStatus) === 'restoring') return;

  const wsId = currentWsId;
  const groupId = currentGroupId;
  const layoutJSON = JSON.stringify(layout.serialize());
  const ui = uiPayload();
  const uiJSON = JSON.stringify(ui);

  writeChain = writeChain.then(() => doWrite(wsId, groupId, layoutJSON, ui, uiJSON)).catch(() => {});
  await writeChain;
  await saveWindowNow();
}

export function startWatching(): () => void {
  const unsubs: Array<() => void> = [
    layout.subscribe(scheduleSave),
    leftSidebar.subscribe(scheduleSave),
    rightSidebar.subscribe(scheduleSave),
    activeWorkspaceId.subscribe(scheduleSave),
  ];
  const onVis = () => {
    if (document.visibilityState === 'hidden') void flushNow();
  };
  const onUnload = () => {
    void flushNow();
  };
  document.addEventListener('visibilitychange', onVis);
  window.addEventListener('beforeunload', onUnload);
  const stopWindow = startWindowWatch();
  setFlushHandler(flush);
  return () => {
    unsubs.forEach((u) => u());
    document.removeEventListener('visibilitychange', onVis);
    window.removeEventListener('beforeunload', onUnload);
    stopWindow();
    setFlushHandler(null);
  };
}
