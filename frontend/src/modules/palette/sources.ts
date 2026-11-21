import { get } from 'svelte/store';
import { sessions, activeWorkspace, activeWorkspaceId, activeProject, selectProject } from '../../lib/stores';
import { focusOrOpenSession, openFilePane, sessionTitle, basename } from '../shell';
import { TreeModel } from '../files/tree/treeModel.svelte';
import { fuzzyFilter, GROUP_LIMIT } from './fuzzy';
import { registerPaletteSource, type PaletteItem } from './registry';

const SKIP_DIRS = new Set(['node_modules', '.git', 'dist', 'build', 'vendor']);
const MAX_DIRS = 40;

// Batasan: tidak ada metode cari/daftar rekursif di lib/ipc atau fsClient, jadi
// pencarian file hanya mencakup file yang sudah dimuat TreeModel (root project
// aktif dan direktori tingkat pertamanya). Hasil bertambah seiring muat asinkron.
const model = new TreeModel();
let signature = '';
let expandedProject = '';

function syncFileModel(): { id: string; path: string; name: string } | null {
  const project = get(activeProject);
  if (!project) {
    if (signature !== '') {
      signature = '';
      expandedProject = '';
      model.setProjects([]);
    }
    return null;
  }
  const sig = `${project.id}:${project.path}`;
  if (sig !== signature) {
    signature = sig;
    expandedProject = '';
    model.setProjects([{ id: project.id, path: project.path, name: project.name }]);
  }
  const rows = model.rows;
  const root = rows.find((r) => r.kind === 'project');
  if (root && !root.expanded) model.toggle(root);
  if (root && expandedProject !== sig) {
    const dirs = rows.filter(
      (r) => r.kind === 'dir' && r.depth === 1 && !SKIP_DIRS.has(r.name) && !r.expanded,
    );
    if (root.expanded && rows.some((r) => r.depth === 1 && !r.placeholder)) {
      expandedProject = sig;
      for (const d of dirs.slice(0, MAX_DIRS)) model.toggle(d);
    }
  }
  return project;
}

export function registerBuiltinSources(): void {
  registerPaletteSource({
    id: 'builtin.projects',
    kind: 'project',
    list: (query) => {
      const items: PaletteItem[] = (get(activeWorkspace)?.projects ?? []).map((p) => ({
        id: 'project:' + p.id,
        kind: 'project',
        title: p.name,
        subtitle: p.path,
        run: () => selectProject(p.id),
      }));
      const byId = new Map((get(activeWorkspace)?.projects ?? []).map((p) => [p.id, p]));
      return fuzzyFilter(items, query, (it) => {
        const p = byId.get(it.id.slice('project:'.length));
        return p ? p.name + ' ' + p.path : it.title;
      }, GROUP_LIMIT);
    },
  });

  registerPaletteSource({
    id: 'builtin.sessions',
    kind: 'session',
    list: (query) => {
      const wsId = get(activeWorkspaceId);
      const projects = get(activeWorkspace)?.projects ?? [];
      const items: PaletteItem[] = get(sessions)
        .filter((s) => s.workspaceId === wsId)
        .map((s) => {
          const proj = projects.find((p) => p.id === s.projectId)?.name ?? basename(s.cwd);
          return {
            id: 'session:' + s.id,
            kind: 'session' as const,
            title: sessionTitle(s.id),
            subtitle: `${s.providerId} · ${proj} · ${s.status}`,
            run: () => { focusOrOpenSession(s.id); },
          };
        });
      return fuzzyFilter(items, query, (it) => it.title + ' ' + (it.subtitle ?? ''), GROUP_LIMIT);
    },
  });

  registerPaletteSource({
    id: 'builtin.files',
    kind: 'file',
    list: (query) => {
      const project = syncFileModel();
      if (!project || !query.trim()) return [];
      const items = model.rows
        .filter((r) => !r.placeholder && (r.kind === 'file' || r.kind === 'symlink'))
        .map((r) => {
          const rel = r.path.startsWith(r.root + '/') ? r.path.slice(r.root.length + 1) : r.path;
          return { row: r, rel };
        });
      return fuzzyFilter(items, query, (x) => x.rel, GROUP_LIMIT).map(({ row, rel }) => ({
        id: 'file:' + row.path,
        kind: 'file' as const,
        title: row.name,
        subtitle: rel,
        run: () => {
          openFilePane({
            projectId: project.id,
            filePath: row.path,
            workspaceId: get(activeWorkspaceId) ?? undefined,
            title: row.name,
          });
        },
      }));
    },
  });
}
