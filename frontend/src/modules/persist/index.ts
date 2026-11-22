import { get } from 'svelte/store';
import { call } from '../../lib/ipc';
import { layout } from '../../lib/stores/layout';
import { setPersistStatus } from '../../lib/stores/persist';
import { activeWorkspaceId, loadWorkspaces, workspaces } from '../../lib/stores/workspaces';
import { enableWrites, flush, getCurrentWsId, scheduleSave, setBaseline, startWatching } from './autosave';
import { applyUi, restoreWorkspace } from './restore';

let started: Promise<void> | null = null;
let switching = false;

async function switchTo(): Promise<void> {
  switching = true;
  try {
    while (get(activeWorkspaceId) !== getCurrentWsId()) {
      const old = getCurrentWsId();
      const next = get(activeWorkspaceId);
      if (old && get(workspaces).some((w) => w.id === old)) await flush();
      setPersistStatus('restoring');
      enableWrites(false);
      try {
        await restoreWorkspace(next);
        enableWrites(true);
        setPersistStatus('ready');
      } catch (e) {
        console.warn('[persist] pindah workspace gagal', e);
        layout.hydrate(null);
        setBaseline(next, '');
        setPersistStatus('error');
      }
    }
  } finally {
    switching = false;
  }
  scheduleSave();
}

async function run(): Promise<void> {
  try {
    setPersistStatus('restoring');
    enableWrites(false);
    switching = true;

    let ui: any = null;
    try {
      ui = await call('workspace.ui.get');
    } catch {
      ui = null;
    }
    try {
      if (get(workspaces).length === 0) await loadWorkspaces();
    } catch {
      // lanjut dengan yang ada
    }
    const target: string | null = null; // start selalu kosong: tanpa workspace aktif
    applyUi(ui);

    try {
      await restoreWorkspace(target);
    } catch (e) {
      layout.hydrate(null);
      setBaseline(target, '');
      setPersistStatus('error');
      console.warn('[persist] pemulihan gagal', e);
      startWatching();
      switching = false;
      return;
    }

    enableWrites(true);
    setPersistStatus('ready');
    startWatching();
    let first = true;
    activeWorkspaceId.subscribe((id) => {
      if (first) {
        first = false;
        return;
      }
      if (switching || id === getCurrentWsId()) return;
      void switchTo();
    });
    switching = false;
  } catch (e) {
    switching = false;
    setPersistStatus('error');
    console.warn('[persist] startPersist gagal', e);
  }
}

export function startPersist(): Promise<void> {
  if (!started) started = run();
  return started;
}

import './register';
