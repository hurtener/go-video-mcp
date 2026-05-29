<!-- Chip.svelte — a compact labeled <select> styled as a control chip. The
     native select keeps it accessible and keyboard-friendly; the chrome is
     cinematic. -->
<script lang="ts">
  import Icon from './Icon.svelte';

  interface Option {
    value: string;
    label: string;
  }
  interface Props {
    icon: string;
    label: string;
    value: string;
    options: readonly Option[];
    onchange?: (value: string) => void;
  }
  let { icon, label, value = $bindable(), options, onchange }: Props = $props();

  function handle(e: Event) {
    value = (e.currentTarget as HTMLSelectElement).value;
    onchange?.(value);
  }
</script>

<label class="chip">
  <span class="chip-icon"><Icon name={icon} size={16} /></span>
  <span class="chip-text">
    <span class="chip-label">{label}</span>
    <span class="chip-value">{options.find((o) => o.value === value)?.label ?? value}</span>
  </span>
  <span class="chip-caret"><Icon name="chevron" size={14} /></span>
  <select {value} onchange={handle} aria-label={label}>
    {#each options as o (o.value)}
      <option value={o.value}>{o.label}</option>
    {/each}
  </select>
</label>

<style>
  .chip {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border-radius: 10px;
    background: var(--fl-panel-2);
    border: 1px solid var(--fl-hairline);
    color: var(--fl-text);
    cursor: pointer;
    transition: border-color 0.18s ease, background 0.18s ease;
    min-width: 0;
  }
  .chip:hover {
    border-color: color-mix(in srgb, var(--fl-accent) 50%, var(--fl-hairline));
  }
  .chip:focus-within {
    border-color: var(--fl-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--fl-accent) 22%, transparent);
  }
  .chip-icon {
    color: var(--fl-muted);
    display: inline-flex;
  }
  .chip-text {
    display: flex;
    flex-direction: column;
    line-height: 1.1;
    min-width: 0;
  }
  .chip-label {
    font-size: 10px;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--fl-muted);
  }
  .chip-value {
    font-size: 13px;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .chip-caret {
    color: var(--fl-muted);
    display: inline-flex;
  }
  /* The native select overlays the whole chip, invisible but interactive. */
  select {
    position: absolute;
    inset: 0;
    opacity: 0;
    width: 100%;
    height: 100%;
    cursor: pointer;
    border: 0;
  }
</style>
