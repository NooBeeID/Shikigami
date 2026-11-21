import { get } from 'svelte/store';
import { activeProjectId } from '../../lib/stores';
import { toggleLeft, toggleRight } from '../../lib/stores/shell';
import { createAndOpenSession, openFolder, zoom, tidyGrid } from '../shell';
import { registerPaletteAction } from './registry';

const PROVIDERS = ['claude', 'codex', 'shell'];

export function registerBuiltinActions(): void {
  for (const providerId of PROVIDERS) {
    registerPaletteAction({
      id: `session.new.${providerId}`,
      title: `Sesi baru: ${providerId}`,
      hint: providerId,
      keywords: ['new', 'session', 'sesi', 'agent'],
      run: async () => {
        const projectId = get(activeProjectId);
        if (!projectId) {
          openFolder();
          return;
        }
        try {
          await createAndOpenSession(providerId, projectId);
        } catch {
          // galat ditampilkan modul lain bila ada; palet diam
        }
      },
    });
  }
  registerPaletteAction({ id: 'folder.open', title: 'Open Folder…', keywords: ['folder', 'project', 'buka'], run: openFolder });
  registerPaletteAction({ id: 'zoom.in', title: 'Zoom in', hint: '⌘+', run: () => zoom('in') });
  registerPaletteAction({ id: 'zoom.out', title: 'Zoom out', hint: '⌘−', run: () => zoom('out') });
  registerPaletteAction({ id: 'zoom.reset', title: 'Zoom reset', hint: '⌘0', run: () => zoom('reset') });
  registerPaletteAction({ id: 'sidebar.left.toggle', title: 'Toggle sidebar kiri', hint: '⌘B', keywords: ['sidebar', 'agent'], run: toggleLeft });
  registerPaletteAction({ id: 'sidebar.right.toggle', title: 'Toggle sidebar kanan', hint: '⌘⌥B', keywords: ['sidebar', 'file'], run: toggleRight });
  registerPaletteAction({ id: 'layout.tidy', title: 'Susun grid', keywords: ['layout', 'tidy', 'grid'], run: tidyGrid });
}
