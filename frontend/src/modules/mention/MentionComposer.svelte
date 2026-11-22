<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { activeWorkspaceId, activeProjectId, sessions, activeTabId } from '../../lib/stores';
  import { inWails } from '../../lib/ipc';
  import { buildSuggestions, errorText, lastOutput, listAgents, listNames, matchAgentRef, mentionQuery, onAgentsChanged, onSessionExit, parseMention, sendMention } from './mention-client';
  import type { AgentInfo, CategoryInfo, MentionSuggestion, NameInfo } from './types';

  let text = $state('');
  let names = $state<NameInfo[]>([]);
  let agents = $state<AgentInfo[]>([]);
  let categories = $state<CategoryInfo[]>([]);
  let namesError = $state('');
  let menuOpen = $state(false);
  let menuIndex = $state(0);
  let query = $state('');
  let mentionStart = $state(-1);
  let includeOutput = $state(false);
  let busy = $state(false);
  let feedback = $state<{ kind: 'ok' | 'err'; msg: string } | null>(null);
  let textareaEl: HTMLTextAreaElement | undefined = $state();

  const suggestions = $derived(buildSuggestions(names, agents, categories));
  const filtered = $derived<MentionSuggestion[]>(suggestions.filter((s) => s.name.startsWith(query)));
  const showMenu = $derived(menuOpen && filtered.length > 0);

  async function refreshAgents() {
    if (!inWails) {
      agents = [];
      categories = [];
      return;
    }
    try {
      const r = await listAgents();
      agents = r.agents;
      categories = r.categories;
    } catch {
      agents = [];
      categories = [];
    }
  }

  async function refreshNames() {
    if (!inWails) {
      namesError = 'Hanya tersedia di aplikasi desktop';
      names = [];
      return;
    }
    const ws = $activeWorkspaceId;
    if (!ws) {
      names = [];
      namesError = '';
      return;
    }
    try {
      const res = await listNames(ws);
      if ($activeWorkspaceId !== ws) return;
      names = res;
      namesError = '';
    } catch (e) {
      if ($activeWorkspaceId !== ws) return;
      namesError = errorText(e);
      names = [];
    }
  }

  $effect(() => {
    $activeWorkspaceId;
    $sessions;
    void refreshNames();
  });

  onMount(() => {
    const offExit = onSessionExit(() => void refreshNames());
    const offAgents = onAgentsChanged(() => void refreshAgents());
    void refreshAgents();
    return () => {
      offExit();
      offAgents();
    };
  });

  function updateMenu() {
    const caret = textareaEl?.selectionStart ?? text.length;
    const q = mentionQuery(text, caret);
    if (q) {
      if (!menuOpen || q.query !== query) menuIndex = 0;
      menuOpen = true;
      query = q.query;
      mentionStart = q.start;
    } else {
      menuOpen = false;
    }
  }

  function onInput() {
    feedback = null;
    updateMenu();
  }

  async function pick(n: MentionSuggestion) {
    const caret = textareaEl?.selectionStart ?? text.length;
    const ins = '@' + n.name + ' ';
    text = text.slice(0, mentionStart) + ins + text.slice(caret);
    const pos = mentionStart + ins.length;
    menuOpen = false;
    await tick();
    textareaEl?.focus();
    textareaEl?.setSelectionRange(pos, pos);
  }

  function onKeydown(e: KeyboardEvent) {
    if (showMenu) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        menuIndex = (menuIndex + 1) % filtered.length;
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        menuIndex = (menuIndex - 1 + filtered.length) % filtered.length;
        return;
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        void pick(filtered[Math.min(menuIndex, filtered.length - 1)]);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        menuOpen = false;
        return;
      }
    } else if (e.key === 'Enter' && !e.shiftKey && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      void submit();
    }
  }

  function stateLabel(state: string, name: string, error?: string): string {
    switch (state) {
      case 'delivered': return `Terkirim ke @${name}`;
      case 'queued': return `Diantre untuk @${name} (menunggu idle)`;
      case 'awaiting_confirm': return 'Menunggu konfirmasi';
      case 'failed': return `Gagal: ${error ?? 'tidak diketahui'}`;
      default: return state;
    }
  }

  function spawnLabel(state: string, sess: string, error?: string): string {
    switch (state) {
      case 'delivered': return `Sesi baru @${sess} dibuka dan pesan terkirim`;
      case 'queued': return `Sesi baru @${sess} dibuka dan pesan diantrekan`;
      case 'awaiting_confirm': return `Sesi baru @${sess} dibuka; pesan menunggu konfirmasi`;
      default: return stateLabel(state, sess, error);
    }
  }

  function fail(msg: string) {
    feedback = { kind: 'err', msg };
  }

  async function submit() {
    if (busy) return;
    feedback = null;
    if (!inWails) return fail('Hanya tersedia di aplikasi desktop');
    const ws = $activeWorkspaceId;
    if (!ws) return fail('Pilih workspace dulu');
    const parsed = parseMention(text);
    if (!parsed) return fail('Awali pesan dengan @nama-sesi atau @agent');
    const known = names.some((n) => n.name === parsed.name) || matchAgentRef(parsed.name, agents, categories);
    if (!namesError && names.length > 0 && !known) {
      return fail(`Sesi @${parsed.name} tidak ada di workspace ini`);
    }
    if (!namesError && names.length === 0 && !known && ws) {
      // daftar berhasil dimuat tetapi kosong: tidak ada sesi sama sekali
      return fail(`Sesi @${parsed.name} tidak ada di workspace ini`);
    }
    if (!parsed.text && !includeOutput) return fail('Isi pesan tidak boleh kosong');
    let body = parsed.text;
    busy = true;
    try {
      if (includeOutput) {
        const tab = $activeTabId;
        if (!tab || !tab.startsWith('session:')) return fail('Tidak ada sesi aktif untuk diambil outputnya');
        let out = '';
        try {
          out = await lastOutput(tab.slice(8), 30);
        } catch (e) {
          return fail(errorText(e));
        }
        if (!out) return fail('Output sesi aktif kosong');
        body = (parsed.text ? parsed.text + '\n\n' : '') + '--- output terakhir sesi aktif ---\n' + out + '\n--- selesai ---';
      }
      const project = $activeProjectId ?? undefined;
      const msg = await sendMention(ws, parsed.name, body, project);
      const label = msg.spawned && msg.state !== 'failed'
        ? spawnLabel(msg.state, msg.spawnedSession || parsed.name, msg.error)
        : stateLabel(msg.state, parsed.name, msg.error);
      feedback = { kind: msg.state === 'failed' ? 'err' : 'ok', msg: label };
      if (msg.state !== 'failed') text = '';
    } catch (e) {
      fail(errorText(e));
    } finally {
      busy = false;
    }
  }
