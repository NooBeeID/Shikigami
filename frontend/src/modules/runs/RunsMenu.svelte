<script lang="ts">
  import { runs, runsCount, refreshDetail } from './store';
  import { focusOrOpenSession, sessionTitle } from '../shell';
  import { sessions } from '../../lib/stores';
  import type { Run } from './types';
  import { formatElapsed, runTitle, progressText, subagentProgress, agentsCount, usageText, usageTitle, shortModel, workflowModels } from './format';

  let open = $state(false);
  let now = $state(Date.now());
  let wrapEl: HTMLElement | undefined = $state();
  // id workflow yang agent-nya sedang dibuka (tertutup secara bawaan)
  let expanded = $state<Record<string, boolean>>({});

  function hasSession(id: string): boolean {
    return $sessions.some((s) => s.id === id);
  }

  function toggle() {
    open = !open;
  }

  function pick(r: Run) {
    try {
      focusOrOpenSession(r.sessionId);
    } catch {
      // abaikan; menu tetap ditutup
    }
    open = false;
  }

  function toggleAgents(id: string) {
    expanded[id] = !expanded[id];
  }

  // Tutup saat klik luar dan Escape; hanya terpasang saat terbuka.
  $effect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (wrapEl && !wrapEl.contains(e.target as Node)) open = false;
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

  // Polling detail + penyegaran durasi: hanya selama terbuka, dibersihkan saat tutup/destroy.
  $effect(() => {
    if (!open) return;
    let stopped = false;
    let pending = false;
    const tick = () => {
      now = Date.now();
      if (pending) return; // jangan menumpuk panggilan
      pending = true;
      refreshDetail()
        .catch(() => {})
        .finally(() => {
          pending = false;
          if (!stopped) now = Date.now();
        });
    };
    tick();
    const timer = setInterval(tick, 3000);
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  });
</script>

<div class="wrap" bind:this={wrapEl}>
  <button class="btn" aria-label="Aktivitas" aria-haspopup="dialog" aria-expanded={open} onclick={toggle}>
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
    </svg>
    {#if $runsCount > 0}
      <span class="badge">{$runsCount > 99 ? '99+' : $runsCount}</span>
    {/if}
  </button>

  {#if open}
    <div class="dropdown" role="dialog" aria-label="Daftar aktivitas">
      <div class="head"><span class="title">Aktivitas ({$runsCount})</span></div>
      {#if $runs.length === 0}
        <div class="empty">Tidak ada workflow atau sub agent berjalan</div>
      {:else}
        <ul>
          {#each $runs as r (r.id)}
            {@const active = hasSession(r.sessionId)}
            {@const agents = r.kind === 'workflow' ? (r.progress?.agents ?? []) : []}
            <li>
              <button class="row" class:gone={!active} disabled={!active} aria-disabled={!active} onclick={() => pick(r)}>
                <span class="body">
                  <span class="name">{runTitle(r)}</span>
                  <span class="meta">
                    {sessionTitle(r.sessionId)} · {formatElapsed(r.startedAt, now)}
                    {#if r.kind === 'subagent' && subagentProgress(r.progress)} · {subagentProgress(r.progress)}{/if}
                  </span>
                  {#if r.kind === 'workflow' && progressText(r.progress)}
                    <span class="meta prog">{progressText(r.progress)}</span>
                  {/if}
                  {#if r.kind === 'subagent' ? shortModel(r.progress?.model) || usageText(r.progress?.usage) : workflowModels(r.progress) || usageText(r.progress?.usage)}
                    <span class="meta stat" title={usageTitle(r.progress?.usage)}>
                      {#if r.kind === 'subagent'}
                        {#if shortModel(r.progress?.model)}<span class="model">{shortModel(r.progress?.model)}</span>{/if}
                      {:else if workflowModels(r.progress)}
                        <span class="model">{workflowModels(r.progress)}</span>
                      {/if}
                      {#if usageText(r.progress?.usage)}<span>{usageText(r.progress?.usage)}</span>{/if}
                    </span>
                  {/if}
                  {#if !r.confirmed}<span class="tag">belum terkonfirmasi</span>{/if}
                </span>
              </button>
              {#if agents.length > 0}
                <button class="sub-toggle" aria-expanded={!!expanded[r.id]} onclick={() => toggleAgents(r.id)}>
                  <span aria-hidden="true">{expanded[r.id] ? '▾' : '▸'}</span> agent {agentsCount(r.progress)}
                </button>
                {#if expanded[r.id]}
                  <ul class="agents">
                    {#each agents as a}
                      <li class="agent" class:done={a.done}><span class="mark" aria-hidden="true">{a.done ? '☑' : '☐'}</span> {a.label}
                        {#if a.model || a.usage}
                          <span class="a-stat" title={usageTitle(a.usage)}>{[shortModel(a.model), usageText(a.usage)].filter(Boolean).join(' · ')}</span>
                        {/if}
                      </li>
                    {/each}
                  </ul>
                {/if}
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</div>

<style>
  .wrap { position: relative; display: inline-flex; --wails-draggable: no-drag; }
  .btn { position: relative; display: inline-flex; align-items: center; justify-content: center; padding: 4px 6px; background: transparent; border-color: transparent; --wails-draggable: no-drag; }
  .badge {
    position: absolute; top: -4px; right: -4px; min-width: 16px; height: 16px; padding: 0 4px;
    border-radius: 8px; background: var(--accent); color: #fff; font-size: 12px; line-height: 16px; text-align: center;
  }
  .dropdown {
    position: absolute; top: calc(100% + 4px); right: 0; width: 340px; max-height: 60vh; overflow-y: auto; z-index: 1000;
    background: var(--bg-elev); border: 1px solid var(--border); border-radius: 6px; box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
    --wails-draggable: no-drag;
  }
  .head { padding: 8px 10px; border-bottom: 1px solid var(--border); }
  .title { font-weight: 600; color: var(--fg); }
  .empty { padding: 16px; text-align: center; color: var(--fg-muted); }
  ul { list-style: none; margin: 0; padding: 0; }
  .row {
    display: flex; align-items: flex-start; width: 100%; text-align: left; border: none; border-radius: 0;
    background: transparent; padding: 8px 10px;
  }
  .row:hover:not(:disabled) { background: var(--bg-hover); }
  .row.gone { opacity: 0.35; cursor: default; }
  .body { display: flex; flex-direction: column; min-width: 0; }
  .name { color: var(--fg); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { color: var(--fg-muted); font-size: 13px; }
  .prog { color: var(--fg); }
  .stat { display: flex; gap: 6px; flex-wrap: wrap; }
  .model { color: var(--accent); }
  .a-stat { color: var(--fg-muted); margin-left: 4px; }
  .tag { align-self: flex-start; margin-top: 2px; font-size: 12px; color: var(--fg-muted); border: 1px solid var(--border); border-radius: 3px; padding: 0 4px; }
  .sub-toggle {
    display: block; margin: 0 10px 4px 20px; padding: 0 4px; background: transparent; border-color: transparent;
    font-size: 13px; color: var(--fg-muted);
  }
  .agents { margin: 0 10px 6px 28px; }
  .agent { font-size: 13px; color: var(--fg); padding: 1px 0; }
  .agent.done { color: var(--fg-muted); }
  .mark { color: var(--accent); }
</style>
