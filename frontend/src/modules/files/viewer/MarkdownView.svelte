<script lang="ts">
  import { parseMarkdown, type Block, type Inline } from './markdown';

  let { source }: { source: string } = $props();
  const blocks = $derived(parseMarkdown(source));

  function openExternal(e: MouseEvent, href: string) {
    const open = (window as any).runtime?.BrowserOpenURL;
    if (typeof open === 'function') {
      e.preventDefault();
      open(href);
    }
  }
</script>

{#snippet inlineView(nodes: Inline[])}
  {#each nodes as n}
    {#if n.t === 'text'}{n.v}
    {:else if n.t === 'code'}<code>{n.v}</code>
    {:else if n.t === 'strong'}<strong>{@render inlineView(n.c)}</strong>
    {:else if n.t === 'em'}<em>{@render inlineView(n.c)}</em>
    {:else if n.t === 'del'}<del>{@render inlineView(n.c)}</del>
    {:else if n.t === 'br'}<br />
    {:else if n.t === 'link'}<a href={n.href} target="_blank" rel="noopener noreferrer" onclick={(e) => openExternal(e, n.href)}>{@render inlineView(n.c)}</a>
    {/if}
  {/each}
{/snippet}

{#snippet blockView(b: Block)}
  {#if b.t === 'heading'}
    {#if b.level === 1}<h1>{@render inlineView(b.c)}</h1>
    {:else if b.level === 2}<h2>{@render inlineView(b.c)}</h2>
    {:else if b.level === 3}<h3>{@render inlineView(b.c)}</h3>
    {:else if b.level === 4}<h4>{@render inlineView(b.c)}</h4>
    {:else if b.level === 5}<h5>{@render inlineView(b.c)}</h5>
    {:else}<h6>{@render inlineView(b.c)}</h6>
    {/if}
  {:else if b.t === 'para'}
    <p>{@render inlineView(b.c)}</p>
  {:else if b.t === 'code'}
    <pre><code>{b.v}</code></pre>
  {:else if b.t === 'quote'}
    <blockquote>
      {#each b.c as c}{@render blockView(c)}{/each}
    </blockquote>
  {:else if b.t === 'list'}
    {#if b.ordered}
      <ol start={b.start}>
        {#each b.items as item}<li>{#each item as c}{@render blockView(c)}{/each}</li>{/each}
      </ol>
    {:else}
      <ul>
        {#each b.items as item}<li>{#each item as c}{@render blockView(c)}{/each}</li>{/each}
      </ul>
    {/if}
  {:else if b.t === 'hr'}
    <hr />
  {:else if b.t === 'table'}
    <table>
      <thead>
        <tr>
          {#each b.head as cell, ci}
            <th class={b.align[ci] ? 'a-' + b.align[ci] : ''}>{@render inlineView(cell)}</th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each b.rows as row}
          <tr>
            {#each row as cell, ci}
              <td class={b.align[ci] ? 'a-' + b.align[ci] : ''}>{@render inlineView(cell)}</td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
{/snippet}

<div class="markdown" role="region" aria-label="Pratinjau markdown">
  {#each blocks as b}{@render blockView(b)}{/each}
</div>

<style>
  .markdown {
    height: 100%;
    box-sizing: border-box;
    overflow: auto;
    padding: 12px 20px;
    max-width: 100%;
    color: var(--fg, inherit);
    line-height: 1.6;
    overflow-wrap: anywhere;
    word-break: break-word;
  }
  .markdown > :global(*) { max-width: 90ch; }
  .markdown :global(h1), .markdown :global(h2) { border-bottom: 1px solid var(--border, currentColor); padding-bottom: 0.25em; }
  .markdown :global(h1), .markdown :global(h2), .markdown :global(h3),
  .markdown :global(h4), .markdown :global(h5), .markdown :global(h6) { margin: 1.2em 0 0.5em; line-height: 1.25; }
  .markdown :global(p) { margin: 0.6em 0; }
  .markdown :global(pre) {
    overflow-x: auto;
    padding: 10px 12px;
    background: var(--bg-elev, rgba(127, 127, 127, 0.12));
    border-radius: 4px;
    word-break: normal;
    overflow-wrap: normal;
  }
  .markdown :global(code) { font-family: ui-monospace, monospace; font-size: 0.92em; }
  .markdown :global(:not(pre) > code) { background: var(--bg-elev, rgba(127, 127, 127, 0.15)); padding: 0.1em 0.35em; border-radius: 3px; }
  .markdown :global(blockquote) { margin: 0.6em 0; padding: 0 12px; border-left: 3px solid var(--border, currentColor); color: var(--fg-muted, inherit); }
  .markdown :global(ul), .markdown :global(ol) { padding-left: 1.6em; margin: 0.5em 0; }
  .markdown :global(hr) { border: 0; border-top: 1px solid var(--border, currentColor); margin: 1em 0; }
  .markdown :global(a) { color: var(--accent, inherit); text-decoration: underline; }
  .markdown :global(table) { border-collapse: collapse; display: block; overflow-x: auto; max-width: 100%; }
  .markdown :global(th), .markdown :global(td) { border: 1px solid var(--border, currentColor); padding: 4px 10px; text-align: left; }
  .markdown :global(.a-left) { text-align: left; }
  .markdown :global(.a-right) { text-align: right; }
  .markdown :global(.a-center) { text-align: center; }
</style>
