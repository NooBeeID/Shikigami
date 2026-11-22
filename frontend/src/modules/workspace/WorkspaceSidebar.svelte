<script lang="ts">
  import { onDestroy } from 'svelte';
  import WorkspacePicker from './WorkspacePicker.svelte';
  import ProjectList from './ProjectList.svelte';
  import SessionList from './SessionList.svelte';
  import NewSession from './NewSession.svelte';

  let error = $state('');
  let timer: ReturnType<typeof setTimeout> | undefined;

  function showError(msg: string) {
    error = msg;
    clearTimeout(timer);
    timer = setTimeout(() => (error = ''), 6000);
  }
  function dismiss() { clearTimeout(timer); error = ''; }

  onDestroy(() => clearTimeout(timer));
</script>

<div class="sidebar">
  {#if error}
    <div class="error" role="alert">
      <span class="msg">{error}</span>
      <button type="button" aria-label="Tutup pesan" title="Tutup" onclick={dismiss}>×</button>
    </div>
  {/if}
  <section>
    <h3>Workspace</h3>
    <WorkspacePicker onerror={showError} />
  </section>
  <section>
    <h3>Project</h3>
    <ProjectList onerror={showError} />
  </section>
  <section>
    <NewSession onerror={showError} />
  </section>
  <section>
    <h3>Sesi</h3>
    <SessionList onerror={showError} />
  </section>
</div>

<style>
  .sidebar { display: flex; flex-direction: column; height: 100%; overflow: auto; font-size: 15px; gap: 12px; padding: 8px; box-sizing: border-box; color: var(--fg, #222); }
  section { min-width: 0; }
  h3 { margin: 0 0 4px; font-size: 13px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--fg-muted, #888); font-weight: 600; }
  .error { display: flex; align-items: center; gap: 6px; color: var(--danger, #d33); border: 1px solid var(--danger, #d33); border-radius: 4px; padding: 3px 6px; min-width: 0; }
  .msg { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .error button { background: transparent; border: none; color: inherit; cursor: pointer; font: inherit; }
</style>
