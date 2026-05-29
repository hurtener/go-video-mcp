<!--
  TemplatePicker.svelte — the V6 cinematic-template picker. A prominent row of
  selectable pills; choosing one pre-fills the look/motion/timing chips with the
  template's preset (the parent applies it via onPick), which the user can then
  tweak. "Custom" carries no preset — set everything by hand.
-->
<script lang="ts">
  import Icon from './Icon.svelte';
  import { TEMPLATES } from '../lib/types.js';

  interface Props {
    value: string;
    onPick: (value: string) => void;
  }
  let { value, onPick }: Props = $props();

  const active = $derived(TEMPLATES.find((t) => t.value === value) ?? TEMPLATES[0]);
</script>

<div class="picker">
  <div class="head">
    <span class="title"><Icon name="sparkles" size={15} /> Template</span>
    <span class="hint">{active.hint}</span>
  </div>
  <div class="pills" role="radiogroup" aria-label="Cinematic template">
    {#each TEMPLATES as t (t.value)}
      <button
        type="button"
        role="radio"
        aria-checked={t.value === value}
        class="pill"
        class:active={t.value === value}
        title={t.hint}
        onclick={() => onPick(t.value)}
      >
        {t.label}
      </button>
    {/each}
  </div>
</div>

<style>
  .picker {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    border-radius: 12px;
    background:
      radial-gradient(120% 140% at 0% 0%, color-mix(in srgb, var(--fl-accent) 8%, transparent), transparent 55%),
      var(--fl-panel-2);
    border: 1px solid var(--fl-hairline);
  }
  .head {
    display: flex;
    align-items: baseline;
    gap: 10px;
    flex-wrap: wrap;
  }
  .title {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--fl-accent);
    font-weight: 600;
  }
  .hint {
    font-size: 12px;
    color: var(--fl-muted);
  }
  .pills {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .pill {
    padding: 7px 12px;
    border-radius: 999px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-canvas);
    color: var(--fl-text);
    font-size: 12.5px;
    font-weight: 500;
    cursor: pointer;
    transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
    white-space: nowrap;
  }
  .pill:hover {
    border-color: color-mix(in srgb, var(--fl-accent) 50%, var(--fl-hairline));
  }
  .pill.active {
    border-color: var(--fl-accent);
    background: color-mix(in srgb, var(--fl-accent) 16%, var(--fl-canvas));
    color: var(--fl-text);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--fl-accent) 20%, transparent);
  }
</style>
