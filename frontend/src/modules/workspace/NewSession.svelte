<script lang="ts">
  import { providers, activeProject, createSession, openTab } from '../../lib/stores';
  import { errMessage } from './errors';

  let { onerror }: { onerror: (msg: string) => void } = $props();

  let providerId = $state('');
  let busy = $state(false);

  const usable = $derived($providers.filter((p) => p.available !== false));

  $effect(() => {
    if (!usable.some((p) => p.id === providerId)) providerId = usable[0]?.id ?? '';
  });

  const hint = $derived(!$activeProject ? 'Tambah project dulu' : usable.length === 0 ? 'Provider tidak tersedia' : '');
  const disabled = $derived(!$activeProject || !providerId || busy);

  async function start() {
    busy = true;
    try {
      const s = await createSession(providerId);
      const p = $providers.find((x) => x.id === s.providerId);
      openTab({ type: 'session', refId: s.id, title: p?.name ?? s.providerId });
    } catch (e) {
      onerror(errMessage(e));
    } finally {
      busy = false;
    }
  }
</script>

<div class="new">
  <select aria-label="Provider" bind:value={providerId} disabled={busy}>
    {#each $providers as p (p.id)}
      <option value={p.id} disabled={p.available === false}>{p.name}{p.available === false ? ' (tidak tersedia)' : ''}</option>
    {/each}
  </select>
  <button type="button" onclick={start} {disabled} title={hint || 'Sesi baru'}>Sesi baru</button>
  {#if hint}<div class="hint">{hint}</div>{/if}
</div>

<style>
  .new { display: flex; flex-wrap: wrap; gap: 4px; align-items: center; min-width: 0; }
  select { flex: 1; min-width: 0; font: inherit; color: var(--fg, #222); background: var(--bg-elev, transparent); border: 1px solid var(--border, #8884); border-radius: 4px; padding: 3px 6px; }
  button { font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 1px solid var(--accent, #3b82f6); border-radius: 4px; padding: 3px 10px; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--bg-hover, #0001); }
  button:disabled { opacity: 0.5; cursor: default; border-color: var(--border, #8884); }
  .hint { flex-basis: 100%; font-size: 13px; color: var(--fg-muted, #888); }
</style>
