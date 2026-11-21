import { fsWrite, FsConflictError } from '../shared/fsClient';

export class ConflictError extends Error {
  currentMtime?: number;
  constructor(message = 'File berubah di disk', currentMtime?: number) {
    super(message);
    this.name = 'ConflictError';
    this.currentMtime = currentMtime;
  }
}

export async function writeFile(path: string, content: string, expectedMtime: number): Promise<{ mtime: number }> {
  try {
    const r = await fsWrite(path, content, expectedMtime);
    return { mtime: r.mtime };
  } catch (e) {
    if (e instanceof FsConflictError) throw new ConflictError(e.message, e.currentMtime);
    throw e;
  }
}
