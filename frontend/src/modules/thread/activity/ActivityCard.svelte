<script lang="ts">
  import { untrack } from 'svelte';

  interface Props {
    agentName: string;
    color: string;
    items: string[];
    count: number;
    running: boolean;
    initialOpen?: boolean;
    onToggle?: (open: boolean) => void;
  }
  let { agentName, color, items, count, running, initialOpen = false, onToggle }: Props = $props();

  let open = $state(untrack(() => initialOpen));
  const listId = `act-${Math.random().toString(36).slice(2, 9)}`;

  function toggle() {
    open = !open;
    onToggle?.(open);
  }
</script>

<div class="card">
  <button type="button" class="head" aria-expanded={open} aria-controls={listId} onclick={toggle}>
    <span class="dot" class:run={running} style:background={color} aria-hidden="true"></span>
    <span class="label">{agentName}: {count} aksi</span>
    <span class="status" class:run={running}>{running ? 'mengerjakan' : 'selesai'}</span>
    <span class="caret" aria-hidden="true">{open ? '▾' : '▸'}</span>
  </button>
  {#if open}
    <ul id={listId} class="items">
      {#if count > items.length && items.length > 0}
        <li class="more">… {count - items.length} aksi lebih lama tidak ditampilkan</li>
      {/if}
      {#each items as it, i (i)}
        <li class="item" title={it}>{it}</li>
      {:else}
        <li class="item empty">Tidak ada rincian</li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .card {
    border: 1px solid var(--border, #8884);
    border-radius: 6px;
    background: var(--bg-elev, #fff);
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
    margin: 2px 8px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    font: inherit;
    font-size: 12px;
    color: var(--fg, #222);
    background: none;
    border: 0;
    padding: 6px 8px;
    cursor: pointer;
    text-align: left;
    min-width: 0;
  }
  .head:focus-visible {
    outline: 2px solid currentColor;
    outline-offset: -2px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }
  .dot.run {
    animation: pulse 1.2s ease-in-out infinite;
  }
  .label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 600;
  }
  .status {
    font-size: 11px;
    color: var(--fg-muted, #888);
    flex: none;
  }
  .status.run {
    color: var(--fg, #222);
  }
  .caret {
    flex: none;
    color: var(--fg-muted, #888);
  }
  .items {
    list-style: none;
    margin: 0;
    padding: 0 0 4px;
    min-width: 0;
  }
  .item,
  .more {
    font-size: 12px;
    color: var(--fg-muted, #888);
    padding: 2px 8px 2px 24px;
    min-width: 0;
  }
  .item {
    font-family: ui-monospace, monospace;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .more {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    font-style: italic;
  }
  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.35;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .dot.run {
      animation: none;
    }
  }
</style>
