<!-- Filmstrip.svelte — the reorderable strip of image clips, with film-
     perforation chrome. Drag to reorder (svelte-dnd-action, accessible + any
     input device); the add tile opens the file picker. -->
<script lang="ts">
  import { dndzone, type DndEvent } from 'svelte-dnd-action';
  import { flip } from 'svelte/animate';
  import Icon from './Icon.svelte';
  import { CLIP_MOTION_OPTIONS, CLIP_TRANSITION_OPTIONS, CLIP_FIT_OPTIONS, type Clip } from '../lib/types.js';

  interface Props {
    clips: Clip[];
    onAdd: (files: FileList) => void;
    onRemove: (id: string) => void;
    onUpdate: (id: string, patch: Partial<Clip>) => void;
  }
  let { clips = $bindable(), onAdd, onRemove, onUpdate }: Props = $props();

  let fileInput: HTMLInputElement | undefined = $state();
  // V3: which clip's per-clip override panel is open (null = none).
  let editingId = $state<string | null>(null);

  const editing = $derived(clips.find((c) => c.id === editingId) ?? null);
  const editingIndex = $derived(clips.findIndex((c) => c.id === editingId));
  const isLast = $derived(editingIndex === clips.length - 1);

  function hasOverride(c: Clip): boolean {
    return !!(c.motion || c.transition || c.fit || (c.duration && c.duration > 0));
  }
  function toggleEdit(id: string) {
    editingId = editingId === id ? null : id;
  }
  function clearOverrides(id: string) {
    onUpdate(id, { motion: '', transition: '', fit: '', duration: undefined });
  }

  function consider(e: CustomEvent<DndEvent<Clip>>) {
    clips = e.detail.items;
  }
  function finalize(e: CustomEvent<DndEvent<Clip>>) {
    clips = e.detail.items;
  }
  function pick() {
    fileInput?.click();
  }
  function onFiles(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    if (input.files && input.files.length) onAdd(input.files);
    input.value = '';
  }
</script>

