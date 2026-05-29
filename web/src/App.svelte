<!--
  App.svelte — Frameline Studio, the inline composer card.

  An interactive MCP App over go-video-mcp's create_cinematic_image_video:
  drop stills, reorder the filmstrip, set the look, add a music bed, and render
  a cinematic reel — calling list_media / ingest_media / create_cinematic_image_
  video through the Dockyard bridge. The model can also drive the tool directly;
  onToolInput pre-fills the card and onToolResult shows the rendered result.

  Mode-aware: built for inline today (Dockyard V1), reading displayMode so the
  fullscreen / PiP layouts can grow from the same component later.
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { createBridge, type StyleVariables, type DisplayMode } from '@dockyard/bridge';

  import Icon from './components/Icon.svelte';
  import Chip from './components/Chip.svelte';
  import Filmstrip from './components/Filmstrip.svelte';
  import AudioStrip from './components/AudioStrip.svelte';
  import Preview from './components/Preview.svelte';
  import Recipe from './components/Recipe.svelte';
  import { applyHostVariables } from './theme.js';
  import {
    CANVAS_PRESETS,
    MOTION_OPTIONS,
    TRANSITION_OPTIONS,
    GRADE_OPTIONS,
    nextId,
    fileToBase64,
    type Clip,
    type AudioBed,
    type CinematicInput,
    type CinematicOutput,
    type IngestMediaOutput,
    type ListMediaOutput,
    type ReadMediaOutput,
    type MediaItem,
  } from './lib/types.js';

  // --- editor state --------------------------------------------------------
  let clips = $state<Clip[]>([]);
  let audio = $state<AudioBed | null>(null);
  let fadeIn = $state(1);
  let fadeOut = $state(2);
  let canvas = $state('1920x1080');
  let fps = $state(30);
  let motion = $state('ken_burns');
  let transition = $state('fade');
  let grade = $state('neutral');
  // advanced
  let secondsPerImage = $state(4);
  let transitionSeconds = $state(1);
  let advancedOpen = $state(false);

  // --- runtime state -------------------------------------------------------
  let rendering = $state(false);
  let renderError = $state<string | null>(null);
  let result = $state<CinematicOutput | null>(null);
  let videoUrl = $state<string | undefined>();
  let videoTooLarge = $state(false);
  let lastFetched: string | undefined;
  let displayMode = $state<DisplayMode>('inline');
  let connected = $state(false);
  let connectError = $state<string | null>(null);

  // browse panel
  let browseOpen = $state(false);
  let browseItems = $state<MediaItem[]>([]);
  let browseError = $state<string | null>(null);

  let rootEl = $state<HTMLDivElement>();

  const readyClips = $derived(clips.filter((c) => c.status === 'ready' && c.path));
  const posterUrl = $derived(clips.find((c) => c.previewUrl)?.previewUrl);
  const aspect = $derived(canvasAspect(canvas));
  // Page states (the four-state rule): connecting → loading; no stills yet →
  // empty; render failure → error (banner below).
  const connecting = $derived(!connected && !connectError);
  const isEmpty = $derived(clips.length === 0 && !result && !rendering);

  const bridge = createBridge({ displayModes: ['inline'] });

  // The model may invoke create_cinematic_image_video directly; reflect its
  // input in the card so the human sees what the agent set.
  const offInput = bridge.onToolInput<CinematicInput>((input) => {
    if (!input) return;
    if (input.canvas) canvas = input.canvas;
    if (input.fps) fps = input.fps;
    if (input.motion_style) motion = input.motion_style;
    if (input.transition_style) transition = input.transition_style;
    if (input.color_grade) grade = input.color_grade;
    if (input.transition_seconds) transitionSeconds = input.transition_seconds;
    if (input.duration_per_image) secondsPerImage = input.duration_per_image;
    if (Array.isArray(input.images) && clips.length === 0) {
      clips = input.images.map((p) => ({
        id: nextId(),
        name: baseName(p),
        path: p,
        status: 'ready' as const,
      }));
    }
  });

  const offResult = bridge.onToolResult<CinematicOutput>((r) => {
    if (r.structuredContent) {
      result = r.structuredContent;
      rendering = false;
      renderError = null;
    }
  });

  let currentVars: StyleVariables | undefined;
  const offHost = bridge.onHostContextChanged((p) => {
    if (p.styles?.variables) {
      currentVars = p.styles.variables;
      if (rootEl) applyHostVariables(rootEl, currentVars);
    }
    if (p.displayMode) displayMode = p.displayMode;
  });

  // When a render produces a file, fetch its bytes (read_media) so the reel
  // plays inline. Guarded by lastFetched so the effect doesn't re-fetch.
  $effect(() => {
    const path = result?.render?.output_path;
    if (path && path !== lastFetched) {
      lastFetched = path;
      void fetchPreview(path);
    }
  });

  async function fetchPreview(path: string) {
    videoUrl = undefined;
    videoTooLarge = false;
    try {
      const res = await bridge.callTool<unknown, ReadMediaOutput>('read_media', { path });
      const out = res.structuredContent;
      if (out?.truncated) videoTooLarge = true;
      else if (out?.data_uri) videoUrl = out.data_uri;
    } catch {
      /* keep the poster — playback is best-effort */
    }
  }

  onMount(() => {
    bridge
      .connect()
      .then(() => {
        connected = true;
      })
      .catch((err: unknown) => {
        connectError = `Bridge handshake failed: ${(err as Error)?.message ?? err}`;
      });
    if (rootEl && currentVars) applyHostVariables(rootEl, currentVars);
  });

  onDestroy(() => {
    offInput();
    offResult();
    offHost();
    bridge.close();
    // Revoke any object URLs we created for previews.
    for (const c of clips) if (c.previewUrl?.startsWith('blob:')) URL.revokeObjectURL(c.previewUrl);
    if (audio?.previewUrl?.startsWith('blob:')) URL.revokeObjectURL(audio.previewUrl);
  });

  // --- actions -------------------------------------------------------------

  async function addFiles(files: FileList) {
    const list = Array.from(files);
    for (const file of list) {
      const clip: Clip = {
        id: nextId(),
        name: file.name,
        previewUrl: URL.createObjectURL(file),
        status: 'uploading',
      };
      clips = [...clips, clip];
      try {
        const b64 = await fileToBase64(file);
        const res = await bridge.callTool<unknown, IngestMediaOutput>('ingest_media', {
          filename: file.name,
          data_base64: b64,
        });
        if (res.isError || !res.structuredContent) throw new Error('ingest failed');
        update(clip.id, { path: res.structuredContent.path, status: 'ready' });
      } catch (err) {
        update(clip.id, { status: 'error', error: (err as Error)?.message ?? 'failed' });
      }
    }
  }

  function update(id: string, patch: Partial<Clip>) {
    clips = clips.map((c) => (c.id === id ? { ...c, ...patch } : c));
  }

  function removeClip(id: string) {
    const c = clips.find((x) => x.id === id);
    if (c?.previewUrl?.startsWith('blob:')) URL.revokeObjectURL(c.previewUrl);
    clips = clips.filter((x) => x.id !== id);
  }

  async function addAudio(file: File) {
    const previewUrl = URL.createObjectURL(file);
    try {
      const res = await bridge.callTool<unknown, IngestMediaOutput>('ingest_media', {
        filename: file.name,
        data_base64: await fileToBase64(file),
      });
      if (res.isError || !res.structuredContent) throw new Error('ingest failed');
      audio = { name: file.name, path: res.structuredContent.path, previewUrl };
    } catch {
      audio = null;
      URL.revokeObjectURL(previewUrl);
    }
  }

  function clearAudio() {
    if (audio?.previewUrl?.startsWith('blob:')) URL.revokeObjectURL(audio.previewUrl);
    audio = null;
  }

  async function toggleBrowse() {
    browseOpen = !browseOpen;
    if (!browseOpen) return;
    browseError = null;
    try {
      const res = await bridge.callTool<unknown, ListMediaOutput>('list_media', { kinds: ['image'] });
      browseItems = res.structuredContent?.items ?? [];
      if (browseItems.length === 0) browseError = 'No images found under the server roots.';
    } catch (err) {
      browseError = (err as Error)?.message ?? 'Browse failed';
    }
  }

  function addFromBrowse(item: MediaItem) {
    clips = [...clips, { id: nextId(), name: item.name, path: item.path, status: 'ready' }];
  }

  async function render() {
    const images = readyClips.map((c) => c.path!) as string[];
    if (images.length === 0) {
      renderError = 'Add at least one still before rendering.';
      return;
    }
    renderError = null;
    rendering = true;
    const args: CinematicInput = {
      images,
      canvas,
      fps,
      motion_style: motion,
      transition_style: transition,
      color_grade: grade,
      duration_per_image: secondsPerImage,
      transition_seconds: transitionSeconds,
    };
    if (audio?.path) {
      args.background_audio = audio.path;
      args.audio_fade_in_seconds = fadeIn;
      args.audio_fade_out_seconds = fadeOut;
    }
    try {
      const res = await bridge.callTool<CinematicInput, CinematicOutput>(
        'create_cinematic_image_video',
        args,
      );
      if (res.isError || !res.structuredContent) {
        throw new Error(textOf(res.content) || 'render failed');
      }
      result = res.structuredContent;
    } catch (err) {
      renderError = (err as Error)?.message ?? 'Render failed.';
    } finally {
      rendering = false;
    }
  }

  // --- helpers -------------------------------------------------------------
  function canvasAspect(c: string): string {
    const m = /^(\d+)x(\d+)$/.exec(c.trim());
    return m ? `${m[1]} / ${m[2]}` : '16 / 9';
  }
  function baseName(p: string): string {
    return p.split(/[\\/]/).pop() ?? p;
  }
  function textOf(content: unknown): string {
    if (Array.isArray(content)) {
      return content
        .map((b) => (b && typeof b === 'object' && 'text' in b ? String((b as { text: unknown }).text) : ''))
        .join(' ')
        .trim();
    }
    return '';
  }
