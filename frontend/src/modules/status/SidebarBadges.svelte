<script lang="ts">
  // Registry tidak menyediakan slot lencana di baris sesi milik shell dan shell
  // tidak boleh diedit, jadi panel ini menampilkan daftar ringkas sesi + lencana.
  import { sessions, providers } from '../../lib/stores';
  import StatusBadge from './StatusBadge.svelte';

  const active = $derived($sessions.filter((s) => s.status === 'running'));

  function providerName(id: string): string {
    return $providers.find((p) => p.id === id)?.name ?? id;
  }
  function base(cwd: string): string {
    const parts = cwd.split('/').filter(Boolean);
    return parts[parts.length - 1] ?? cwd;
  }
</script>

<div class="wrap">
  <div class="title">Status sesi</div>
  {#if active.length === 0}
    <div class="muted">Tidak ada sesi aktif</div>
  {:else}
    <ul>
      {#each active as s (s.id)}
        <li>
          <StatusBadge sessionId={s.id} />
          <span class="name">{providerName(s.providerId)}</span>
          <span class="cwd">{base(s.cwd)}</span>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .wrap { padding: 6px 8px; border-top: 1px solid var(--border, #8884); color: var(--fg, inherit); }
  .title { font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--fg-muted, #888); margin-bottom: 4px; }
  .muted { font-size: 12px; color: var(--fg-muted, #888); }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  li { display: flex; align-items: center; gap: 6px; min-width: 0; padding: 2px 4px; border-radius: 4px; font-size: 12px; }
  li:hover { background: var(--bg-hover, #0001); }
  .name { flex: none; }
  .cwd { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--fg-muted, #888); }
</style>
