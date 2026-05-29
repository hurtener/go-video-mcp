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

## Frameline Studio (the MCP App)

- [ ] **In-iframe video playback** of the rendered reel (CSP/bytes story) — *in progress*
- [ ] Graceful **no-host state** — when opened standalone (no MCP host), show a
      clear "waiting for host" message and a short handshake timeout instead of a
      blank stage for 30s.
- [ ] **Render progress in the UI** — stream ffmpeg `-progress` (% / frame) to the
      card; needs `task_support` + progress notifications (today render is sync).
      Unlocks the PiP "Rendering… 62%" pill.
- [ ] **Fullscreen** editing-suite layout (media bin / stage / inspector / timeline),
      exposing only real controls (see spec §9 capability map).
- [ ] **PiP** floating-monitor layout (preview + scrubber + status pill).
- [ ] **list_media thumbnails** — browsed files have no preview bytes; fetch a
      thumbnail (read_media or a thumbnail tool) so the Browse panel shows images.
- [ ] Per-clip selection + overrides UI (motion/transition/duration) — pairs with V3.
- [ ] Drag-onto-stage upload (drop directly on the preview), multi-select Browse.
- [ ] Timeline with transition markers + audio waveform alignment (fullscreen).

## Cinematic engine (create_cinematic_image_video layers)

- [ ] **V3** — richer per-image motion presets (diagonal drift, true `parallax_like`;
      today parallax aliases ken_burns) + per-clip overrides in the contract.
- [ ] **V4** — captions / lower-thirds via `drawtext` with safe escaping + a font
      allowlist (contract already accepts `captions[]`; today echoed as warnings).
- [ ] **V5** — audio: ducking, loudness normalize (`loudnorm`), `amix` when stills
      carry audio, beat-detection for `beat_sync` (contract field exists).
- [ ] **V6** — cinematic templates ("wedding reel", "product launch", "memory
      montage") — named presets that set canvas/motion/transition/grade/timing.
- [ ] **V7** — storyboard JSON + preview thumbnails (and a low-bitrate preview
      render) before committing to the full render — cheap iteration.
- [ ] `watermark` overlay (contract field exists; echoed as warning today).
- [ ] `safe_area` title-safe margin enforcement (contract field exists).
- [ ] Colour-grade intensity / LUT files (mock showed an intensity slider).
- [ ] Deterministic `random_safe` seeding (currently index-cycled).

## Tools / kernel

- [ ] `read_media` — read a media file under the roots as a (capped) data URI —
      *in progress, for in-iframe playback*.
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
- [ ] Inspector fixtures for create_cinematic_image_video (happy/empty/error) for the
      Fixtures switcher.
- [ ] Publish `@dockyard/bridge`/`@dockyard/ui` to npm upstream → drop the relative
      `file:` web deps (tracked in Dockyard, not here).
