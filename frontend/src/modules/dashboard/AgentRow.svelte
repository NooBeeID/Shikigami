<script lang="ts">
  import { focusOrOpenSession } from '../shell';
  import type { AgentRow as AgentRowData } from './types';
  import { STATUS_LABEL } from './types';

  let { row, now }: { row: AgentRowData; now: number } = $props();

  function formatDuration(since: number, ref: number): string {
    const s = Math.max(0, Math.floor((ref - since) / 1000));
    if (s < 5) return 'baru saja';
    if (s < 60) return `${s} dtk`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m} mnt`;
    return `${Math.floor(m / 60)} jam`;
  }

  const duration = $derived(formatDuration(row.since, now));
  const statusLabel = $derived(STATUS_LABEL[row.status]);
  const waitText = $derived(row.status === 'waiting' ? (row.reason ? `menunggu: ${row.reason}` : 'menunggu') : '');
  const ariaLabel = $derived(
    `${row.title}, ${statusLabel}, ${duration}${row.projectName ? `, ${row.projectName}` : ''}`,
  );

  function open() {
    try {
      focusOrOpenSession(row.sessionId);
    } catch {
      /* abaikan */
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp' && e.key !== 'Home' && e.key !== 'End') return;
    const cur = e.currentTarget as HTMLButtonElement;
    const root = cur.closest('.rows-root');
    if (!root) return;
    e.preventDefault();
    const btns = Array.from(root.querySelectorAll<HTMLButtonElement>('button.agent-row'));
    const i = btns.indexOf(cur);
    let next = i;
    if (e.key === 'ArrowDown') next = Math.min(btns.length - 1, i + 1);
    else if (e.key === 'ArrowUp') next = Math.max(0, i - 1);
    else if (e.key === 'Home') next = 0;
    else next = btns.length - 1;
    btns[next]?.focus();
  }
</script>

<div role="listitem" class="item" class:waiting={row.status === 'waiting'}>
  <button type="button" class="agent-row" aria-label={ariaLabel} onclick={open} onkeydown={onKey}>
    <span class="status"><span class="dot dot-{row.status}" aria-hidden="true"></span><span class="status-label">{statusLabel}</span></span>
    <span class="main">
      <span class="name">{row.title}</span>
      <span class="project">{row.projectName}</span>
      <span class="duration">{duration}</span>
    </span>
    {#if waitText}<span class="wait">{waitText}</span>{/if}
    {#if row.lastActivity}<span class="activity" title={row.lastActivity}>{row.lastActivity}</span>{/if}
  </button>
</div>

<style>
  .item {
    border-left: 3px solid transparent;
  }
  .item.waiting {
    border-left-color: var(--warn, #f5a524);
    background: color-mix(in srgb, var(--warn, #f5a524) 10%, transparent);
  }
  .agent-row {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    min-width: 0;
    text-align: left;
    padding: 6px 10px;
    background: none;
    border: 0;
    color: var(--fg, inherit);
    cursor: pointer;
    font: inherit;
  }
  .agent-row:hover,
  .agent-row:focus-visible {
    background: var(--bg-hover, color-mix(in srgb, currentColor 10%, transparent));
  }
  .agent-row:focus-visible {
    outline: 1px solid var(--accent, currentColor);
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 12px;
    color: var(--fg-muted, inherit);
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: none;
  }
  .dot-waiting { background: var(--warn, #f5a524); }
  .dot-running { background: var(--ok, #30a46c); }
  .dot-idle { background: var(--fg-muted, #8b8d98); }
  .dot-done { background: var(--accent, #3e63dd); }
  .main {
    display: flex;
    align-items: baseline;
    gap: 6px;
    min-width: 0;
  }
  .name {
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .project {
    color: var(--fg-muted, inherit);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .duration {
    color: var(--fg-muted, inherit);
    margin-left: auto;
    flex: none;
  }
  .wait {
    color: var(--warn, #f5a524);
    font-weight: 600;
  }
  .activity {
    display: block;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--fg-muted, inherit);
    font-size: 12px;
    min-width: 0;
  }
</style>