<div class="strip-wrap">
  <div
    class="strip"
    use:dndzone={{ items: clips, flipDurationMs: 180, dropTargetStyle: {} }}
    onconsider={consider}
    onfinalize={finalize}
  >
    {#each clips as clip (clip.id)}
      <div class="frame" animate:flip={{ duration: 180 }} class:uploading={clip.status === 'uploading'} class:errored={clip.status === 'error'}>
        {#if clip.previewUrl}
          <img src={clip.previewUrl} alt={clip.name} draggable="false" />
        {:else}
          <div class="placeholder"><Icon name="image" size={20} /></div>
        {/if}
        {#if clip.status === 'uploading'}
          <div class="badge spin" title="Uploading…"></div>
        {:else if clip.status === 'error'}
          <div class="badge err" title={clip.error ?? 'Failed'}><Icon name="alert" size={12} /></div>
        {:else if hasOverride(clip)}
          <div class="badge ov" title="Has per-clip overrides"></div>
        {/if}
        <button class="remove" title="Remove" aria-label="Remove {clip.name}" onclick={() => onRemove(clip.id)}>
          <Icon name="x" size={12} />
        </button>
        {#if clip.status === 'ready'}
          <button
            class="edit"
            class:on={editingId === clip.id}
            title="Per-clip overrides"
            aria-label="Edit clip {clip.name}"
            data-dnd-ignore
            onclick={() => toggleEdit(clip.id)}
          >
            <Icon name="sliders" size={12} />
          </button>
        {/if}
      </div>
    {/each}

    <button class="frame add" onclick={pick} title="Add stills" data-dnd-ignore>
      <Icon name="plus" size={20} />
      <span>add</span>
    </button>
  </div>

  {#if editing}
    <!-- V3 per-clip inspector: override motion / transition / duration for the
         selected still; "Inherit" / blank falls back to the global settings. -->
    <div class="clip-inspector">
      <div class="ci-head">
        <span class="ci-title">Clip {editingIndex + 1} · {editing.name}</span>
        <div class="ci-actions">
          {#if hasOverride(editing)}
            <button class="ci-reset" onclick={() => clearOverrides(editing!.id)}>Reset</button>
          {/if}
          <button class="ci-close" aria-label="Close" onclick={() => (editingId = null)}><Icon name="x" size={13} /></button>
        </div>
      </div>
      <div class="ci-fields">
        <label>
          Motion
          <select value={editing.motion ?? ''} onchange={(e) => onUpdate(editing!.id, { motion: (e.currentTarget as HTMLSelectElement).value })}>
            {#each CLIP_MOTION_OPTIONS as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
          </select>
        </label>
        <label>
          Fit
          <select value={editing.fit ?? ''} onchange={(e) => onUpdate(editing!.id, { fit: (e.currentTarget as HTMLSelectElement).value })}>
            {#each CLIP_FIT_OPTIONS as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
          </select>
        </label>
        <label class:dim={isLast}>
          Transition
          <select
            value={editing.transition ?? ''}
            disabled={isLast}
            title={isLast ? 'The last clip has no following transition' : 'Transition into the next clip'}
            onchange={(e) => onUpdate(editing!.id, { transition: (e.currentTarget as HTMLSelectElement).value })}
          >
            {#each CLIP_TRANSITION_OPTIONS as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
          </select>
        </label>
        <label>
          Seconds
          <input
            type="number"
            min="0"
            step="0.5"
            placeholder="auto"
            value={editing.duration ?? ''}
            onchange={(e) => {
              const v = parseFloat((e.currentTarget as HTMLInputElement).value);
              onUpdate(editing!.id, { duration: Number.isFinite(v) && v > 0 ? v : undefined });
            }}
          />
        </label>
      </div>
    </div>
  {/if}

  <input
    bind:this={fileInput}
    type="file"
    accept="image/*"
    multiple
    onchange={onFiles}
    hidden
  />
</div>

<style>
  .strip-wrap {
    width: 100%;
  }
  .strip {
    display: flex;
    gap: 8px;
    overflow-x: auto;
    padding: 6px 2px 10px;
    scrollbar-width: thin;
  }
  .frame {
    position: relative;
    flex: 0 0 auto;
    width: 92px;
    height: 64px;
    border-radius: 8px;
    overflow: hidden;
    background: var(--fl-panel-2);
    border: 1px solid var(--fl-hairline);
    /* film-perforation top/bottom edge */
    background-image:
      radial-gradient(circle, var(--fl-canvas) 1.4px, transparent 1.6px),
      radial-gradient(circle, var(--fl-canvas) 1.4px, transparent 1.6px);
    background-size: 10px 4px;
    background-position: top 1px left 2px, bottom 1px left 2px;
    background-repeat: repeat-x;
    cursor: grab;
    transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
  }
  .frame:active {
    cursor: grabbing;
  }
  .frame:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.5);
    border-color: color-mix(in srgb, var(--fl-accent) 45%, var(--fl-hairline));
  }
  .frame img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .placeholder {
    width: 100%;
    height: 100%;
    display: grid;
    place-items: center;
    color: var(--fl-muted);
    background: linear-gradient(135deg, var(--fl-panel-2), var(--fl-canvas));
  }
  .frame.uploading {
    opacity: 0.7;
  }
  .frame.errored {
    border-color: var(--fl-error);
  }
  .badge {
    position: absolute;
    top: 4px;
    left: 4px;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    display: grid;
    place-items: center;
  }
  .badge.spin {
    border: 2px solid color-mix(in srgb, var(--fl-accent) 35%, transparent);
    border-top-color: var(--fl-accent);
    animation: spin 0.7s linear infinite;
  }
  .badge.err {
    background: var(--fl-error);
    color: #fff;
  }
  .badge.ov {
    width: 9px;
    height: 9px;
    top: 5px;
    left: 5px;
    background: var(--fl-accent-2);
    box-shadow: 0 0 0 2px rgba(0, 0, 0, 0.4);
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .remove {
    position: absolute;
    top: 4px;
    right: 4px;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    border: none;
    background: rgba(0, 0, 0, 0.55);
    color: #fff;
    display: grid;
    place-items: center;
    opacity: 0;
    cursor: pointer;
    transition: opacity 0.15s ease;
  }
  .frame:hover .remove {
    opacity: 1;
  }
  .edit {
    position: absolute;
    bottom: 4px;
    right: 4px;
    width: 18px;
    height: 18px;
    border-radius: 5px;
    border: none;
    background: rgba(0, 0, 0, 0.55);
    color: #fff;
    display: grid;
    place-items: center;
    opacity: 0;
    cursor: pointer;
    transition: opacity 0.15s ease, background 0.15s ease;
  }
  .frame:hover .edit {
    opacity: 1;
  }
  .edit.on {
    opacity: 1;
    background: var(--fl-accent);
    color: #1a1206;
  }
  .clip-inspector {
    margin-top: 4px;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--fl-panel-2);
    border: 1px solid color-mix(in srgb, var(--fl-accent) 35%, var(--fl-hairline));
  }
  .ci-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
  }
  .ci-title {
    font-size: 12px;
    font-weight: 500;
    color: var(--fl-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .ci-actions {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    flex: 0 0 auto;
  }
  .ci-reset {
    border: none;
    background: none;
    color: var(--fl-muted);
    font-size: 11px;
    cursor: pointer;
  }
  .ci-reset:hover {
    color: var(--fl-accent);
  }
  .ci-close {
    border: none;
    background: none;
    color: var(--fl-muted);
    display: inline-flex;
    cursor: pointer;
  }
  .ci-close:hover {
    color: var(--fl-text);
  }
  .ci-fields {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
  }
  .ci-fields label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fl-muted);
  }
  .ci-fields label.dim {
    opacity: 0.5;
  }
  .ci-fields select,
  .ci-fields input {
    padding: 5px 7px;
    border-radius: 7px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-canvas);
    color: var(--fl-text);
    font-size: 12px;
    min-width: 92px;
  }
  .ci-fields input {
    width: 70px;
    min-width: 0;
  }
  .add {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    color: var(--fl-muted);
    border-style: dashed;
    background-image: none;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    cursor: pointer;
  }
  .add:hover {
    color: var(--fl-accent);
    border-color: var(--fl-accent);
  }
</style>
