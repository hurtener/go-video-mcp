# go-video-mcp — Contributor & Agent Normatives

> This file is **binding** for anyone — human or AI — modifying this repository.
> It adapts the engineering practices of the [Dockyard](https://github.com/hurtener/dockyard)
> framework (which this server is built on) to this project. When a rule here
> conflicts with Dockyard's own framework rules, Dockyard's framework behavior
> wins for framework concerns; this file governs project concerns.

---

## 1. What this is

`go-video-mcp` is a **Dockyard MCP server** for video editing. It exposes a small,
curated set of high-value tools over a clean FFmpeg core — deliberately **not** a
flat pile of 40 thin FFmpeg wrappers.

```text
internal/kernel/        # ffmpeg.kernel — the one place that touches FFmpeg
  Probe(input)          #   ffprobe → typed media facts
  RunPlan(plan)         #   execute a structured Plan (argv, never a shell string)
  ValidatePath(path)    #   path safety: no traversal, allowed roots, real files
  ResolveArtifact(uri)  #   uri/path → concrete, validated local path
  EmitProgress(stderr)  #   parse ffmpeg -progress / stderr → obs events
  Cancel(job_id)        #   cooperative cancellation of a running job

internal/contracts/     # typed Go structs = the source of truth (generated schema)
internal/handlers/      # one handler per tool, over the kernel
tools (registered in main.go):
  probe_media · convert_video · trim_video · extract_audio
  create_video_from_images · create_slideshow
  create_cinematic_image_video   # the killer tool: compiles a slideshow plan
                                  # into a single FFmpeg filter_complex graph
  apply_video_effect
```

The engineering value concentrates in **plan → `filter_complex` compilation**:
labeled streams, chained filters (comma-separated), separate chains
(semicolon-separated), multi-input/multi-output graphs. Effects are layered
(V1 transitions → V2 ken burns → V3 motion presets → V4 captions → V5 audio bed
→ V6 templates → V7 preview/storyboard artifacts).

---

## 2. Build, test, run (Dockyard CLI)

The `dockyard` CLI drives everything. It is not on PATH by default — the binary
lives in the sibling checkout:

```bash
export PATH="/Users/santiagobenvenuto/Repos/dockyard/bin:$PATH"
```

```bash
dockyard generate     # regenerate JSON Schema + TS from the Go contracts
dockyard validate     # quality gates — must report 0 blockers
dockyard test         # contract + spec + go-test gate
dockyard dev          # live-reload dev loop + auto-attached inspector
dockyard build        # one CGo-free static binary (UI embedded if present)
dockyard run          # build + serve (stdio default; --transport http)
dockyard install claude   # register the built server with a host
go test ./...         # the Go unit + contract tests
```

**Always run `dockyard generate` after a contract change, then `dockyard validate`.**

---

## 3. Contract-first (the headline rule)

The typed Go struct in `internal/contracts/` is the **single source of truth** for
a tool's schema. JSON Schema (`*.schema.json`) and TypeScript (`contracts.ts`) are
**generated**.

- **Never hand-edit a generated file** (`*.gen.*`, `*.schema.json`, `contracts.ts`).
  `dockyard validate` fails on drift. Edit the Go struct and regenerate.
- **Document every field** with a leading `//` comment — it becomes the schema
  `description` the model reads. An undocumented field is a missed chance to guide
  the model.
- **`omitempty` for optional fields**; the codegen reads it as "may be absent".
- **Named scalar types for constrained values** (e.g. `type TransitionStyle string`
  with documented allowed values) — guides the model and types the UI.
- **`Kind` discriminator** on outputs that drive a multi-renderer UI.

---

## 4. Go conventions

- **Toolchain.** Go 1.26. **No CGo in the shipped artifact** (`CGO_ENABLED=0`);
  `-race` test runs use CGo and are the lone exception.
- **Style.** `gofmt -s`; `go vet` and (if configured) `golangci-lint run` clean.
  Generated code stays boring and readable.
- **Errors.** `errors.Is`/`As`, `%w` wrapping, sentinel errors, `errors.Join`.
  Wrap with context. **Never `panic` for control flow; never panic across the MCP
  boundary** — a handler must return an `error`, not crash the server.
- **Context.** `context.Context` is the first parameter of anything that does I/O,
  blocks, or can be cancelled. The kernel honours cancellation — a cancelled
  context kills the FFmpeg child process.
- **Logging.** `log/slog` only. No `log.Printf`, no third-party loggers. No
  unredacted secrets or full user file contents in logs.
- **Concurrency.** Race detector on tests. Anything reused concurrently (the
  kernel, a job registry) is safe under concurrent use — prove it with a `-race`
  test.
- **Tests.** Table-driven where it fits. The FFmpeg-graph compiler is covered by
  **golden tests**: a fixed plan compiles to a fixed `filter_complex` string.

---

## 5. FFmpeg safety — non-negotiable

This server runs an external binary on user-influenced input. Treat every input as
hostile.

- **Build argv arrays, never shell strings.** Invoke FFmpeg with
  `exec.CommandContext(ctx, "ffmpeg", args...)` and an explicit `[]string`. Never
  pass a command through `sh -c`. No string interpolation into a shell.
- **Validate every path** through `kernel.ValidatePath`: reject `..` traversal,
  resolve symlinks, confine reads/writes to allowed roots, and confirm an input
  actually exists before invoking FFmpeg. Output paths must be inside an allowed
  output root.
- **Escape all `drawtext` content.** Captions/lower-thirds go through a dedicated
  escaper (`:`, `\`, `'`, `%`, newlines). Fonts come from an **allowlist** — never
  an arbitrary user-supplied font path.
- **Never execute a user-supplied raw filter string** or raw FFmpeg command. Tools
  accept structured, typed plans; the kernel compiles the graph. (`apply_video_effect`
  exposes a *bounded, named* effect set — not arbitrary filters.)
- **Resource limits.** Enforce timeouts, max input size/duration, and a max output
  resolution. A job is cancellable and bounded.
- No hardcoded secrets, anywhere — including tests.

---

## 6. Testing

- `dockyard validate` reports **0 blockers** before any commit.
- `go test ./...` (and `dockyard test`) pass.
- The graph compiler has golden tests; update goldens deliberately and review the
  diff — a changed `filter_complex` is a behavior change.
- Tests that actually shell out to FFmpeg are gated behind a build tag or a
  `FFMPEG_E2E` env check so the unit suite stays hermetic and fast.

---

## 7. Commits & branches

- **Commits:** imperative, scoped — `feat(kernel): …`, `feat(tools): …`,
  `fix(graph): …`, `docs: …`, `chore: …`. Small and coherent.
- **Branches:** feature work on `feat/*` / `fix/*` / `docs/*` once past the initial
  scaffold; don't pile unrelated work onto one commit.
- A new tool, manifest field, or contract change is **generated + validated** in the
  same commit (`dockyard generate` output is committed alongside the Go change).

---

## 8. Forbidden

- Hand-written JSON Schema or TypeScript for a tool contract.
- Passing FFmpeg work through a shell, or interpolating user input into a shell
  string.
- Executing a user-supplied raw filter graph or arbitrary FFmpeg command.
- An unvalidated path reaching FFmpeg.
- `panic` for control flow; panicking across the MCP boundary.
- A CGo runtime dependency, or building the shipped artifact with `CGO_ENABLED=1`.
- Hardcoded secrets, including in tests.
- Editing a generated file by hand instead of the Go contract.
