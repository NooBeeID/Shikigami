import { writable, type Writable } from 'svelte/store';
import { call, onEvent } from '../ipc';
import { loadWorkspaces } from './workspaces';
import { loadProviders } from './providers';
import { loadSessions, sessions } from './sessions';

export const connection: Writable<{ state: 'connecting' | 'ok' | 'error'; message: string }> =
  writable({ state: 'connecting' as 'connecting' | 'ok' | 'error', message: '' });

// FINAL(shell-layout)
export async function initStores(): Promise<() => void> {
  connection.set({ state: 'connecting', message: '' });
  try {
    await call('ping');
  } catch (e) {
    connection.set({ state: 'error', message: e instanceof Error ? e.message : String(e) });
    return () => {};
  }

  // Hanya ping yang menentukan error koneksi; method yang belum ada diabaikan.
  try { await loadWorkspaces(); } catch { /* daemon belum punya workspace.* */ }
  try { await loadProviders(); } catch { /* daemon belum punya provider.list */ }
  try { await loadSessions(); } catch { /* abaikan */ }

  const off1 = onEvent('session.exit', (_t, p) => {
    if (!p || typeof p !== 'object') return;
    const { id, exitCode } = p as { id?: unknown; exitCode?: unknown };
    if (typeof id !== 'string') return;
    sessions.update((l) =>
      l.map((s) =>
        s.id === id
          ? { ...s, status: 'exited', exitCode: typeof exitCode === 'number' ? exitCode : s.exitCode }
          : s,
      ),
    );
  });
  // Event updated (termasuk dari workspace.touch) tidak mengaktifkan workspace dan tidak mengubah openWorkspaceIds.
  const off2 = onEvent('workspace.changed', () => {
    void loadWorkspaces().catch(() => {});
  });

  connection.set({ state: 'ok', message: '' });
  return () => {
    off1();
    off2();
  };
}