</script>

<div class="composer">
  <div class="title">Mention sesi</div>
  <div class="field">
    <textarea
      rows="3"
      placeholder="@nama-sesi atau @agent pesan..."
      bind:this={textareaEl}
      bind:value={text}
      oninput={onInput}
      onclick={updateMenu}
      onkeyup={(e) => { if (e.key.startsWith('Arrow') && showMenu) return; if (e.key === 'Enter' || e.key === 'Tab' || e.key === 'Escape') return; updateMenu(); }}
      onkeydown={onKeydown}
    ></textarea>
    {#if showMenu}
      <ul class="menu" role="listbox">
        {#each filtered as n, i (n.kind + ':' + n.name)}
          <li
            role="option"
            aria-selected={i === menuIndex}
            class:active={i === menuIndex}
            onmousedown={(e) => { e.preventDefault(); void pick(n); }}
          >
            <span class="nm">@{n.name}</span>
            {#if n.kind === 'session'}
              <span class="muted">{n.projectName} · {n.providerId}</span>
              <span class="st st-{n.status}">{n.status}</span>
            {:else if n.kind === 'agent'}
              <span class="muted">{n.categoryName} · {n.providerId} · {n.model || 'default'}</span>
              {#if !n.hasSession}<span class="new">agent baru</span>{/if}
            {:else}
              <span class="muted">kategori · default: @{n.defaultAgent}</span>
              {#if !n.hasSession}<span class="new">agent baru</span>{/if}
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </div>
  <label class="opt"><input type="checkbox" bind:checked={includeOutput} /> Sertakan output terakhir sesi aktif</label>
  <div class="row">
    <button class="send" onclick={submit} disabled={busy}>Kirim</button>
    {#if namesError}<span class="muted small">{namesError}</span>{/if}
  </div>
  <div role="status" class="fb">
    {#if feedback}<span class:err={feedback.kind === 'err'} class:ok={feedback.kind === 'ok'}>{feedback.msg}</span>{/if}
  </div>
</div>

<style>
  .composer { padding: 8px; border-top: 1px solid var(--border, #35353d); position: relative; color: var(--fg, #e6e6ea); font-size: 14px; }
  .title { font-weight: 600; margin-bottom: 6px; }
  .field { position: relative; }
  textarea {
    width: 100%; box-sizing: border-box; resize: vertical;
    background: var(--bg, #1b1b1f); color: var(--fg, #e6e6ea);
    border: 1px solid var(--border, #35353d); border-radius: 4px; font: inherit; padding: 6px;
  }
  .menu {
    position: absolute; left: 0; right: 0; top: 100%; z-index: 20; margin: 2px 0 0; padding: 2px; list-style: none;
    background: var(--bg-elev, #24242a); border: 1px solid var(--border, #35353d); border-radius: 4px;
    max-height: 180px; overflow: auto;
  }
  .menu li { display: flex; gap: 6px; align-items: center; padding: 4px 6px; cursor: pointer; border-radius: 3px; }
  .menu li.active { background: var(--bg-hover, #2f2f37); }
  .nm { font-weight: 600; }
  .muted { color: var(--fg-muted, #9b9ba6); }
  .small { font-size: 13px; }
  .st { margin-left: auto; font-size: 12px; }
  .new { margin-left: auto; font-size: 12px; padding: 0 6px; border: 1px solid var(--accent, #5b94ff); border-radius: 8px; color: var(--accent, #5b94ff); }
  .st-running { color: var(--accent, #5b94ff); }
  .st-waiting { color: #e0a030; }
  .st-done, .st-idle { color: var(--ok, #4cc788); }
  .opt { display: flex; gap: 6px; align-items: center; margin: 6px 0; color: var(--fg-muted, #9b9ba6); }
  .row { display: flex; gap: 8px; align-items: center; }
  .send { background: var(--accent, #5b94ff); color: #fff; border: 0; border-radius: 4px; padding: 4px 12px; cursor: pointer; }
  .send:disabled { opacity: 0.6; cursor: default; }
  .fb { min-height: 16px; margin-top: 6px; }
  .err { color: var(--danger, #ff6b81); }
  .ok { color: var(--ok, #4cc788); }
</style>
