<script lang="ts">
  import { tick } from 'svelte';
  import { providers, activeProjectId } from '../../lib/stores';
  import { createAndOpenSession } from './helpers';
  import { newSessionPicker, closeNewSessionPicker } from './picker';

  const ORDER = ['shell', 'claude', 'codex'];

  let index = $state(0);
  let busy = $state(false);
  let error = $state('');
  let box: HTMLElement | undefined = $state();

  const items = $derived(
    ORDER.map((id) => $providers.find((p) => p.id === id)).filter((p): p is NonNullable<typeof p> => !!p),
  );

  $effect(() => {
    if (!$newSessionPicker) return;
    index = Math.max(0, items.findIndex((p) => p.id === 'claude'));
    error = '';
    void tick().then(() => box?.focus());
  });

  async function pick(i: number) {
    const p = items[i];
    const projectId = $activeProjectId;
    if (!p || busy || !projectId) return;
    if (p.available === false) {
      error = `${p.name} tidak tersedia`;
      return;
    }
    busy = true;
    try {
      await createAndOpenSession(p.id, projectId);
      closeNewSessionPicker();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      closeNewSessionPicker();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      index = (index + 1) % items.length;
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      index = (index - 1 + items.length) % items.length;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      void pick(index);
    } else if (/^[1-9]$/.test(e.key) && Number(e.key) <= items.length) {
      e.preventDefault();
      void pick(Number(e.key) - 1);
    } else if (e.key === 'Tab') {
      e.preventDefault();
    }
  }
</script>

{#if $newSessionPicker}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div class="backdrop" onpointerdown={(e) => { if (e.target === e.currentTarget) closeNewSessionPicker(); }}>
    <div class="box" role="dialog" aria-label="Sesi baru" tabindex="-1" bind:this={box} onkeydown={onKey}>
      <div class="title">Sesi baru</div>
      {#each items as p, i (p.id)}
        <button
          type="button"
          class="item"
          class:sel={i === index}
          disabled={busy}
          onmouseenter={() => (index = i)}
          onclick={() => pick(i)}
        >
          <span>{p.name}{p.available === false ? ' (tidak tersedia)' : ''}</span>
          <kbd>{i + 1}</kbd>
        </button>
      {/each}
      {#if error}<div class="err">{error}</div>{/if}
    </div>
  </div>
{/if}

<style>
  .backdrop { position: fixed; inset: 0; z-index: 100; display: flex; align-items: flex-start; justify-content: center; padding-top: 18vh; background: #0005; }
  .box { width: 280px; display: flex; flex-direction: column; gap: 2px; padding: 8px; outline: none; background: var(--bg-elev, #fff); border: 1px solid var(--border, #8884); border-radius: 8px; box-shadow: 0 10px 30px #0006; }
  .title { font-size: 13px; color: var(--fg-muted, #888); padding: 2px 6px 6px; }
  .item { display: flex; justify-content: space-between; align-items: center; text-align: left; font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 0; border-radius: 5px; padding: 8px 10px; cursor: pointer; }
  .item.sel { background: var(--bg-hover, #0002); }
  .item:disabled { opacity: 0.5; cursor: default; }
  kbd { font: inherit; font-size: 12px; color: var(--fg-muted, #888); }
  .err { font-size: 13px; color: #e5484d; padding: 4px 6px; }
</style>
