<!-- Recipe.svelte — a disclosure revealing the compiled FFmpeg filtergraph and
     command line for the last render. Frameline shows its work. -->
<script lang="ts">
  import Icon from './Icon.svelte';

  interface Props {
    filterComplex: string;
    command?: string;
  }
  let { filterComplex, command }: Props = $props();
  let open = $state(false);
</script>

<div class="recipe">
  <button class="head" onclick={() => (open = !open)} aria-expanded={open}>
    <Icon name="sparkles" size={14} />
    <span>the recipe</span>
    <span class="caret" class:open><Icon name="chevron" size={14} /></span>
  </button>
  {#if open}
    <div class="body">
      <div class="label">filter_complex</div>
      <pre>{filterComplex}</pre>
      {#if command}
        <div class="label">command</div>
        <pre>{command}</pre>
      {/if}
    </div>
  {/if}
</div>

<style>
  .recipe {
    border-top: 1px solid var(--fl-hairline);
    padding-top: 8px;
  }
  .head {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    border: none;
    background: none;
    color: var(--fl-muted);
    font-size: 11px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    cursor: pointer;
  }
  .head:hover {
    color: var(--fl-accent);
  }
  .caret {
    display: inline-flex;
    transition: transform 0.18s ease;
  }
  .caret.open {
    transform: rotate(180deg);
  }
  .body {
    margin-top: 8px;
  }
  .label {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--fl-muted);
    margin: 6px 0 3px;
  }
  pre {
    margin: 0;
    padding: 10px;
    border-radius: 8px;
    background: var(--fl-canvas);
    border: 1px solid var(--fl-hairline);
    color: color-mix(in srgb, var(--fl-accent-2) 80%, var(--fl-text));
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 11px;
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 180px;
    overflow: auto;
  }
</style>
