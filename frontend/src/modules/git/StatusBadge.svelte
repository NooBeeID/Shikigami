<script lang="ts">
  type Kind = 'pipeline' | 'review' | 'mr';
  type Tone = 'good' | 'bad' | 'accent' | 'warn' | 'neutral';
  type Entry = { label: string; glyph: string; tone: Tone };

  let { status, kind }: { status: string; kind?: Kind } = $props();

  const TABLES: Record<Kind, Record<string, Entry>> = {
    pipeline: {
      pending: { label: 'Menunggu', glyph: '◌', tone: 'warn' },
      running: { label: 'Berjalan', glyph: '●', tone: 'accent' },
      success: { label: 'Berhasil', glyph: '✓', tone: 'good' },
      failed: { label: 'Gagal', glyph: '✕', tone: 'bad' },
      canceled: { label: 'Dibatalkan', glyph: '⊘', tone: 'neutral' },
      skipped: { label: 'Dilewati', glyph: '–', tone: 'neutral' },
      unknown: { label: 'Tidak diketahui', glyph: '?', tone: 'neutral' },
    },
    review: {
      approved: { label: 'Disetujui', glyph: '✓', tone: 'good' },
      changes_requested: { label: 'Perlu perubahan', glyph: '✕', tone: 'bad' },
      review_required: { label: 'Belum direview', glyph: '◌', tone: 'warn' },
    },
    mr: {
      open: { label: 'Terbuka', glyph: '○', tone: 'accent' },
      merged: { label: 'Digabung', glyph: '✓', tone: 'good' },
      closed: { label: 'Tertutup', glyph: '⊘', tone: 'neutral' },
    },
  };
  const ORDER: Kind[] = ['pipeline', 'review', 'mr'];

  let effKind = $derived<Kind | undefined>(
    kind ?? ORDER.find((k) => Object.prototype.hasOwnProperty.call(TABLES[k], status)),
  );
  let entry = $derived<Entry>(
    (effKind && Object.prototype.hasOwnProperty.call(TABLES[effKind], status)
      ? TABLES[effKind][status]
      : undefined) ?? { label: status, glyph: '–', tone: 'neutral' },
  );
</script>

<span class="badge tone-{entry.tone}" title={entry.label}><span class="glyph" aria-hidden="true">{entry.glyph}</span>{entry.label}</span>

<style>
  .badge {
    --c: var(--fg-muted, #888);
    display: inline-flex; align-items: center; gap: 3px;
    font-size: 12px; padding: 0 6px; border-radius: 8px; white-space: nowrap;
    color: var(--c);
    background: color-mix(in srgb, var(--c) 16%, transparent);
    border: 1px solid color-mix(in srgb, var(--c) 35%, transparent);
  }
  .tone-good { --c: var(--success, #4caf7a); }
  .tone-bad { --c: var(--danger, #e49a9a); }
  .tone-accent { --c: var(--accent, #6aa8e8); }
  .tone-warn { --c: var(--warn, #d9a441); }
  .tone-neutral { --c: var(--fg-muted, #888); }
  .glyph { font-size: 11px; line-height: 1; }
</style>
