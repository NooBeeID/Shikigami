<script lang="ts">
  import Modal from '../Modal.svelte';
  import { providers, loadProviders } from '../../../lib/stores/providers';
  import { upsertAgent, type Agent } from './client';
  import { loadAgents } from './store';

  let {
    open,
    onclose,
    categoryId,
    agent,
  }: { open: boolean; onclose: () => void; categoryId: string; agent: Agent | null } = $props();

  let name = $state('');
  let providerId = $state('');
  let model = $state('');
  let isDefault = $state(false);
  let saving = $state(false);
  let error = $state<string | null>(null);
  let submitted = $state(false);

  const filtered = $derived(
    $providers.filter(
      (p) =>
        p.available !== false &&
        p.id !== 'shell' &&
        (categoryId !== 'orchestrator' || p.capabilities?.mcp === true),
    ),
  );
  const staleProvider = $derived(
    !!agent && agent.providerId !== '' && !filtered.some((p) => p.id === agent.providerId),
  );
  const staleLabel = $derived.by(() => {
    if (!agent) return '';
    const p = $providers.find((x) => x.id === agent.providerId);
    return `${p?.name ?? agent.providerId} (tidak valid untuk kategori ini)`;
  });
  const providerValid = $derived(filtered.some((p) => p.id === providerId));
  const nameInvalid = $derived(submitted && name.trim() === '');
  const canSave = $derived(!saving && filtered.length > 0 && providerValid);

  $effect(() => {
    if (!open) return;
    name = agent?.name ?? '';
    providerId = agent?.providerId ?? '';
    model = agent?.model ?? '';
    isDefault = agent?.isDefault ?? false;
    error = null;
    submitted = false;
    saving = false;
    if ($providers.length === 0) {
      loadProviders().catch((e) => {
        error = e instanceof Error ? e.message : String(e);
      });
    }
  });

  // Agent baru: pilih provider valid pertama setelah daftar tersedia.
  $effect(() => {
    if (!open || agent) return;
    if (providerId === '' && filtered.length > 0) providerId = filtered[0].id;
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (saving) return;
    submitted = true;
    if (name.trim() === '' || !canSave) return;
    saving = true;
    error = null;
    try {
      await upsertAgent({
        id: agent?.id,
        name: name.trim(),
        categoryId,
        providerId,
        model: model.trim(),
        isDefault,
      });
      await loadAgents();
      onclose();
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      saving = false;
    }
  }
</script>

<Modal {open} {onclose} title={agent ? 'Ubah agent' : 'Agent baru'}>
  <form class="wrap" onsubmit={submit}>
    <label for="agent-form-name">Nama</label>
    <input
      id="agent-form-name"
      type="text"
      autocomplete="off"
      bind:value={name}
      aria-invalid={nameInvalid}
      aria-describedby={nameInvalid ? 'agent-form-name-err agent-form-name-hint' : 'agent-form-name-hint'}
    />
    <div class="muted" id="agent-form-name-hint">
      Huruf kecil, angka, dan tanda hubung; spasi menjadi tanda hubung.
    </div>
    {#if nameInvalid}<div class="error" id="agent-form-name-err" role="alert">Nama wajib diisi.</div>{/if}

    <label for="agent-form-provider">Provider</label>
    {#if filtered.length === 0}
      <div class="muted">Tidak ada provider yang tersedia untuk kategori ini.</div>
    {:else}
      <select id="agent-form-provider" bind:value={providerId}>
        {#if staleProvider && agent}
          <option value={agent.providerId}>{staleLabel}</option>
        {/if}
        {#each filtered as p (p.id)}
          <option value={p.id}>{p.name || p.id}</option>
        {/each}
      </select>
      {#if staleProvider && !providerValid}
        <div class="error" role="alert">
          Provider saat ini tidak valid atau tidak tersedia. Pilih provider lain sebelum menyimpan.
        </div>
      {/if}
    {/if}

    <label for="agent-form-model">Model</label>
    <input
      id="agent-form-model"
      type="text"
      autocomplete="off"
      placeholder="default provider"
      bind:value={model}
    />

    <label class="check" for="agent-form-default">
      <input id="agent-form-default" type="checkbox" bind:checked={isDefault} />
      Jadikan default
    </label>

    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Batal</button>
      <button type="submit" disabled={!canSave}>{saving ? 'Menyimpan...' : 'Simpan'}</button>
    </div>
  </form>
</Modal>

<style>
  .wrap { display: flex; flex-direction: column; gap: 8px; padding: 16px; overflow: auto; max-height: 100%; box-sizing: border-box; }
  label { font-size: 14px; color: var(--fg-muted, #888); }
  .check { display: flex; align-items: center; gap: 8px; color: var(--fg, #ddd); font-size: 15px; }
  input[type='text'], select { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 6px 8px; font-size: 15px; }
  .muted { color: var(--fg-muted, #888); font-size: 14px; }
  .error { color: #e49a9a; font-size: 15px; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px; }
  button { background: var(--bg-elev, #222); color: var(--fg, #ddd); border: 1px solid var(--border, #333); border-radius: 4px; padding: 4px 12px; font-size: 15px; cursor: pointer; }
  button:hover:not(:disabled) { background: var(--bg-hover, #2c2c2c); }
  button:disabled { opacity: 0.6; cursor: default; }
  button:focus-visible, input:focus-visible, select:focus-visible { outline: 2px solid var(--fg-muted, #888); outline-offset: 2px; }
</style>
