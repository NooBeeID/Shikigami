export type FsEntryType = 'file' | 'dir' | 'symlink';
export interface FsEntry { name: string; type: FsEntryType; size: number; mtime: number }
export interface FileTabData { path: string; projectId: string; workspaceId: string }
export interface ReadResultRaw { content: string; mtime: number; kind: 'markdown' | 'text'; size: number; offset: number; length: number; eof: boolean }
export interface FileChangedEvent { root: string; path: string; op: 'write' | 'remove' | 'rename' | 'create'; mtime: number }

export const CHUNK_SIZE = 256 * 1024;