</script>

<div class="frameline" bind:this={rootEl} data-display={displayMode} data-testid="frameline">
  <header class="bar">
    <h1>Frameline Studio</h1>
    <button class="icon-btn" title="Advanced settings" aria-label="Advanced settings" onclick={() => (advancedOpen = !advancedOpen)}>
      <Icon name="sliders" size={18} />
    </button>
  </header>

  <Preview {posterUrl} {result} {rendering} {aspect} {videoUrl} />

  {#if connecting}
    <p class="hint" data-state="loading">
      <span class="dot"></span> Loading Frameline Studio…
    </p>
  {:else if isEmpty}
    <p class="hint" data-state="empty">
      <Icon name="image" size={15} /> Your reel is empty — drop stills below to begin.
    </p>
  {/if}

  {#if advancedOpen}
    <div class="advanced">
      <label>FPS <input type="number" min="1" max="60" bind:value={fps} /></label>
      <label>Seconds / image <input type="number" min="0.5" step="0.5" bind:value={secondsPerImage} /></label>
      <label>Transition (s) <input type="number" min="0" step="0.25" bind:value={transitionSeconds} /></label>
    </div>
  {/if}

  <div class="film-row">
    <Filmstrip bind:clips onAdd={addFiles} onRemove={removeClip} />
    <div class="browse">
      <button class="ghost-btn" onclick={toggleBrowse}><Icon name="folder" size={15} /> Browse</button>
      {#if browseOpen}
        <div class="browse-panel">
          {#if browseError}
            <p class="browse-msg">{browseError}</p>
          {:else if browseItems.length === 0}
            <p class="browse-msg">Searching…</p>
          {:else}
            {#each browseItems as item (item.path)}
              <button class="browse-item" onclick={() => addFromBrowse(item)} title={item.path}>
                <Icon name="image" size={14} /> <span>{item.name}</span>
              </button>
            {/each}
          {/if}
        </div>
      {/if}
    </div>
  </div>

  <div class="chips">
    <Chip icon="monitor" label="Canvas" bind:value={canvas} options={CANVAS_PRESETS} />
    <Chip icon="motion" label="Motion" bind:value={motion} options={MOTION_OPTIONS} />
    <Chip icon="sun" label="Look" bind:value={grade} options={GRADE_OPTIONS} />
    <Chip icon="film" label="Transition" bind:value={transition} options={TRANSITION_OPTIONS} />
  </div>

  <AudioStrip {audio} bind:fadeIn bind:fadeOut onPick={addAudio} onClear={clearAudio} />

  {#if renderError}
    <div class="banner error"><Icon name="alert" size={15} /> {renderError}</div>
  {:else if connectError}
    <div class="banner warn"><Icon name="alert" size={15} /> {connectError}</div>
  {:else if result?.warnings && result.warnings.length}
    <div class="banner warn"><Icon name="alert" size={15} /> {result.warnings.join(' · ')}</div>
  {:else if videoTooLarge && result}
    <div class="banner warn"><Icon name="alert" size={15} /> Reel rendered — too large to preview inline; saved to {result.render.output_path}</div>
  {/if}

  <button class="render" onclick={render} disabled={rendering || readyClips.length === 0}>
    <Icon name="film" size={18} />
    {rendering ? 'Rendering…' : 'Render the reel'}
  </button>

  {#if result}
    <Recipe filterComplex={result.filter_complex} command={result.render.command} />
  {/if}
</div>

<style>
  /* Cinematic palette — derived from the host theme when present (the bridge
     applies --dy-* vars to this root), with Frameline's dark fallback. */
  .frameline {
    --fl-canvas: var(--dy-color-canvas, #0b0c0e);
    --fl-panel: var(--dy-color-surface, #14161a);
    --fl-panel-2: var(--dy-color-surface-2, #1b1e24);
    --fl-hairline: var(--dy-color-border, #23262d);
    --fl-text: var(--dy-color-ink, #f4f1ea);
    --fl-muted: var(--dy-color-ink-soft, #9aa0a6);
    --fl-accent: var(--dy-color-accent, #f0a23b);
    --fl-accent-2: #2dd4bf;
    --fl-ok: #7bd88f;
    --fl-error: var(--dy-state-error-fg, #e06c5b);

    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 18px;
    max-width: 760px;
    margin: 0 auto;
    border-radius: 16px;
    background:
      radial-gradient(140% 120% at 50% -10%, color-mix(in srgb, var(--fl-accent) 5%, transparent), transparent 60%),
      var(--fl-canvas);
    color: var(--fl-text);
    font-family: var(--dy-font-sans, system-ui, -apple-system, 'Segoe UI', sans-serif);
    /* faint film grain */
    position: relative;
  }
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  h1 {
    margin: 0;
    font-size: 24px;
    font-weight: 500;
    letter-spacing: 0.01em;
    font-family: var(--dy-font-serif, Georgia, 'Times New Roman', serif);
  }
  .icon-btn {
    width: 38px;
    height: 38px;
    border-radius: 10px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-panel-2);
    color: var(--fl-muted);
    display: grid;
    place-items: center;
    cursor: pointer;
  }
  .icon-btn:hover {
    color: var(--fl-text);
    border-color: color-mix(in srgb, var(--fl-accent) 45%, var(--fl-hairline));
  }
  .hint {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    padding: 9px 12px;
    border-radius: 9px;
    font-size: 12.5px;
    color: var(--fl-muted);
    background: var(--fl-panel-2);
    border: 1px dashed var(--fl-hairline);
  }
  .hint .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    border: 2px solid color-mix(in srgb, var(--fl-accent) 35%, transparent);
    border-top-color: var(--fl-accent);
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .advanced {
    display: flex;
    flex-wrap: wrap;
    gap: 14px;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--fl-panel-2);
    border: 1px solid var(--fl-hairline);
  }
  .advanced label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--fl-muted);
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .advanced input {
    width: 56px;
    padding: 4px 6px;
    border-radius: 6px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-canvas);
    color: var(--fl-text);
    font-size: 12px;
  }
  .film-row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
  }
  .film-row :global(.strip-wrap) {
    flex: 1;
    min-width: 0;
  }
  .browse {
    position: relative;
    flex: 0 0 auto;
    padding-top: 6px;
  }
  .ghost-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 10px;
    border-radius: 9px;
    border: 1px solid var(--fl-hairline);
    background: var(--fl-panel-2);
    color: var(--fl-muted);
    font-size: 12px;
    cursor: pointer;
    white-space: nowrap;
  }
  .ghost-btn:hover {
    color: var(--fl-accent);
    border-color: color-mix(in srgb, var(--fl-accent) 45%, var(--fl-hairline));
  }
  .browse-panel {
    position: absolute;
    right: 0;
    top: 100%;
    margin-top: 6px;
    width: 240px;
    max-height: 240px;
    overflow: auto;
    z-index: 5;
    padding: 6px;
    border-radius: 10px;
    background: var(--fl-panel);
    border: 1px solid var(--fl-hairline);
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.55);
  }
  .browse-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 7px 8px;
    border: none;
    border-radius: 7px;
    background: none;
    color: var(--fl-text);
    font-size: 12px;
    cursor: pointer;
    text-align: left;
  }
  .browse-item span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .browse-item:hover {
    background: var(--fl-panel-2);
  }
  .browse-msg {
    margin: 0;
    padding: 10px;
    font-size: 12px;
    color: var(--fl-muted);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .chips :global(.chip) {
    flex: 1 1 150px;
  }
  .banner {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 9px 12px;
    border-radius: 9px;
    font-size: 12.5px;
  }
  .banner.error {
    background: color-mix(in srgb, var(--fl-error) 16%, transparent);
    color: var(--fl-error);
    border: 1px solid color-mix(in srgb, var(--fl-error) 40%, transparent);
  }
  .banner.warn {
    background: color-mix(in srgb, var(--fl-accent) 12%, transparent);
    color: color-mix(in srgb, var(--fl-accent) 80%, var(--fl-text));
    border: 1px solid color-mix(in srgb, var(--fl-accent) 30%, transparent);
  }
  .render {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    width: 100%;
    padding: 14px;
    border: none;
    border-radius: 12px;
    font-size: 15px;
    font-weight: 600;
    letter-spacing: 0.01em;
    color: #0b0c0e;
    cursor: pointer;
    background: linear-gradient(180deg, color-mix(in srgb, var(--fl-accent) 92%, #fff), var(--fl-accent));
    box-shadow: 0 8px 24px color-mix(in srgb, var(--fl-accent) 28%, transparent);
    transition: transform 0.12s ease, filter 0.18s ease;
  }
  .render:hover:not(:disabled) {
    filter: brightness(1.05);
  }
  .render:active:not(:disabled) {
    transform: translateY(1px);
  }
  .render:disabled {
    opacity: 0.5;
    cursor: default;
    box-shadow: none;
  }
</style>
