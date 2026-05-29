# go-video-mcp

An MCP server for video editing — probe, convert, trim, extract audio, and
compile image sequences into **cinematic slideshow videos**. Built with
[Dockyard](https://github.com/hurtener/dockyard) (contract-first, single static
binary) and powered by [FFmpeg](https://ffmpeg.org/).

## Why this exists

Most FFmpeg MCP servers expose 40 thin wrappers — one per FFmpeg sub-feature.
go-video-mcp instead offers a small set of high-value tools over one clean
**FFmpeg kernel**, and concentrates its engineering on a flagship tool that
compiles a declarative slideshow spec into a single FFmpeg `filter_complex`
graph (labeled streams, per-image motion, transitions, colour grade, music bed).

## Requirements

- [FFmpeg](https://ffmpeg.org/) (`ffmpeg` + `ffprobe`) on your `PATH`.
- Go 1.26+ to build.

## Tools

| Tool | What it does |
| ---- | ------------ |
| `probe_media` | ffprobe → typed facts (format, duration, dimensions, fps, codecs). |
| `convert_video` | Re-encode to a new container / codec / size / frame rate. |
| `trim_video` | Cut a `[start, end)` range (fast stream-copy or frame-accurate). |
| `extract_audio` | Pull the audio track out to mp3 / m4a / wav / flac. |
| `create_cinematic_image_video` | **Flagship.** Compile images into a cinematic slideshow. |

### `create_cinematic_image_video`

Compiles a sequence of images into a slideshow with:

- **Canvas presets** — `1920x1080` (landscape), `1080x1920` (portrait/reel), or a custom `WxH`.
- **Per-image motion** — `ken_burns`, `slow_push`, `pan_left`, `pan_right` (Ken Burns family).
- **Transitions** — `fade`, `wipe`, `slide`, `zoom_blur`, `film_dissolve`, `random_safe` (cross-faded with `xfade`).
- **Colour grade** — `neutral`, `warm`, `cinematic`, `vintage`, `high_contrast`.
- **Music bed** — optional background audio with fade-in/out, length-matched to the reel.
- **Timing** — set `duration_per_image`, or set `total_duration` and let the per-image time be derived (handy for matching a song).

It returns the produced file **and** the compiled `filter_complex` graph, so the
caller can inspect or refine the render.

Captions, watermark, beat-sync, and safe-area are accepted by the contract today
and reported back as warnings until their render layers land (see the roadmap).

## Architecture

```text
internal/kernel/      ffmpeg.kernel — the ONLY place that touches FFmpeg.
                      Probe · RunPlan · ValidatePath · ResolveArtifact ·
                      progress parsing · Cancel. Builds argv arrays, never
                      shell strings. Confines every path to allowed roots.
internal/slideshow/   The slideshow compiler: a declarative Spec → a typed
                      FilterGraph → a kernel.Plan. Pure (no I/O), golden-tested.
internal/contracts/   Typed Go contracts (source of truth) + GENERATED schema/TS.
internal/handlers/    One thin handler per tool, over the kernel.
```

## Run

```sh
go run .                           # serve over stdio (the default)
DOCKYARD_TRANSPORT=http go run .   # serve the streamable-HTTP transport

# Confine reads/writes to specific directories (defaults to the working dir):
GO_VIDEO_MCP_ROOTS="$HOME/Pictures:$HOME/Movies" go run .
```

Install into an MCP host (Claude, Cursor) with Dockyard:

```sh
dockyard build
dockyard install claude
```

## Develop

```sh
dockyard dev        # live-reload loop + local inspector
dockyard generate   # regenerate schema/TS after a contract change
dockyard validate   # quality gates (0 blockers expected)
dockyard test       # contract + spec + go-test gate
go test ./...       # unit + golden tests (hermetic; no FFmpeg needed)

# Real-FFmpeg end-to-end render tests (opt-in):
FFMPEG_E2E=1 go test ./internal/... -run E2E -v
```

See [`CLAUDE.md`](./CLAUDE.md) for the engineering conventions (contract-first,
FFmpeg safety rules, testing).

## Roadmap

The cinematic tool grows in layers:

- ✅ V1 — crossfade / wipe / slide transitions
- ✅ V2 — Ken Burns motion (zoompan) + pan presets
- ⏳ V3 — richer per-image motion presets (diagonal drift, parallax)
- ⏳ V4 — captions / lower-thirds (drawtext, with safe escaping + font allowlist)
- ⏳ V5 — audio ducking + loudness normalisation
- ⏳ V6 — cinematic templates ("wedding reel", "product launch", "memory montage")
- ⏳ V7 — storyboard JSON + preview thumbnails before the full render
- ⏳ Additional tools — `create_slideshow`, `create_video_from_images`, `apply_video_effect`

## License

[Apache-2.0](./LICENSE).
