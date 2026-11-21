<script lang="ts">
  import { sessions, activeWorkspaceId } from '../../lib/stores';
  import { statusStore } from '../status/store';
  import { countByStatus, sessionsOfActive } from './util';

  const mine = $derived(sessionsOfActive($sessions, $activeWorkspaceId));
  const counts = $derived(countByStatus(mine, $statusStore));
  const exited = $derived(mine.filter((s) => s.status === 'exited').length);
  const n = $derived(counts.running);
  const tip = $derived(
    [
      `running ${counts.running}`,
      `waiting ${counts.waiting}`,
      `done ${counts.done}`,
      `idle ${counts.idle}`,
      ...(exited > 0 ? [`berakhir ${exited}`] : []),
    ].join('\n'),
  );
</script>

<div class="counter" title={tip}>
  <span class="dot" class:on={n > 0}></span>
  <span>{n} sesi</span>
</div>

<style>
  .counter {
    --wails-draggable: no-drag;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 0 8px;
    font-size: 12px;
    color: var(--fg-muted, #888);
    white-space: nowrap;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--fg-muted, #888);
    opacity: 0.4;
  }
  .dot.on {
    background: var(--ok, #3fb950);
    opacity: 1;
  }
</style>
