<script lang="ts">
  import { get } from 'svelte/store';
  import { Modal } from '../settings';
  import { activeWorkspaceId } from '../../lib/stores/workspaces';
  import { larkLinkParse, larkSourceAdd, parseLarkError, type ParsedLink } from './client';
  import { refreshSources } from './store';

  let { kind, open, onclose }: { kind: 'ticket' | 'wiki'; open: boolean; onclose: () => void } = $props();

  const uid = Math.random().toString(36).slice(2, 9);
  const linkId = `lark-link-${uid}`;
  const nameId = `lark-name-${uid}`;

  let link = $state('');
  let parsed = $state<ParsedLink | null>(null);
  let name = $state('');
  let parsing = $state(false);
  let saving = $state(false);
  let error = $state<string | null>(null);
  let parseSeq = 0;
  let lastParsed = '';
  let wasOpen = false;
  let linkEl: HTMLInputElement | undefined = $state();

  $effect(() => {
    if (open && !wasOpen) {
      link = '';
      parsed = null;
      name = '';
      parsing = false;
      saving = false;
      error = null;
      lastParsed = '';
      parseSeq++;
      queueMicrotask(() => linkEl?.focus());
    }
    wasOpen = open;
  });

  function hint(code: string): string {
    return code === 'cli_missing' || code === 'not_logged_in' ? ' Buka Pengaturan > Connector.' : '';
  }

  async function parse() {
    const v = link.trim();
    if (!v || v === lastParsed) return;
    lastParsed = v;
    const my = ++parseSeq;
    parsing = true;
    error = null;
    parsed = null;
    try {
      const r = await larkLinkParse(v);
      if (my !== parseSeq) return;
      if (kind === 'ticket' && r.kind !== 'base') {
        error = 'Link ini bukan Base. Gunakan Tambah wiki untuk halaman wiki.';
      } else if (kind === 'wiki' && r.kind !== 'wiki') {
        error = 'Link ini adalah Base. Gunakan Tambah Base.';
      } else {
        parsed = r;
        name = r.defaultName ?? '';
      }
    } catch (x) {
      if (my !== parseSeq) return;
      const e = parseLarkError(x);
      error = e.message + hint(e.code);
      parsed = null;
    } finally {
      if (my === parseSeq) parsing = false;
    }
  }

  function onLinkInput() {
    if (link.trim() !== lastParsed) {
      parsed = null;
      parseSeq++;
      parsing = false;
    }
  }

  async function save() {
    if (!parsed || saving || !name.trim()) return;
    const ws = get(activeWorkspaceId);
    if (!ws) {
      error = 'Tidak ada workspace aktif';
      return;
    }
    saving = true;
    error = null;
    try {
      if (kind === 'ticket') {
        await larkSourceAdd(ws, {
          kind: 'ticket',
          name: name.trim(),
          baseToken: parsed.baseToken!,
          host: parsed.host,
          tableId: parsed.tableId,
          viewId: parsed.viewId,
        });
      } else {
        await larkSourceAdd(ws, {
          kind: 'wiki',
          name: name.trim(),
          spaceId: parsed.spaceId!,
          rootToken: parsed.nodeToken,
          host: parsed.host,
        });
      }
      await refreshSources();
      onclose();
    } catch (x) {
      const e = parseLarkError(x);
      error = e.code === 'duplicate' ? 'Sumber sudah ada' : e.message + hint(e.code);
    } finally {
      saving = false;
    }
  }

  let linkLabel = $derived(kind === 'ticket' ? 'Tempel link Base' : 'Tempel link wiki');
</script>

<Modal {open} {onclose} title={kind === 'ticket' ? 'Tambah Base' : 'Tambah wiki'}>
  <div class="body">
    <label for={linkId}>{linkLabel}</label>
    <input
      id={linkId}
      type="text"
      bind:this={linkEl}
      bind:value={link}
      placeholder="https://xxx.larksuite.com/base/..."
      oninput={onLinkInput}
      onpaste={() => setTimeout(() => void parse(), 0)}
      onblur={() => void parse()}
      onkeydown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault();
          void parse();
        }
      }}
    />
    {#if parsing}<p class="muted">Memeriksa link...</p>{/if}
    {#if parsed}
      <label for={nameId}>Nama</label>
      <input id={nameId} type="text" bind:value={name} />
      {#if kind === 'wiki'}
        <p class="muted">{parsed.nodeToken ? `Akar: ${parsed.defaultName ?? parsed.nodeToken}` : 'Akar: seluruh space'}</p>
      {/if}
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="actions">
      <button type="button" onclick={save} disabled={!parsed || saving || !name.trim()}>{saving ? 'Menyimpan...' : 'Simpan'}</button>
      <button type="button" onclick={onclose}>Batal</button>
    </div>
  </div>
</Modal>

<style>
  .body {
    display: flex;
    flex-direction: column;
    gap: 8px;
    color: var(--fg, #ddd);
    font-size: 15px;
    min-width: 0;
  }
  p {
    margin: 0;
  }
  label {
    color: var(--fg-muted, #888);
    font-size: 14px;
  }
  input {
    background: var(--bg-elev, #222);
    color: var(--fg, #ddd);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    padding: 5px 8px;
    font-size: 15px;
  }
  .muted {
    color: var(--fg-muted, #888);
  }
  .error {
    color: #e49a9a;
  }
  .actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }
  button {
    background: var(--bg-elev, #222);
    color: var(--fg, #ddd);
    border: 1px solid var(--border, #333);
    border-radius: 4px;
    padding: 4px 12px;
    font-size: 15px;
    cursor: pointer;
  }
  button:hover:not(:disabled) {
    background: var(--bg-hover, #2c2c2c);
  }
  button:disabled {
    opacity: 0.5;
    cursor: default;
  }
</style>
