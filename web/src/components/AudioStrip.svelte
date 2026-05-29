<!-- AudioStrip.svelte — the music bed: pick a track, see its waveform
     (wavesurfer.js, bundled), and set fade in/out. The waveform renders when a
     local preview URL is available; a browsed track without bytes shows a
     decorative bar. -->
<script lang="ts">
  import WaveSurfer from 'wavesurfer.js';
  import Icon from './Icon.svelte';
  import type { AudioBed } from '../lib/types.js';

  interface Props {
    audio: AudioBed | null;
    fadeIn: number;
    fadeOut: number;
    onPick: (file: File) => void;
    onClear: () => void;
  }
  let { audio, fadeIn = $bindable(), fadeOut = $bindable(), onPick, onClear }: Props = $props();

  let fileInput: HTMLInputElement | undefined = $state();
  let waveEl: HTMLDivElement | undefined = $state();
  let ws: WaveSurfer | undefined;

  function pick() {
    fileInput?.click();
  }
  function onFile(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    if (input.files && input.files[0]) onPick(input.files[0]);
    input.value = '';
  }

  // (Re)build the waveform whenever the previewable track changes.
  $effect(() => {
    const url = audio?.previewUrl;
    ws?.destroy();
    ws = undefined;
    if (url && waveEl) {
      ws = WaveSurfer.create({
        container: waveEl,
        url,
        height: 28,
        waveColor: getCss('--fl-accent-2', '#2dd4bf'),
        progressColor: getCss('--fl-accent-2', '#2dd4bf'),
        cursorWidth: 0,
        barWidth: 2,
        barGap: 1,
        barRadius: 2,
        interact: false,
      });
    }
    return () => {
      ws?.destroy();
      ws = undefined;
    };
  });

  function getCss(name: string, fallback: string): string {
    if (typeof getComputedStyle === 'undefined' || !waveEl) return fallback;
    const v = getComputedStyle(waveEl).getPropertyValue(name).trim();
    return v || fallback;
  }
</script>

<div class="audio">
  {#if audio}
    <span class="ico"><Icon name="music" size={16} /></span>
    <span class="name" title={audio.name}>{audio.name}</span>
    <div class="wave" bind:this={waveEl}>
      {#if !audio.previewUrl}<div class="static-bar"></div>{/if}
    </div>
    <label class="fade" title="Fade in (s)">
      in <input type="number" min="0" step="0.5" bind:value={fadeIn} />
    </label>
    <label class="fade" title="Fade out (s)">
      out <input type="number" min="0" step="0.5" bind:value={fadeOut} />
    </label>
    <button class="ghost" aria-label="Remove track" title="Remove track" onclick={onClear}><Icon name="x" size={14} /></button>
  {:else}
    <span class="ico"><Icon name="music" size={16} /></span>
    <button class="add" onclick={pick}><Icon name="plus" size={14} /> Add a music bed</button>
  {/if}
  <input bind:this={fileInput} type="file" accept="audio/*" onchange={onFile} hidden />
</div>

<style>
  .audio {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    border-radius: 10px;
    background: var(--fl-panel-2);
    border: 1px solid var(--fl-hairline);
    min-height: 44px;
  }
  .ico {
    color: var(--fl-muted);
    display: inline-flex;
    flex: 0 0 auto;
  }
  .name {
    flex: 0 0 auto;
    max-width: 130px;
    font-size: 12px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--fl-text);
  }
  .wave {
    flex: 1;
    min-width: 40px;
    height: 28px;
    display: flex;
    align-items: center;
  }
  .static-bar {
    width: 100%;
    height: 12px;
    border-radius: 6px;
    background: repeating-linear-gradient(
      90deg,
      color-mix(in srgb, var(--fl-accent-2) 70%, transparent) 0 2px,
      transparent 2px 4px
    );
    opacity: 0.6;
  }
  .fade {
    flex: 0 0 auto;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fl-muted);
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .fade input {
    width: 42px;
    padding: 3px 4px;
    border-radius: 6px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-canvas);
    color: var(--fl-text);
    font-size: 11px;
  }
  .add {
    border: none;
    background: none;
    color: var(--fl-muted);
    font-size: 12px;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .add:hover {
    color: var(--fl-accent);
  }
  .ghost {
    flex: 0 0 auto;
    border: none;
    background: none;
    color: var(--fl-muted);
    cursor: pointer;
    display: inline-flex;
  }
  .ghost:hover {
    color: var(--fl-text);
  }
</style>
