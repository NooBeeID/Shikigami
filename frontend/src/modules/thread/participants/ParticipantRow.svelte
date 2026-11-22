<script lang="ts">
  import { statusOf } from '../../status/store';
  import { avatarColor, initials } from '../format';
  import type { Participant, Thread } from '../types';
  import { openParticipantTerminal } from './terminal';

  let { participant, thread, projectName }: { participant: Participant; thread: Thread; projectName: string } = $props();

  let status = $state<string>('idle');
  $effect(() => statusOf(participant.sessionId).subscribe((v) => (status = v)));
  const working = $derived(status === 'running');

  const label = $derived(
    participant.name || (participant.role === 'master' ? 'master' : participant.sessionId.slice(0, 8)),
  );
  const sub = $derived([participant.role, projectName].filter(Boolean).join(' · '));
</script>

<button type="button" class="row" title="Buka terminal" onclick={() => openParticipantTerminal(participant, thread)}>
  <span class="avatar" style:background={avatarColor(participant.name || participant.sessionId)} aria-hidden="true">{initials(label)}</span>
  <span class="text">
    <span class="top"><span class="name">{label}</span><span class="badge">thread</span></span>
    <span class="sub">{sub}</span>
  </span>
  <span class="state" aria-label={working ? 'kerja' : 'idle'}>
    <span class="dot" class:working></span>{working ? 'kerja' : 'idle'}
  </span>
</button>

<style>
  .row { display: flex; align-items: center; gap: 8px; width: 100%; padding: 6px 8px; border: 0; border-radius: 6px; background: transparent; color: var(--fg, #222); text-align: left; cursor: pointer; font: inherit; }
  .row:hover { background: var(--bg-elev, #f2f2f2); }
  .avatar { flex: none; width: 28px; height: 28px; border-radius: 50%; color: #fff; font-size: 13px; font-weight: 600; display: flex; align-items: center; justify-content: center; }
  .text { min-width: 0; flex: 1; display: flex; flex-direction: column; gap: 2px; }
  .top { display: flex; align-items: center; gap: 6px; min-width: 0; }
  .name { font-size: 15px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .badge { flex: none; font-size: 12px; line-height: 1.4; padding: 0 5px; border: 1px solid var(--fg-muted, #888); border-radius: 8px; color: var(--fg-muted, #888); }
  .sub { font-size: 13px; color: var(--fg-muted, #888); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .state { flex: none; display: flex; align-items: center; gap: 4px; font-size: 13px; color: var(--fg-muted, #888); }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: #9a9a9a; }
  .dot.working { background: var(--accent, #3b82f6); }
</style>
