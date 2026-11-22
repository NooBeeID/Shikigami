<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    open,
    onclose,
    title,
    children,
  }: { open: boolean; onclose: () => void; title?: string; children: Snippet } = $props();

  let dialogEl: HTMLDivElement | undefined = $state();

  const FOCUSABLE =
    'a[href], button:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  function focusables(): HTMLElement[] {
    if (!dialogEl) return [];
    return Array.from(dialogEl.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
      (el) => el.getClientRects().length > 0,
    );
  }

  function onBackdrop(e: PointerEvent) {
    if (e.target === e.currentTarget) onclose();
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation();
      e.preventDefault();
      onclose();
      return;
    }
    if (e.key !== 'Tab' || !dialogEl) return;
    const items = focusables();
    if (items.length === 0) {
      e.preventDefault();
      dialogEl.focus();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    const active = document.activeElement as HTMLElement | null;
    const inside = !!active && dialogEl.contains(active) && active !== dialogEl;
    if (e.shiftKey) {
      if (!inside || active === first) {
        e.preventDefault();
        last.focus();
      }
    } else if (!inside || active === last) {
      e.preventDefault();
      first.focus();
    }
  }

  $effect(() => {
    if (!open) return;
    const trigger = document.activeElement as HTMLElement | null;
    window.addEventListener('keydown', onKey, true);
    queueMicrotask(() => {
      if (!dialogEl) return;
      const body = dialogEl.querySelector<HTMLElement>('.body');
      const target = body
        ? Array.from(body.querySelectorAll<HTMLElement>(FOCUSABLE)).find(
            (el) => el.getClientRects().length > 0,
          )
        : undefined;
      (target ?? dialogEl).focus();
    });
    return () => {
      window.removeEventListener('keydown', onKey, true);
      if (trigger && trigger.isConnected) trigger.focus();
    };
  });
</script>

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="backdrop" onpointerdown={onBackdrop}>
    <div
      class="dialog"
      role="dialog"
      aria-modal="true"
      aria-label={title}
      tabindex="-1"
      bind:this={dialogEl}
    >
      <header>
        <span class="title">{title}</span>
        <button type="button" class="close" aria-label="Tutup" onclick={onclose}>×</button>
      </header>
      <div class="body">{@render children()}</div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 2000;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .dialog {
    background: var(--bg-elev, #222);
    color: var(--fg, #eee);
    border: 1px solid var(--border, #444);
    border-radius: 8px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
    width: 760px;
    height: 520px;
    max-width: calc(100vw - 48px);
    max-height: calc(100vh - 48px);
    display: flex;
    flex-direction: column;
    outline: none;
  }
  header {
    display: flex;
    flex-direction: row;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border, #444);
  }
  .title {
    font-weight: 600;
  }
  .close {
    background: transparent;
    border: none;
    color: var(--fg-muted, #999);
    font-size: 20px;
    line-height: 1;
    padding: 2px 8px;
    border-radius: 4px;
    cursor: pointer;
  }
  .close:hover {
    background: var(--bg-hover, #333);
    color: var(--fg, #eee);
  }
  .body {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }
</style>
