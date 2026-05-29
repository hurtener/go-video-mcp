<!-- Preview.svelte — the hero preview stage. Before a render it shows the first
     still as a poster; during a render, a spinner overlay; after, a "rendered"
     badge with the duration. (In-iframe playback of the server-rendered file is
     a later layer — the CSP/byte-transfer story; V1 is honest about that.) -->
<script lang="ts">
  import Icon from './Icon.svelte';
  import type { CinematicOutput } from '../lib/types.js';

  interface Props {
    posterUrl?: string;
    result: CinematicOutput | null;
    rendering: boolean;
    aspect?: string;
    progress?: number;
  }
  let { posterUrl, result, rendering, aspect = '16 / 9', progress = 0 }: Props = $props();

  function fmt(sec: number): string {
    const s = Math.max(0, Math.round(sec));
    const m = Math.floor(s / 60);
    return `${m}:${String(s % 60).padStart(2, '0')}`;
  }
  let total = $derived(result ? fmt(result.render.duration_sec) : '0:00');
</script>

<div class="stage" style="aspect-ratio: {aspect};">
  {#if posterUrl}
    <img class="poster" src={posterUrl} alt="preview" />
  {:else}
    <div class="poster empty"><Icon name="film" size={40} /></div>
  {/if}
  <div class="vignette"></div>

  {#if rendering}
    <div class="overlay">
      <div class="ring"></div>
      <div class="overlay-text">Rendering{progress > 0 ? `… ${Math.round(progress)}%` : '…'}</div>
    </div>
  {:else if result}
    <div class="done"><Icon name="check" size={13} /> Rendered</div>
  {/if}

  <div class="transport">
    <button class="play" aria-label="Play" disabled={!result}><Icon name="play" size={14} /></button>
    <div class="scrub"><div class="fill" style="width: {result ? 60 : 0}%"></div><div class="knob" style="left: {result ? 60 : 0}%"></div></div>
    <span class="time">{total}</span>
  </div>
</div>

<style>
  .stage {
    position: relative;
    width: 100%;
    border-radius: 12px;
    overflow: hidden;
    background: #000;
    border: 1px solid var(--fl-hairline);
  }
  .poster {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .poster.empty {
    display: grid;
    place-items: center;
    color: color-mix(in srgb, var(--fl-muted) 60%, transparent);
    background: radial-gradient(circle at 50% 35%, var(--fl-panel-2), #000 75%);
  }
  .vignette {
    position: absolute;
    inset: 0;
    pointer-events: none;
    box-shadow: inset 0 0 120px 30px rgba(0, 0, 0, 0.55);
    background: radial-gradient(120% 90% at 50% 45%, transparent 55%, rgba(0, 0, 0, 0.45));
  }
  .overlay {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    align-content: center;
    gap: 12px;
    background: rgba(0, 0, 0, 0.45);
    backdrop-filter: blur(2px);
  }
  .ring {
    width: 38px;
    height: 38px;
    border-radius: 50%;
    border: 3px solid color-mix(in srgb, var(--fl-accent) 30%, transparent);
    border-top-color: var(--fl-accent);
    animation: spin 0.8s linear infinite;
  }
  .overlay-text {
    color: var(--fl-text);
    font-size: 13px;
    letter-spacing: 0.02em;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .done {
    position: absolute;
    top: 10px;
    right: 10px;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 4px 9px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 600;
    color: #0b0c0e;
    background: var(--fl-ok);
  }
  .transport {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 14px;
    background: linear-gradient(to top, rgba(0, 0, 0, 0.7), transparent);
  }
  .play {
    flex: 0 0 auto;
    width: 30px;
    height: 30px;
    border-radius: 50%;
    border: 1px solid rgba(255, 255, 255, 0.25);
    background: rgba(255, 255, 255, 0.08);
    color: #fff;
    display: grid;
    place-items: center;
    cursor: pointer;
  }
  .play:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .scrub {
    position: relative;
    flex: 1;
    height: 3px;
    border-radius: 3px;
    background: rgba(255, 255, 255, 0.22);
  }
  .fill {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    border-radius: 3px;
    background: var(--fl-accent);
  }
  .knob {
    position: absolute;
    top: 50%;
    width: 11px;
    height: 11px;
    border-radius: 50%;
    background: var(--fl-accent);
    transform: translate(-50%, -50%);
    box-shadow: 0 0 8px color-mix(in srgb, var(--fl-accent) 70%, transparent);
  }
  .time {
    flex: 0 0 auto;
    font-size: 12px;
    font-variant-numeric: tabular-nums;
    color: var(--fl-text);
  }
</style>
