<script lang="ts">
  import { onMount } from 'svelte';
  import { activeWorkspaceId } from '../../lib/stores';
  import { inWails } from '../../lib/ipc';
  import { listNames } from './mention-client';
  import { pendingMessages, pendingCount, queueError, startQueueSync, doCancel, doConfirm } from './queue-store';

  let open = $state(false);
  let btn: HTMLButtonElement | undefined = $state();
  let popoverEl: HTMLDivElement | undefined = $state();
  let pos = $state({ left: 0, bottom: 0 });
  let nameById = $state<Record<string, string>>({});
  let rowBusy = $state<Record<string, boolean>>({});
  let rowError = $state<Record<string, string>>({});

  onMount(() => startQueueSync());

  const hasAwaiting = $derived($pendingMessages.some((m) => m.state === 'awaiting_confirm'));

  async function loadNames() {
    const ws = $activeWorkspaceId;
    if (!inWails || !ws) return;
    try {
      const list = await listNames(ws);
      const map: Record<string, string> = {};
      for (const n of list) map[n.sessionId] = n.name;
      nameById = map;
    } catch {
      // abaikan: label jatuh ke id sesi
    }
  }

  $effect(() => {
    if (open) {
      $pendingMessages;
      $activeWorkspaceId;
      void loadNames();
    }
  });

  function toggle() {
    if (!open && btn) {
      const rect = btn.getBoundingClientRect();
      const w = Math.min(420, window.innerWidth - 16);
      pos = { left: Math.max(8, Math.min(rect.left, window.innerWidth - w - 8)), bottom: window.innerHeight - rect.top + 4 };
    }
    open = !open;
  }

  function onKey(e: KeyboardEvent) {
    if (open && e.key === 'Escape') open = false;
  }

  function onPointerDown(e: PointerEvent) {
    if (!open) return;
    const t = e.target as Node;
    if (popoverEl?.contains(t) || btn?.contains(t)) return;
    open = false;
  }

  function targetLabel(id: string): string {
    return '@' + (nameById[id] ?? id.slice(0, 8));
  }

  function preview(t: string): string {
    const s = t.replace(/\s*\n\s*/g, ' ');
    return s.length > 100 ? s.slice(0, 100) + '…' : s;
  }

  async function act(fn: (id: string) => Promise<string>, id: string) {
    rowBusy[id] = true;
    rowError[id] = await fn(id);
    rowBusy[id] = false;
  }
</script>

<svelte:window onkeydown={onKey} onpointerdown={onPointerDown} />

<button
  class="badge"
  class:active={$pendingCount > 0}
  bind:this={btn}
  onclick={toggle}
  aria-haspopup="dialog"
  aria-expanded={open}
  title={$queueError || 'Pesan mention tertunda'}
>
  {#if $queueError}<span class="warn" aria-hidden="true">⚠</span>{/if}
  Antrean: {$pendingCount}
  {#if $pendingCount > 0 && hasAwaiting}<span class="dot"></span>{/if}
</button>

{#if open}
  <div class="popover" role="dialog" aria-label="Antrean mention" bind:this={popoverEl} style="left:{pos.left}px; bottom:{pos.bottom}px">
    {#if $pendingMessages.length === 0}
      <div class="empty">Tidak ada pesan tertunda</div>
    {:else}
      {#each $pendingMessages as m (m.id)}
        <div class="item">
          <div class="top">
            <span>dari {m.fromName || m.from} → ke {targetLabel(m.to)}</span>
            <span class="state">{m.state === 'awaiting_confirm' ? 'Menunggu konfirmasi' : 'Antre'}</span>
          </div>
          <div class="txt" title={m.text}>{preview(m.text)}</div>
          <div class="actions">
            {#if m.state === 'awaiting_confirm'}
              <button disabled={rowBusy[m.id]} onclick={() => act(doConfirm, m.id)}>Konfirmasi</button>
            {/if}
            <button disabled={rowBusy[m.id]} onclick={() => act(doCancel, m.id)}>Batalkan</button>
            {#if rowError[m.id]}<span class="rerr">{rowError[m.id]}</span>{/if}
          </div>
        </div>
      {/each}
    {/if}
  </div>
{/if}

<style>
  .badge {
    font-size: 12px; line-height: 1; padding: 0 6px; border: 0; background: transparent; cursor: pointer;
    color: var(--fg-muted, #9b9ba6); display: inline-flex; align-items: center; gap: 4px; height: 100%;
  }
  .badge.active { color: var(--accent, #5b94ff); }
  .dot { width: 6px; height: 6px; border-radius: 50%; background: #e0a030; display: inline-block; }
  .warn { color: var(--danger, #ff6b81); }
  .popover {
    position: fixed; z-index: 1000; width: min(420px, calc(100vw - 16px)); max-height: 50vh; overflow: auto;
    background: var(--bg-elev, #24242a); color: var(--fg, #e6e6ea); border: 1px solid var(--border, #35353d);
    border-radius: 6px; font-size: 12px; line-height: 1.3; box-sizing: border-box;
  }
  .empty { padding: 10px; color: var(--fg-muted, #9b9ba6); }
  .item { padding: 8px; border-bottom: 1px solid var(--border, #35353d); }
  .item:last-child { border-bottom: 0; }
  .top { display: flex; justify-content: space-between; gap: 8px; color: var(--fg-muted, #9b9ba6); }
  .state { color: #e0a030; white-space: nowrap; }
  .txt { margin: 4px 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .actions { display: flex; gap: 6px; align-items: center; }
  .actions button {
    font-size: 11px; padding: 2px 8px; border: 1px solid var(--border, #35353d); border-radius: 4px;
    background: var(--bg-hover, #2f2f37); color: var(--fg, #e6e6ea); cursor: pointer;
  }
  .actions button:disabled { opacity: 0.6; cursor: default; }
  .rerr { color: var(--danger, #ff6b81); font-size: 11px; }
</style>
