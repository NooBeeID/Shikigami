<script lang="ts">
  import { onMount } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import '@xterm/xterm/css/xterm.css';
  import {
    attachSession,
    base64ToBytes,
    resizeSession,
    subscribeExit,
    subscribeOutput,
    writeSession,
    type OutputEvent,
  } from './terminal-client';
  import { currentTerminalFontSize, registerTerminalInstance } from './font-scale';

  let { sessionId }: { sessionId: string } = $props();

  let host: HTMLDivElement;
  let attachError = $state('');

  onMount(() => {
    const id = sessionId;
    const cs = getComputedStyle(host);
    const val = (n: string, fb: string) => cs.getPropertyValue(n).trim() || fb;
    const bg = val('--term-bg', '#1e1e1e');
    host.style.background = bg;

    const term = new Terminal({
      theme: { background: bg, foreground: val('--term-fg', '#d4d4d4'), cursor: val('--term-cursor', '#d4d4d4') },
      cursorBlink: true,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      fontSize: currentTerminalFontSize(),
      scrollback: 2000,
      allowProposedApi: false,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    const safeFit = () => {
      try { fit.fit(); } catch { /* host tersembunyi */ }
    };
    safeFit();
    const unregisterFont = registerTerminalInstance(term, () => {
      safeFit();
      sendResize();
    });

    let ended = false;
    let attachOffset: number | null = null;
    const pending: OutputEvent[] = [];
    let disposed = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    function markEnded() {
      if (ended) return;
      ended = true;
      term.write('\r\n[sesi berakhir]\r\n');
      term.options.disableStdin = true;
    }

    // Subscribe sebelum attach.
    const unsubOut = subscribeOutput(id, (ev) => {
      if (attachOffset === null) pending.push(ev);
      else if (ev.offset > attachOffset) term.write(base64ToBytes(ev.data));
    });
    const unsubExit = subscribeExit(id, () => markEnded());

    const sendResize = () => { resizeSession(id, term.cols, term.rows).catch(() => {}); };

    (async () => {
      try {
        const res = await attachSession(id);
        if (disposed) return;
        term.write(res.scrollback);
        attachOffset = res.offset;
        for (const ev of pending) {
          if (ev.offset > res.offset) term.write(base64ToBytes(ev.data));
        }
        pending.length = 0;
        if (res.session.status === 'exited') markEnded();
        sendResize();
      } catch (e) {
        if (disposed) return;
        const msg = 'Gagal terhubung ke sesi: ' + (e instanceof Error ? e.message : String(e));
        attachError = msg;
        term.write('\r\n' + msg + '\r\n');
      }
    })();

    term.onData((d) => {
      if (ended) return;
      writeSession(id, d).catch(() => {});
    });

    const ro = new ResizeObserver(() => {
      safeFit();
      clearTimeout(timer);
      timer = setTimeout(sendResize, 100);
    });
    ro.observe(host);

    return () => {
      disposed = true;
      unsubOut();
      unsubExit();
      unregisterFont();
      ro.disconnect();
      clearTimeout(timer);
      term.dispose();
    };
  });
</script>

<div class="term-host" bind:this={host}></div>
{#if attachError}<div class="term-error">{attachError}</div>{/if}

<style>
  .term-host { width: 100%; height: 100%; min-height: 0; }
  .term-host :global(.xterm) { padding: 4px; height: 100%; }
  .term-error { font-size: 0.8rem; color: #c33; padding: 0.25rem 0.5rem; }
</style>
