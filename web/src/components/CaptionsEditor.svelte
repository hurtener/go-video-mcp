<!-- CaptionsEditor.svelte — add/remove timed caption rows (title / lower-third).
     Collapsible to keep the card calm when unused. -->
<script lang="ts">
  import Icon from './Icon.svelte';
  import { CAPTION_POSITIONS, nextId, type UICaption } from '../lib/types.js';

  interface Props {
    captions: UICaption[];
  }
  let { captions = $bindable() }: Props = $props();
  let open = $state(false);

  function add() {
    open = true;
    captions = [...captions, { id: nextId(), text: '', start: 0, end: 3, position: 'lower_third' }];
  }
  function remove(id: string) {
    captions = captions.filter((c) => c.id !== id);
  }
</script>

<div class="captions">
  <button class="head" onclick={() => (open = !open)} aria-expanded={open}>
    <Icon name="sparkles" size={14} />
    <span>captions{captions.length ? ` · ${captions.length}` : ''}</span>
    <span class="caret" class:open><Icon name="chevron" size={14} /></span>
  </button>

  {#if open}
    <div class="body">
      {#each captions as cap (cap.id)}
        <div class="row">
          <input class="text" type="text" placeholder="Caption text" bind:value={cap.text} />
          <input class="num" type="number" min="0" step="0.5" bind:value={cap.start} title="Start (s)" />
          <span class="dash">→</span>
          <input class="num" type="number" min="0" step="0.5" bind:value={cap.end} title="End (s)" />
          <select class="pos" bind:value={cap.position} aria-label="Position">
            {#each CAPTION_POSITIONS as p (p.value)}
              <option value={p.value}>{p.label}</option>
            {/each}
          </select>
          <button class="rm" title="Remove caption" aria-label="Remove caption" onclick={() => remove(cap.id)}>
            <Icon name="x" size={13} />
          </button>
        </div>
      {/each}
      <button class="add" onclick={add}><Icon name="plus" size={13} /> Add caption</button>
    </div>
  {/if}
</div>

<style>
  .captions {
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
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .text {
    flex: 1;
    min-width: 0;
    padding: 6px 8px;
    border-radius: 7px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-canvas);
    color: var(--fl-text);
    font-size: 12px;
  }
  .num {
    width: 48px;
    padding: 6px 4px;
    border-radius: 7px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-canvas);
    color: var(--fl-text);
    font-size: 12px;
    text-align: center;
  }
  .dash {
    color: var(--fl-muted);
    font-size: 12px;
  }
  .pos {
    padding: 6px 4px;
    border-radius: 7px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-canvas);
    color: var(--fl-text);
    font-size: 12px;
  }
  .rm {
    flex: 0 0 auto;
    border: none;
    background: none;
    color: var(--fl-muted);
    cursor: pointer;
    display: inline-flex;
  }
  .rm:hover {
    color: var(--fl-error);
  }
  .add {
    align-self: flex-start;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 1px dashed var(--fl-hairline);
    background: none;
    color: var(--fl-muted);
    font-size: 12px;
    padding: 6px 10px;
    border-radius: 8px;
    cursor: pointer;
  }
  .add:hover {
    color: var(--fl-accent);
    border-color: var(--fl-accent);
  }
</style>
