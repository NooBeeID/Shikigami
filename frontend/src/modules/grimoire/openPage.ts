import { get } from 'svelte/store';
import { layout } from '../../lib/stores/layout';
import { findTab } from '../layout/model';
import { call } from '../../lib/ipc';

export async function openGrimoirePage(projectId: string, relPath: string): Promise<string> {
  if (!projectId || !relPath) throw new Error('projectId dan path halaman wajib diisi');
  if (relPath.startsWith('/') || relPath.split('/').includes('..')) throw new Error('path halaman tidak valid');

  const st = await call<{ dir?: string }>('grimoire.status', { projectId });
  if (!st?.dir) throw new Error('direktori grimoire tidak tersedia; pastikan daemon sudah diperbarui');

  const dir = st.dir.replace(/\/+$/, '');
  const filePath = dir + '/' + relPath;

  const existing = findTab(get(layout), (t) => t.kind === 'file' && t.filePath === filePath && t.projectId === projectId);
  if (existing) {
    layout.activateTab(existing.pane.id, existing.tab.id);
    return existing.pane.id;
  }

  const title = (relPath.split('/').pop() ?? relPath).replace(/\.md$/, '');
  return layout.openPane({ kind: 'file', projectId, filePath, root: dir, readOnly: true, title });
}
