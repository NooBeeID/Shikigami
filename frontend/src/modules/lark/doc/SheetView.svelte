<script lang="ts">
  let { cells, truncated }: { cells: string[][]; truncated: boolean } = $props();

  const colCount = $derived(Math.max(0, ...cells.map((r) => r.length)));
  const cols = $derived(Array.from({ length: colCount }, (_, i) => i));

  function colLabel(i: number): string {
    let n = i + 1;
    let s = '';
    while (n > 0) {
      const r = (n - 1) % 26;
      s = String.fromCharCode(65 + r) + s;
      n = Math.floor((n - 1) / 26);
    }
    return s;
  }
</script>

<div class="sheet">
  {#if cells.length === 0}
    <p class="empty" role="status">Sheet kosong</p>
  {:else}
    <div class="scroll">
      <table aria-label="Isi sheet">
        <thead>
          <tr>
            <th class="corner"></th>
            {#each cols as c}<th scope="col">{colLabel(c)}</th>{/each}
          </tr>
        </thead>
        <tbody>
          {#each cells as row, ri}
            <tr>
              <th scope="row">{ri + 1}</th>
              {#each cols as c}<td>{row[c] ?? ''}</td>{/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
  {#if truncated}
    <p class="note">Menampilkan 50 baris x 20 kolom pertama</p>
  {/if}
</div>

<style>
  .sheet { flex: 1; min-height: 0; min-width: 0; display: flex; flex-direction: column; color: var(--fg, inherit); }
  .scroll { flex: 1; min-height: 0; overflow: auto; max-height: 100%; }
  table { border-collapse: separate; border-spacing: 0; font-size: 14px; }
  th, td { border-right: 1px solid var(--border, currentColor); border-bottom: 1px solid var(--border, currentColor); padding: 3px 8px; text-align: left; vertical-align: top; }
  td { white-space: pre-wrap; overflow-wrap: anywhere; max-width: 32ch; }
  th { background: var(--bg-elev, var(--bg, Canvas)); color: var(--fg-muted, inherit); font-weight: 600; }
  thead th { position: sticky; top: 0; z-index: 2; }
  tbody th { position: sticky; left: 0; z-index: 1; text-align: right; }
  thead th.corner { left: 0; z-index: 3; }
  .empty, .note { margin: 0; padding: 8px 10px; color: var(--fg-muted, inherit); font-size: 14px; }
</style>
