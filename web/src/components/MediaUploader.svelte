<!--
  MediaUploader.svelte — the dedicated media intake card. A clear, single-intent
  surface ("I want to upload media"): drop photos & music onto the server (via
  ingest_media), see them land with their server paths, then hand them off to
  the composer. Same darkroom look & feel as Frameline Studio.
-->
<script lang="ts">
  import Icon from './Icon.svelte';
  import type { IngestedItem } from '../lib/types.js';

  interface Props {
    note: string;
    roots: string[];
    items: IngestedItem[];
    onPick: (files: FileList) => void;
    onUseInStudio: () => void;
    onClear: () => void;
  }
  let { note, roots, items, onPick, onUseInStudio, onClear }: Props = $props();

  let fileInput: HTMLInputElement | undefined = $state();
  let dragging = $state(false);

  const ready = $derived(items.filter((i) => i.status === 'ready').length);
  const busy = $derived(items.some((i) => i.status === 'uploading'));

  function pick() {
    fileInput?.click();
  }
  function onFiles(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    if (input.files && input.files.length) onPick(input.files);
    input.value = '';
  }
  function onDrop(e: DragEvent) {
    e.preventDefault();
    dragging = false;
    if (e.dataTransfer?.files?.length) onPick(e.dataTransfer.files);
  }
  function fmtSize(n?: number): string {
    if (!n) return '';
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
    return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  }
  function kindIcon(kind?: string): string {
    if (kind === 'audio') return 'music';
    if (kind === 'video') return 'film';
    return 'image';
  }
</script>

<div class="uploader">
  <p class="lead">{note || 'Drop your photos and music here — they go straight onto the server, ready to compose.'}</p>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="dropzone"
    class:drag={dragging}
    role="button"
    tabindex="0"
    onclick={pick}
    onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && pick()}
    ondragover={(e) => {
      e.preventDefault();
      dragging = true;
    }}
    ondragleave={() => (dragging = false)}
    ondrop={onDrop}
  >
    <Icon name="image" size={26} />
    <span class="dz-title">Drop stills &amp; music</span>
    <span class="dz-sub">or click to choose files</span>
  </div>

  {#if items.length > 0}
    <ul class="files">
      {#each items as item (item.id)}
        <li class="file" class:errored={item.status === 'error'}>
          <span class="f-ico"><Icon name={kindIcon(item.kind)} size={15} /></span>
          <span class="f-main">
            <span class="f-name" title={item.name}>{item.name}</span>
            {#if item.status === 'ready' && item.path}
              <span class="f-path" title={item.path}>{item.path}</span>
            {:else if item.status === 'uploading'}
              <span class="f-path muted">Uploading…</span>
            {:else if item.status === 'error'}
              <span class="f-path err">{item.error ?? 'Upload failed'}</span>
            {/if}
          </span>
          <span class="f-meta">
            {#if item.status === 'uploading'}<span class="spin"></span>
            {:else if item.status === 'ready'}{fmtSize(item.size)}<Icon name="check" size={13} />
            {:else}<Icon name="alert" size={13} />{/if}
          </span>
        </li>
      {/each}
    </ul>
  {/if}

  <div class="foot">
    <span class="status">
      {#if ready > 0}
        {ready} file{ready === 1 ? '' : 's'} ready{#if roots.length}<span class="muted"> · in {roots[0]}</span>{/if}
      {:else}
        Nothing uploaded yet{#if roots.length}<span class="muted"> · files land in {roots[0]}</span>{/if}
      {/if}
    </span>
    <div class="actions">
      {#if items.length > 0}
        <button class="ghost" onclick={onClear}>Clear</button>
      {/if}
      <button class="primary" disabled={ready === 0 || busy} onclick={onUseInStudio}>
        <Icon name="film" size={15} /> Use in Frameline Studio
      </button>
    </div>
  </div>

  <input bind:this={fileInput} type="file" accept="image/*,audio/*" multiple onchange={onFiles} hidden />
</div>

<style>
  .uploader {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .lead {
    margin: 0;
    font-size: 13px;
    color: var(--fl-muted);
  }
  .dropzone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 34px 16px;
    border-radius: 14px;
    border: 1.5px dashed var(--fl-hairline);
    background:
      radial-gradient(120% 140% at 50% 0%, color-mix(in srgb, var(--fl-accent) 7%, transparent), transparent 60%),
      var(--fl-panel-2);
    color: var(--fl-muted);
    cursor: pointer;
    transition: border-color 0.18s ease, color 0.18s ease, background 0.18s ease;
  }
  .dropzone:hover,
  .dropzone:focus-visible {
    color: var(--fl-text);
    border-color: color-mix(in srgb, var(--fl-accent) 55%, var(--fl-hairline));
    outline: none;
  }
  .dropzone.drag {
    border-color: var(--fl-accent);
    color: var(--fl-text);
    background: color-mix(in srgb, var(--fl-accent) 12%, var(--fl-panel-2));
  }
  .dz-title {
    font-size: 15px;
    font-weight: 500;
    color: var(--fl-text);
  }
  .dz-sub {
    font-size: 12px;
  }
  .files {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 260px;
    overflow: auto;
  }
  .file {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border-radius: 9px;
    background: var(--fl-panel-2);
    border: 1px solid var(--fl-hairline);
  }
  .file.errored {
    border-color: var(--fl-error);
  }
  .f-ico {
    color: var(--fl-accent-2);
    display: inline-flex;
    flex: 0 0 auto;
  }
  .f-main {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
  }
  .f-name {
    font-size: 12.5px;
    color: var(--fl-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .f-path {
    font-size: 10.5px;
    color: var(--fl-accent-2);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .f-path.muted {
    color: var(--fl-muted);
    font-family: inherit;
  }
  .f-path.err {
    color: var(--fl-error);
    font-family: inherit;
  }
  .f-meta {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    flex: 0 0 auto;
    font-size: 11px;
    color: var(--fl-muted);
  }
  .spin {
    width: 13px;
    height: 13px;
    border-radius: 50%;
    border: 2px solid color-mix(in srgb, var(--fl-accent) 35%, transparent);
    border-top-color: var(--fl-accent);
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    flex-wrap: wrap;
  }
  .status {
    font-size: 12px;
    color: var(--fl-text);
  }
  .muted {
    color: var(--fl-muted);
  }
  .actions {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }
  .ghost {
    border: 1px solid var(--fl-hairline);
    background: var(--fl-panel-2);
    color: var(--fl-muted);
    border-radius: 9px;
    padding: 8px 12px;
    font-size: 12px;
    cursor: pointer;
  }
  .ghost:hover {
    color: var(--fl-text);
  }
  .primary {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    border: none;
    border-radius: 9px;
    padding: 9px 14px;
    font-size: 13px;
    font-weight: 500;
    color: #1a1206;
    background: var(--fl-accent);
    cursor: pointer;
  }
  .primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
