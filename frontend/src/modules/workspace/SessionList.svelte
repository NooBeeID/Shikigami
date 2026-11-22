<script lang="ts">
  import { sessions, providers, activeWorkspace, activeTabId, openTab, killSession } from '../../lib/stores';
  import { errMessage } from './errors';

  let { onerror }: { onerror: (msg: string) => void } = $props();

  function providerName(id: string): string {
    return $providers.find((p) => p.id === id)?.name ?? id;
  }
  function projectName(projectId: string, cwd: string): string {
    const p = $activeWorkspace?.projects.find((x) => x.id === projectId);
    if (p) return p.name;
    const parts = cwd.split('/').filter(Boolean);
    return parts[parts.length - 1] ?? cwd;
  }

  async function kill(id: string) {
    try { await killSession(id); } catch (e) { onerror(errMessage(e)); }
  }
</script>

{#if $sessions.length === 0}
  <div class="muted">Belum ada sesi</div>
{:else}
  <ul>
    {#each $sessions as s (s.id)}
      <li class:active={$activeTabId === 'session:' + s.id} class:exited={s.status === 'exited'}>
        <button
          type="button"
          class="main"
          onclick={() => openTab({ type: 'session', refId: s.id, title: providerName(s.providerId) })}
        >
          {#if s.status === 'running'}
            <span class="dot" aria-label="running"></span>
          {/if}
          <span class="text">
            <span class="name">{providerName(s.providerId)}</span>
            <span class="sub">
              {projectName(s.projectId, s.cwd)}{#if s.status === 'exited'} · exited{#if s.exitCode !== undefined} ({s.exitCode}){/if}{/if}
            </span>
          </span>
        </button>
        {#if s.status === 'running'}
          <button type="button" aria-label="Kill sesi" title="Kill sesi" onclick={() => kill(s.id)}>Kill</button>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
  li { display: flex; align-items: center; gap: 4px; min-width: 0; border-radius: 4px; }
  li.active { background: var(--bg-hover, #0001); }
  li.exited .main { opacity: 0.55; }
  .main { flex: 1; min-width: 0; display: flex; align-items: center; gap: 6px; text-align: left; border: none; padding: 4px 6px; font-size: 15px; }
  .dot { width: 7px; height: 7px; border-radius: 50%; flex: none; background: var(--ok, #22c55e); }
  .text { display: flex; flex-direction: column; min-width: 0; }
  .name, .sub { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub { font-size: 13px; color: var(--fg-muted, #888); }
  button { font: inherit; font-size: 14px; color: var(--fg, #222); background: transparent; border: 1px solid var(--border, #8884); border-radius: 4px; padding: 2px 8px; cursor: pointer; }
  button.main { border: none; }
  button:hover:not(:disabled) { background: var(--bg-hover, #0001); }
  .muted { color: var(--fg-muted, #888); }
</style>
