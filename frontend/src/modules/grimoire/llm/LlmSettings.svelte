<script lang="ts">
  import {
    llmState,
    startLlmStore,
    requestEnable,
    confirmEnable,
    cancelEnable,
    disableLlm,
    setMaxModules,
    formatTokens,
    type LlmState,
  } from './llmStore';
  import LlmProgressView from './LlmProgress.svelte';
  import LlmCostView from './LlmCost.svelte';

  let { projectId, exists }: { projectId: string | null; exists: boolean } = $props();

  let s = $state<LlmState>({
    projectId: null,
    loaded: false,
    llm: false,
    maxModules: null,
    building: false,
    progress: null,
    lastBuildCost: null,
    estimate: null,
    busy: false,
    error: null,
  });

  $effect(() => llmState.subscribe((v) => { s = v; }));

  // Berhenti otomatis saat project berganti, null, atau unmount.
  $effect(() => {
    if (!projectId) return;
    const stop = startLlmStore(projectId);
    return stop;
  });

  let draft = $state('');
  let editing = $state(false);

  $effect(() => {
    if (!editing) draft = s.maxModules === null ? '' : String(s.maxModules);
  });

  function onToggle() {
    if (s.busy) return;
    if (s.llm) void disableLlm();
    else void requestEnable();
  }

  function commitMax() {
    editing = false;
    const t = draft.trim();
    if (t === '') {
      if (s.maxModules !== null) void setMaxModules(null);
      return;
    }
    const n = Number(t);
    if (!Number.isInteger(n) || n < 1) {
      void setMaxModules(n);
      return;
    }
    if (n !== s.maxModules) void setMaxModules(n);
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      commitMax();
    } else if (e.key === 'Escape') {
      editing = false;
      draft = s.maxModules === null ? '' : String(s.maxModules);
    }
  }
</script>

{#if projectId}
  <div class="llm-settings">
    <div class="row">
      <span class="label" id="llm-switch-label">Ringkasan LLM</span>
      <button
        type="button"
        class="switch"
        class:on={s.llm}
        role="switch"
        aria-checked={s.llm}
        aria-labelledby="llm-switch-label"
        disabled={s.busy || !s.loaded}
        onclick={onToggle}
      ><span class="knob"></span></button>
    </div>

    {#if !exists && s.llm}
      <div class="muted">Generate dulu untuk memakai ringkasan</div>
    {/if}

    {#if s.estimate}
      {@const est = s.estimate}
      <div class="preview" role="group" aria-label="Pratinjau estimasi">
        <div>
          {#if est.modules === 0}
            Tidak ada modul antrean saat ini
          {:else}
            {est.modules} modul, ~{formatTokens(est.estTokens)} token
          {/if}
          {#if est.overLimit && est.overLimit > 0}({est.overLimit} modul di atas batas dilewati){/if}
        </div>
        <div class="actions">
          <button type="button" class="btn" disabled={s.busy} onclick={() => void confirmEnable()}>Konfirmasi</button>
          <button type="button" class="btn" disabled={s.busy} onclick={cancelEnable}>Batal</button>
        </div>
        <div class="muted">Perkiraan; biaya nyata bergantung pada claude</div>
      </div>
    {/if}

    <div class="row">
      <label class="label" for="llm-max-modules">Batas modul</label>
      <input
        id="llm-max-modules"
        type="number"
        min="1"
        step="1"
        inputmode="numeric"
        placeholder="tanpa batas"
        aria-label="Batas modul per build"
        bind:value={draft}
        onfocus={() => (editing = true)}
        onchange={commitMax}
        onblur={commitMax}
        onkeydown={onKeydown}
      />
    </div>
    <div class="muted">kosong = tanpa batas</div>

    <LlmProgressView building={s.building} progress={s.progress} />
    <LlmCostView cost={s.lastBuildCost} />

    {#if s.error}<div class="err" role="alert" title={s.error}>{s.error}</div>{/if}
  </div>
{/if}

<style>
  .llm-settings {
    padding: 4px 12px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
    flex: none;
  }
  .row {
    display: flex;
    gap: 8px;
    align-items: center;
    justify-content: space-between;
    min-width: 0;
  }
  .label {
    min-width: 0;
  }
  .muted {
    font-size: 11px;
    color: var(--fg-muted, inherit);
  }
  .switch {
    flex: none;
    position: relative;
    width: 28px;
    height: 16px;
    padding: 0;
    border: 1px solid var(--border, currentColor);
    border-radius: 8px;
    background: var(--bg-elev, transparent);
    cursor: pointer;
  }
  .switch:hover:not(:disabled) {
    background: var(--bg-hover, var(--bg-elev, transparent));
  }
  .switch:focus-visible {
    outline: 1px solid var(--accent, currentColor);
    outline-offset: 1px;
  }
  .switch:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .switch.on {
    border-color: var(--accent, currentColor);
  }
  .knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--fg-muted, currentColor);
  }
  .switch.on .knob {
    left: 14px;
    background: var(--accent, currentColor);
  }
  .preview {
    display: flex;
    flex-direction: column;
    gap: 6px;
    border: 1px solid var(--border, currentColor);
    border-radius: 4px;
    background: var(--bg-elev, transparent);
    padding: 8px;
  }
  .actions {
    display: flex;
    gap: 8px;
  }
  .btn {
    height: 24px;
    padding: 0 10px;
    border: 1px solid var(--border, currentColor);
    border-radius: 4px;
    background: var(--bg-elev, transparent);
    color: inherit;
    font: inherit;
    cursor: pointer;
  }
  .btn:hover:not(:disabled) {
    background: var(--bg-hover, var(--bg-elev, transparent));
  }
  .btn:focus-visible {
    outline: 1px solid var(--accent, currentColor);
  }
  .btn:disabled {
    opacity: 0.6;
    cursor: default;
  }
  input {
    width: 72px;
    background: var(--bg-elev, transparent);
    color: inherit;
    border: 1px solid var(--border, currentColor);
    border-radius: 4px;
    font: inherit;
    padding: 2px 6px;
  }
  input:focus-visible {
    outline: 1px solid var(--accent, currentColor);
  }
  .err {
    color: var(--danger, inherit);
    font-size: 11px;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
