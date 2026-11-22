<script lang="ts">
  import type { Leaf } from '../layout/model';
  let { pane, visible }: { pane: Leaf; visible: boolean } = $props();
</script>

<div class="git-pane" hidden={!visible}>
  <div class="head">{pane.title || 'Merge Request'}</div>
  <p class="muted">Detail MR segera hadir ({pane.provider ?? '-'} / {pane.projectId ?? '-'} / #{pane.number ?? '-'})</p>
</div>

<style>
  .git-pane { display: flex; flex-direction: column; height: 100%; color: var(--fg, #ddd); }
  .git-pane[hidden] { display: none; }
  .head { padding: 8px 12px; border-bottom: 1px solid var(--border, #333); background: var(--bg-elev, #222); font-size: 14px; font-weight: 600; }
  .muted { margin: 0; padding: 8px 12px; color: var(--fg-muted, #888); font-size: 14px; }
</style>
