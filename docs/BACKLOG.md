# go-video-mcp — backlog

Living list of planned work. Not a commitment or a schedule — a place so ideas
don't get lost. Grouped by area; rough priority noted. Checked items are done.

## Done

- [x] ffmpeg.kernel (Probe, RunPlan, ValidatePath/ResolveArtifact, progress, Cancel)
- [x] Tools: probe_media, convert_video, trim_video, extract_audio
- [x] create_cinematic_image_video — V1 transitions + V2 Ken Burns/pan, colour grade, music bed
- [x] Slideshow compiler (pure, golden-tested) → filter_complex
- [x] list_media + ingest_media (browse roots / base64 upload)
- [x] Frameline Studio inline composer card (Svelte MCP App), embedded + served
- [x] Verified end-to-end render through the inspector host (real ffmpeg)
- [x] read_media tool + in-iframe reel playback (verified: data: video decodes
      under the sandboxed deny-by-default CSP in the inspector host)
- [x] **V4 captions** — pure-Go rendered overlay PNGs composited via FFmpeg
      `overlay` (drawtext/libfreetype absent in the common build); font allowlist;
      captions editor in the card; verified burned-in (top + lower-third)

## Planned order

Next up: **V6 → V5 → V3** (templates, then audio, then per-clip motion).
After those, rough priority: **V7** (storyboard + low-bitrate preview) →
**watermark + safe_area** (reuse the V4 overlay machinery) → **fullscreen + PiP**
UI → polish (no-host state, thumbnails) → more tools → infra/CI.

## Frameline Studio (the MCP App)

- [ ] Confirm in-iframe playback under a **real host** (Claude Desktop) — the
      inspector permits data:/blob: media; production host CSPs may be stricter
      (fallback: a low-bitrate preview transport / MCP resource).
- [ ] Graceful **no-host state** — when opened standalone (no MCP host), show a
      clear "waiting for host" message and a short handshake timeout instead of a
      blank stage for 30s.
- [ ] **Render progress in the UI** — blocked on a Dockyard bridge feature: the
      server can report progress via `TaskHandle.Progress`, but the View-side
      `@dockyard/bridge` has no task-progress notification — only
      `sendElicitationResponse`. So task progress reaches the *host* task UI, not
      the embedded card. Options: (a) upstream a bridge progress notification in
      Dockyard, then show "Rendering… %" in the card; (b) for now, run the tool as
      a task (`task_support: optional`) so progress shows in the host/inspector
      Tasks panel while the card keeps its indeterminate spinner.
- [ ] **Fullscreen** editing-suite layout (media bin / stage / inspector / timeline),
      exposing only real controls (see spec §9 capability map).
- [ ] **PiP** floating-monitor layout (preview + scrubber + status pill).
- [ ] **list_media thumbnails** — browsed files have no preview bytes; fetch a
      thumbnail (read_media or a thumbnail tool) so the Browse panel shows images.
- [x] Per-clip selection + overrides UI (motion/transition/duration) — a per-clip
      inspector on each filmstrip thumbnail (paired with V3).
- [ ] Drag-onto-stage upload (drop directly on the preview), multi-select Browse.
- [ ] Timeline with transition markers + audio waveform alignment (fullscreen).

## Cinematic engine (create_cinematic_image_video layers)

- [x] **V3** — richer per-image motion + per-clip overrides. New presets
      `diagonal_drift` (zoom + dual-axis drift) and a now-distinct `parallax_like`
      (stronger zoom + horizontal slide, no longer aliased to ken_burns). Per-clip
      overrides via a sparse `clips[]` (motion / transition / duration) indexed to
      images; the compiler uses cumulative xfade offsets so durations may differ.
      UI: a per-clip inspector on each filmstrip thumbnail. Verified mixed presets
      + variable durations against real ffmpeg. (Open: per-join hard cuts inside a
      blended reel.)
- [x] **V4** — captions / lower-thirds. Implemented as pure-Go rendered overlay
      PNGs composited via FFmpeg `overlay` (drawtext needs libfreetype, which the
      common ffmpeg build lacks). Font from an allowlist. Verified burned-in
      (top + lower-third) in a real render.
- [x] **V5** — audio polish: single-pass `loudnorm` (default on, `normalize_audio`
      toggle), `bpm`-driven `beat_sync` (snaps the per-image advance to whole
      beats so cuts land on the beat — no onset detection yet), and a closing
      `apad` so a short bed never truncates the reel. UI: normalize + cut-to-beat
      + BPM in the audio strip. Verified against real ffmpeg (−16 LUFS, beat-
      aligned length, audio not truncated). Still open: ducking (needs a VO
      input), true onset detection, `amix` when stills carry audio.
- [x] **V6** — cinematic templates (`wedding_reel`, `product_launch`,
      `memory_montage`, `travel_diary`) — named presets that set canvas/motion/
      transition/grade/timing. Pure registry in `internal/templates`; precedence
      is explicit field > template > default. UI: a prominent template picker
      that pre-fills the chips. Verified per-template against real ffmpeg.
- [ ] **V7** — storyboard JSON + preview thumbnails (and a low-bitrate preview
      render) before committing to the full render — cheap iteration.
- [ ] `watermark` overlay (contract field exists; echoed as warning today).
- [ ] `safe_area` title-safe margin enforcement (contract field exists).
- [ ] Colour-grade intensity / LUT files (mock showed an intensity slider).
- [ ] Deterministic `random_safe` seeding (currently index-cycled).

## Tools / kernel

- [ ] `create_slideshow` + `create_video_from_images` — thinner cousins over the
      compiler (reduced specs).
- [ ] `apply_video_effect` — bounded, named effect set (NOT arbitrary filters).
- [ ] Work-dir lifecycle: cleanup policy / TTL for `frameline-work` uploads + reels.
- [ ] HEIC/HEIF input support (verify ffmpeg build; may need libheif).
- [ ] Cancel surfaced as a tool (`cancel_job`) using kernel.Cancel.

## Infra / quality

- [ ] CI: GitHub Actions running `dockyard validate` + `dockyard test` + `go test -race`.
- [ ] Cross-compile release matrix (`dockyard build --cross-compile`) + checksums on a tag.
- [ ] Fuzz the filter_complex compiler; broaden kernel coverage (progress parser, RunPlan).
- [x] Inspector fixtures for create_cinematic_image_video (happy/empty/error/
      loading) for the Fixtures switcher.
- [ ] Publish `@dockyard/bridge`/`@dockyard/ui` to npm upstream → drop the relative
      `file:` web deps (tracked in Dockyard, not here).
