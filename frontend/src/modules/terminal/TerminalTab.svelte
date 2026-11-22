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
  import { registerTerminalPaste } from './paste';

  let { sessionId, scrollback = 2000, visible = true, fontSize, onTitle }:
    { sessionId: string; scrollback?: number; visible?: boolean; fontSize?: number; onTitle?: (t: string) => void } = $props();

  const MAX_QUEUE_BYTES = 8 * 1024 * 1024;
  let visibleNow = visible;
  let applyVisible: (v: boolean) => void = () => {};
  let setFontSize: (n: number) => void = () => {};

  $effect(() => { applyVisible(visible); });
  $effect(() => { if (fontSize !== undefined) setFontSize(fontSize); });

  let host: HTMLDivElement;
  let attachError = $state('');

  onMount(() => {
    const id = sessionId;
    visibleNow = visible;
    const term = new Terminal({
      theme: { background: '#1e1e1e', foreground: '#d4d4d4', cursor: '#d4d4d4' },
      cursorBlink: true,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      fontSize: fontSize ?? currentTerminalFontSize(),
      scrollback,
      allowProposedApi: false,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    let opened = false;
    const safeFit = () => {
      if (!opened) return;
      try { fit.fit(); } catch { /* host tersembunyi */ }
    };
    // Buka xterm hanya saat host sudah terpasang di DOM dan terlihat.
    const tryOpen = () => {
      if (opened || !host.isConnected || !visibleNow) return;
      const cs = getComputedStyle(host);
      const val = (n: string, fb: string) => cs.getPropertyValue(n).trim() || fb;
      const bg = val('--term-bg', '#1e1e1e');
      host.style.background = bg;
      term.options.theme = { background: bg, foreground: val('--term-fg', '#d4d4d4'), cursor: val('--term-cursor', '#d4d4d4') };
      term.open(host);
      opened = true;
      safeFit();
    };
    tryOpen();
    const unregisterFont = registerTerminalInstance(term, () => {
      if (!visibleNow || !opened) return;
      safeFit();
      sendResize();
    });

    // Antrian tulis saat tersembunyi.
    let queue: (Uint8Array | string)[] = [];
    let queueBytes = 0;
    const sizeOf = (c: Uint8Array | string) => (typeof c === 'string' ? c.length : c.byteLength);
    function emit(chunk: Uint8Array | string) {
      if (visibleNow) { term.write(chunk); return; }
      queue.push(chunk);
      queueBytes += sizeOf(chunk);
      while (queueBytes > MAX_QUEUE_BYTES && queue.length > 1) {
        queueBytes -= sizeOf(queue.shift()!);
      }
    }

    let ended = false;
    let attachOffset: number | null = null;
    const pending: OutputEvent[] = [];
    let disposed = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    function markEnded() {
      if (ended) return;
      ended = true;
      emit('\r\n[sesi berakhir]\r\n');
      term.options.disableStdin = true;
    }

    // Subscribe sebelum attach.
    const unsubOut = subscribeOutput(id, (ev) => {
      if (attachOffset === null) pending.push(ev);
      else if (ev.offset > attachOffset) emit(base64ToBytes(ev.data));
    });
    const unsubExit = subscribeExit(id, () => markEnded());

    const sendResize = () => { resizeSession(id, term.cols, term.rows).catch(() => {}); };

    (async () => {
      try {
        const res = await attachSession(id);
        if (disposed) return;
        emit(res.scrollback);
        attachOffset = res.offset;
        for (const ev of pending) {
          if (ev.offset > res.offset) emit(base64ToBytes(ev.data));
        }
        pending.length = 0;
        if (res.session.status === 'exited') markEnded();
        sendResize();
      } catch (e) {
        if (disposed) return;
        const msg = 'Gagal terhubung ke sesi: ' + (e instanceof Error ? e.message : String(e));
        attachError = msg;
        emit('\r\n' + msg + '\r\n');
      }
    })();

    // Program (mis. Claude Code setelah /rename) mengubah judul lewat OSC 0/2.
    term.onTitleChange((t) => onTitle?.(t));

    const unregisterPaste = registerTerminalPaste(id, (text) => {
      if (!ended) term.paste(text);
    });

    term.onData((d) => {
      if (ended) return;
      writeSession(id, d).catch(() => {});
    });

    applyVisible = (v) => {
      if (v === visibleNow) return;
      if (!v) {
        visibleNow = false;
        clearTimeout(timer);
        return;
      }
      visibleNow = true;
      tryOpen();
      for (const c of queue) term.write(c);
      queue = [];
      queueBytes = 0;
      safeFit();
      sendResize();
      try { term.refresh(0, term.rows - 1); } catch { /* belum terbuka */ }
    };
    setFontSize = (n) => {
      term.options.fontSize = n;
      if (visibleNow && opened) {
        safeFit();
        sendResize();
      }
    };

    // fit() memakai ukuran sel xterm yang ter-cache; xterm mengukur ulang sel
    // lewat event DPR terpisah (mis. zoom native webview). Fit ulang setelah
    // ukuran sel mengendap, baru kirim ukuran ke PTY.
    const refitSettled = () => {
      if (!visibleNow) return;
      tryOpen();
      safeFit();
      clearTimeout(timer);
      timer = setTimeout(() => {
        if (!visibleNow || disposed) return;
        safeFit();
        sendResize();
      }, 100);
    };
    const ro = new ResizeObserver(refitSettled);
    ro.observe(host);

    let dprMq: MediaQueryList | undefined;
    const onDprChange = () => { refitSettled(); watchDpr(); };
    const watchDpr = () => {
      dprMq?.removeEventListener('change', onDprChange);
      dprMq = window.matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`);
      dprMq.addEventListener('change', onDprChange, { once: true });
    };
    watchDpr();

    return () => {
      disposed = true;
      unregisterPaste();
      unsubOut();
      unsubExit();
      unregisterFont();
      ro.disconnect();
      dprMq?.removeEventListener('change', onDprChange);
      clearTimeout(timer);
      queue = [];
      applyVisible = () => {};
      setFontSize = () => {};
      term.dispose();
    };
  });
</script>

<div class="term-host" bind:this={host}></div>
{#if attachError}<div class="term-error">{attachError}</div>{/if}

<style>
  .term-host { width: 100%; height: 100%; min-height: 0; }
  .term-host :global(.xterm) { padding: 8px 10px; height: 100%; }
  .term-error { font-size: 0.8rem; color: #c33; padding: 0.25rem 0.5rem; }
</style>
