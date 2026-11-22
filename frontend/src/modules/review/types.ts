// Tipe frontend untuk review perubahan agent (kontrak: overview 2.5).
export type ReviewStatus = 'modified' | 'added' | 'deleted';
export interface ReviewFile { path: string; status: ReviewStatus; additions: number; deletions: number; reviewed: boolean; external: boolean; binary: boolean; tooLarge: boolean }
export interface ReviewFilesResult { supported: boolean; files: ReviewFile[] }
