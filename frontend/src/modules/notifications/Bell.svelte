<script lang="ts">
  import { notifications, unreadCount, markRead, markAllRead, clear } from './store';
  import { focusOrOpenSession } from '../shell';
  import { sessions } from '../../lib/stores';

  let open = $state(false);
  let now = $state(Date.now());
  let bellEl: HTMLElement | undefined = $state();

  function relTime(at: number, ref: number): string {
    const s = Math.max(0, Math.floor((ref - at) / 1000));
    if (s < 60) return 'baru saja';
    const m = Math.floor(s / 60);
    if (m < 60) return `${m} mnt lalu`;
    const h = Math.floor(m / 60);
    if (h < 24) return `${h} jam lalu`;
    return `${Math.floor(h / 24)} hari lalu`;
  }

  function hasSession(id: string): boolean {
    return $sessions.some((s) => s.id === id);
  }

  function toggle() {
    open = !open;
  }

  function pick(n: { id: string; sessionId: string }) {
    try {
      focusOrOpenSession(n.sessionId);
    } catch {
      // abaikan; tetap tandai dibaca
    }
    markRead(n.id);
    open = false;
  }

  // Perbarui waktu relatif hanya saat dibuka dan saat daftar berubah.
  $effect(() => {
    void $notifications;
    if (open) now = Date.now();
  });

  $effect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (bellEl && !bellEl.contains(e.target as Node)) open = false;
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') open = false;
    };
    window.addEventListener('pointerdown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('pointerdown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  });
</script>

<div class="wrap" bind:this={bellEl}>
  <button class="bell" aria-label="Notifikasi" aria-haspopup="true" aria-expanded={open} onclick={toggle}>
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" />
      <path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" />
    </svg>
    {#if $unreadCount > 0}
      <span class="badge">{$unreadCount > 99 ? '99+' : $unreadCount}</span>
    {/if}
  </button>

  {#if open}
    <div class="dropdown" role="dialog" aria-label="Daftar notifikasi">
      <div class="head">
        <span class="title">Notifikasi</span>
        <button class="txt" disabled={$unreadCount === 0} onclick={markAllRead}>Tandai semua dibaca</button>
        <button class="txt" disabled={$notifications.length === 0} onclick={clear}>Bersihkan</button>
      </div>
      {#if $notifications.length === 0}
        <div class="empty">Belum ada notifikasi</div>
      {:else}
        <ul>
          {#each $notifications as n (n.id)}
            {@const active = hasSession(n.sessionId)}
            <li>
              <button class="row" class:read={n.read} class:gone={!active} disabled={!active} aria-disabled={!active} onclick={() => pick(n)}>
                <span class="dot" class:on={!n.read}></span>
                <span class="body">
                  <span class="name">{n.title}</span>
                  <span class="meta">{n.kind === 'waiting' ? 'menunggu input' : 'selesai'} · {relTime(n.at, now)}</span>
                </span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</div>

<style>
  .wrap { position: relative; display: inline-flex; --wails-draggable: no-drag; }
  .bell { position: relative; display: inline-flex; align-items: center; justify-content: center; padding: 4px 6px; background: transparent; border-color: transparent; --wails-draggable: no-drag; }
  .badge {
    position: absolute; top: -4px; right: -4px; min-width: 16px; height: 16px; padding: 0 4px;
    border-radius: 8px; background: var(--danger, #e5484d); color: #fff; font-size: 12px; line-height: 16px; text-align: center;
  }
  .dropdown {
    position: absolute; top: calc(100% + 4px); right: 0; width: 320px; max-height: 60vh; overflow-y: auto; z-index: 1000;
    background: var(--bg-elev); border: 1px solid var(--border); border-radius: 6px; box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
    --wails-draggable: no-drag;
  }
  .head { display: flex; align-items: center; gap: 6px; padding: 8px 10px; border-bottom: 1px solid var(--border); }
  .title { flex: 1; font-weight: 600; color: var(--fg); }
  .txt { background: transparent; border-color: transparent; padding: 2px 4px; font-size: 13px; color: var(--fg-muted); }
  .empty { padding: 16px; text-align: center; color: var(--fg-muted); }
  ul { list-style: none; margin: 0; padding: 0; }
  .row {
    display: flex; align-items: flex-start; gap: 8px; width: 100%; text-align: left; border: none; border-radius: 0;
    background: transparent; padding: 8px 10px;
  }
  .row:hover:not(:disabled) { background: var(--bg-hover); }
  .row.read { opacity: 0.6; }
  .row.gone { opacity: 0.35; cursor: default; }
  .dot { flex: none; width: 7px; height: 7px; margin-top: 5px; border-radius: 50%; background: transparent; }
  .dot.on { background: var(--accent); }
  .body { display: flex; flex-direction: column; min-width: 0; }
  .name { color: var(--fg); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { color: var(--fg-muted); font-size: 13px; }
</style>
