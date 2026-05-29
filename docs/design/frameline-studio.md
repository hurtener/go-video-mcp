# Frameline Studio — MCP App design spec

> The interactive UI for `go-video-mcp`'s flagship tool
> `create_cinematic_image_video`. Frameline Studio lets a person drop in photos
> and a music track and watch an agent compose, grade, and refine a cinematic
> reel — directly inside the chat surface.

**Name.** *Frameline Studio*. "Frameline" = the line between film frames + a
timeline; cinematic, professional, brand-able, not tied to any single occasion.

**Locked visual direction.** The look below is realised in
[`mockups/`](./mockups/):

- [`01-inline-composer-card.png`](./mockups/01-inline-composer-card.png) — inline
- [`02-fullscreen-editing-suite.png`](./mockups/02-fullscreen-editing-suite.png) — fullscreen
- [`03-pip-floating-monitor.png`](./mockups/03-pip-floating-monitor.png) — PiP

These are the design target. §9 maps every control in them to what the backend
actually does today, so the built UI stays honest.

---

## 1. Product in one sentence

A chat-native cinematic editing suite: upload stills + a song, and an AI editor
arranges them into a graded, music-timed reel — you reorder the filmstrip, nudge
the look, and re-render, all without leaving the conversation.

## 2. Who it's for

Anyone turning a folder of photos into something that feels *made*, not
auto-generated: creators assembling a launch teaser, a couple making an
anniversary montage, a brand cutting a product reel, a family making a memory
piece. The bar is "looks like a pro touched it."

---

## 3. Look & feel

The aesthetic is **a darkroom editing suite, not a SaaS form**. Calm,
theatrical, editorial — the UI recedes so the imagery leads.

**Palette** (cinematic teal-orange, dark-first):

| Token | Value | Use |
| ----- | ----- | --- |
| `canvas` | `#0B0C0E` | app background (near-black, faintly warm) |
| `panel` | `#14161A` | raised panels, glassy with subtle blur |
| `hairline` | `#23262D` | 1px dividers, thumbnail borders |
| `text` | `#F4F1EA` | primary text (warm off-white) |
| `muted` | `#9AA0A6` | secondary text, labels |
| `accent` | `#F0A23B` | film-amber: primary actions, playhead, focus glow |
| `accent-2` | `#2DD4BF` | teal: selection, audio waveform, secondary highlights |
| `render-ok` | `#7BD88F` | render-complete state |

Always derive from the host theme first (CSS custom properties like
`--color-background-primary`); the palette above is the **fallback** when the
host supplies nothing, and the brand look in our own marketing shots.

**Texture & depth.** A whisper of film grain (~4% opacity) over the canvas; a
soft vignette on the preview stage; glass panels (backdrop-blur, ~8% white
overlay) floating over the dark base with one diffuse shadow. No hard borders
except hairlines.

**Type.** UI in a refined grotesk (Geist / Inter). The wordmark and section
titles in a high-contrast editorial serif (Fraunces / Instrument Serif), used
sparingly — it signals "craft". Numerals tabular for timecodes.

**Shape & motion.** Panels 14px radius, controls 8px, thumbnails 8px. The
filmstrip carries a subtle sprocket-hole motif along its top/bottom edge. Motion
is 200–280ms ease-out; the playhead glows amber and "scrubs" smoothly; thumbnails
lift slightly on drag. Nothing bounces. Icons are thin line icons (Lucide).

**Voice.** Microcopy is a confident editor's: "Drop your stills", "Set the look",
"Cut to the beat", "Render the reel". Never "Submit".

---

## 4. The three render modes (MCP Apps protocol)

MCP Apps Views declare which display modes they support; the host decides what to
grant (`displayMode` arrives in host context). Frameline Studio is built
**mode-aware** — it reads `displayMode` and lays itself out accordingly. Dockyard
V1 renders **inline**; fullscreen and PiP are designed now so they light up the
moment the host/Dockyard grant them.

### 4.1 Inline — the *composer card* (default)

Embedded in the conversation as a message, sized to chat width (~520–720px),
height-bounded but internally scrollable. This is the "at a glance + quick nudge"
surface.

```
┌──────────────────────────────────────────────┐
│  ◐ Frameline Studio              ⤢ Fullscreen │  ← title + promote affordance
│ ┌────────────────────────────────────────────┐│
│ │                                            ││
│ │        ▶  PREVIEW (16:9 / 9:16)            ││  ← hero player, vignette
│ │              ●━━━━━━━━━━━━──── 0:07         ││  ← amber playhead
│ └────────────────────────────────────────────┘│
│  ▢ ▢ ▢ ▢ ▢ ▢   + add        ← filmstrip (drag) │
│  Canvas[16:9 ▾] Motion[Ken Burns ▾] Look[Warm ▾]│ ← control chips
│  ♪ song.mp3  ▁▂▅▇▅▂▁  fade 1s/2s               │ ← audio bed strip
│  [ Render the reel ]        recipe ⌄            │ ← primary action + graph
└──────────────────────────────────────────────┘
```

- **Hero preview** on top (matches chosen canvas aspect), with a slim transport.
- **Filmstrip** of reorderable thumbnails (drag to reorder, `+` to add).
- **Control chips** — canvas, motion, transition, colour grade — as compact
  dropdowns, not a wall of form fields.
- **Audio bed strip** — track name + a mini waveform + fade values.
- **Primary action** renders; a disclosure reveals the compiled `filter_complex`
  ("the recipe") for the curious.

### 4.2 Fullscreen — the *editing suite*

Takes over the space below the host header (header shows the title + an ✕ to
exit). The serious workspace: a classic NLE three-zone + timeline.

```
┌───────────────────────────────────────────────────────────────┐
│  ◐ Frameline Studio                                        ✕   │
│ ┌─────────┬───────────────────────────────┬──────────────────┐ │
│ │ MEDIA   │                               │  INSPECTOR        │ │
│ │ BIN     │      LARGE PREVIEW STAGE       │  Canvas  16:9     │ │
│ │ ▢ ▢     │        ▶   (vignette)          │  FPS     30       │ │
│ │ ▢ ▢     │                               │  Motion  Ken Burns│ │
│ │ ▢ ▢     │   ●━━━━━━━━━━━━━━━━━━── 0:07/0:21│ Transition Fade  │ │
│ │ ⤓ drop  │                               │  Grade   Cinematic│ │
│ │ to add  │                               │  ─ per-clip ─     │ │
│ └─────────┴───────────────────────────────┴──────────────────┘ │
│  TIMELINE   ▢──▢──▢──▢──▢──▢   (transitions marked ◇)           │
│  AUDIO      ▁▂▅▇█▇▅▂▁▂▅▇█▇▅▂▁  (beats ┊, transition markers ◇)   │
└───────────────────────────────────────────────────────────────┘
```

- **Left rail — Media bin:** the upload dropzone + an image grid; thumbnails drag
  into the timeline.
- **Center — Stage:** large preview with a full transport (play, scrub,
  in/out), timecode.
- **Right — Inspector:** every global control, plus a **per-clip** section
  (override motion/transition/duration on the selected thumbnail — the V3 surface).
- **Bottom — Timeline:** the filmstrip track aligned above an **audio waveform**;
  transition points render as diamond markers, beats as ticks (the beat-sync
  story). Dragging a transition marker over a beat is the "cut to the beat"
  gesture.

### 4.3 PiP — the *floating monitor*

A small floating window pinned above the conversation, the only mode that honours
the host `maxHeight` (content scrolls inside). For *watching while you chat* —
keep the reel playing/monitoring render progress while the agent iterates.

```
┌───────────────────────┐
│ ◐ Frameline      ⤢  ✕ │
│ ┌───────────────────┐ │
│ │   ▶  PREVIEW       │ │
│ │  ●━━━━━━──  0:07   │ │
│ └───────────────────┘ │
│  ● Rendering… 62%      │ ← live render/status pill
└───────────────────────┘
```

- Just the preview + slim scrubber + a **status pill** (idle / rendering % /
  done). Tapping ⤢ promotes to fullscreen. On mobile widths the host may promote
  PiP → fullscreen automatically.

---

## 5. Core flows

1. **Add media** — drag photos onto the dropzone (or `+`); drag a song onto the
   audio strip. Thumbnails appear in order added.
2. **Arrange** — drag thumbnails to reorder; the timeline + duration update live.
3. **Set the look** — pick canvas, motion, transition, grade from chips/inspector.
   Defaults are good out of the box (Ken Burns + fade + neutral, 4s/image).
4. **Match the music** — set total duration to the song length (the tool derives
   per-image timing); add fade-in/out. (Beat-sync: drag transition markers to
   beats — future.)
5. **Render** — calls `create_cinematic_image_video`; the preview swaps to the
   rendered file; the "recipe" disclosure shows the exact `filter_complex`.
6. **Refine** — the agent (or the user) tweaks one knob and re-renders; iteration
   is the whole point.

## 6. The four required states (Dockyard §20)

Every mode routes through the shared `PageState`:

- **Loading** — a filmstrip skeleton + a soft amber shimmer on the stage; "Composing…".
- **Empty** — the dropzone front-and-centre: "Drop your stills to begin" with a
  subtle film-frame illustration. Real copy, a working drop target.
- **Error** — render failed: the FFmpeg error tail in a calm panel + a "Try again"
  that re-runs the last plan. Never a dead end.
- **Ready** — the composer card / suite as drawn above.

(Permission state, if the host gates file access: "Frameline needs access to your
media folder" + the host's grant affordance.)

## 7. Build notes — reusable components

Compose, don't hand-roll. **Dockyard's stack is `@dockyard/ui` components +
`--dy-*` design tokens, not Tailwind/shadcn** — and the App ships as a single
**iife** bundle (the sandboxed iframe has no `allow-same-origin`, so module
scripts won't run). So we compose the Dockyard design system and add a couple of
focused libraries, all bundled into the single file.

| Need | Choice |
| ---- | ------ |
| Chrome + the four states (loading/empty/error/permission) | **`@dockyard/ui`** — `PageState`, `ActionBar`, `StatusChip`, `CodeBlock` (for the "recipe"), `AppShell` (fullscreen later) |
| Reorderable filmstrip / timeline (accessible, any input) | **svelte-dnd-action** |
| Upload dropzone | native HTML5 drag-drop + File API (no extra dep needed); `filedrop-svelte` optional |
| Audio waveform + beat markers | **wavesurfer.js** (bundled) |
| Preview player | native `<video>` |
| Buttons / chips / sliders not in `@dockyard/ui` | hand-styled with `--dy-*` tokens + the §3 cinematic fallback |

**Bridge:** `createBridge({ displayModes: ['inline'] })`; receive via
`onToolResult` / `onToolInput`; **call tools via `bridge.callTool(name, args)`**
(`ingest_media`, `list_media`, `create_cinematic_image_video`). `@dockyard/bridge`
and `@dockyard/ui` are workspace `file:` deps — the `web/` build needs the local
Dockyard checkout (`--dockyard-path`); the Go side stays on the published module.

**Theme:** apply the host's `styles.variables` to the root (the bridge
propagates them); the §3 cinematic palette is the fallback when the host supplies
none. **CSP** stays deny-by-default — everything (wavesurfer, fonts) is bundled
into the single file, so `connect: []` / `resource: []` holds.

---

## 8. Why it's promotable

It's the rare MCP App that is *visibly* a craft tool: a dark, cinematic editing
suite living inside a chat, where an agent does the tedious composition and the
human does the taste. The inline composer card is a great screenshot; the
fullscreen suite is a great demo video; the PiP monitor is a great "look, it
keeps working while you talk" moment. Built on `go-video-mcp` + Dockyard — a clean
kernel, a real `filter_complex` engine, contract-first.

---

## 9. Control → capability mapping (build honestly)

The mockups are a north star and intentionally show more than the V1 backend can
do. The **built** UI exposes only what `create_cinematic_image_video` honors
today; aspirational controls are either hidden until their layer lands or shown
disabled with a "coming" affordance — never a control that silently does nothing.

**Inline composer card** — maps cleanly; this is the first build target.

| Mockup control | Backend today | Action |
| -------------- | ------------- | ------ |
| Canvas 16:9 ▾ | `canvas` (presets + custom WxH) | ✅ ship |
| Motion Ken Burns ▾ | `motion_style` | ✅ ship |
| Look Warm ▾ | `color_grade` | ✅ ship |
| song.mp3 · fade 1s/2s + waveform | `background_audio`, `audio_fade_in/out_seconds`; waveform via wavesurfer | ✅ ship (beat markers decorative until beat-sync) |
| Filmstrip reorder / add | `images[]` order | ✅ ship |
| Render the reel | `tools/call create_cinematic_image_video` | ✅ ship |
| Preview + scrub | the **rendered** mp4 (no client-side pre-render) — poster/empty before first render | ✅ ship, honest about timing |
| **Transition** | `transition_style` + `transition_seconds` — **missing from the mock** | ➕ add a 4th chip (or settings popover) |
| ⚙ settings (top-right) | `fps`, `duration_per_image` / `total_duration`, `transition_seconds` | ✅ ship as an "advanced" popover |

**Fullscreen suite** — north star; build progressively, only the real rows.

| Mockup control | Backend today | Action |
| -------------- | ------------- | ------ |
| Aspect Ratio / Resolution | `canvas` (WxH) | ✅ |
| FPS | `fps` | ✅ |
| Transition Default + Duration | `transition_style` + `transition_seconds` | ✅ |
| Grade LUT | `color_grade` | ✅ (named grade, not a LUT file) |
| Timeline transition diamonds | per-join transition | ⏳ V3 (per-clip) |
| Background colour | — | ⛔ not a tool param (drop or add later) |
| Motion Stabilization + Amount | — | ⛔ not in tool; the real "Motion" row is the `motion_style` preset |
| Grade Intensity / Rec.709·Log·HDR | — | ⛔ grades are fixed; no intensity/colorspace |
| Per-clip Opacity/Speed/Position/Scale | — | ⛔ beyond scope; V3 per-clip = motion/transition/duration only |
| Audio volume envelope / keyframes | — | ⏳ V5 (ducking, normalise) |
| Color / Deliver pages, Autosave | — | ⛔ vision only (no project persistence yet) |

**PiP monitor** — fully buildable: preview + scrubber + a render-status pill fed
by the kernel's `-progress` stream (job id + percent), expand → fullscreen.

**Principle.** A control that the backend can't honor does not ship enabled. When
a layer lands (V3 per-clip, V5 audio), its controls light up — and the tool
contract grows in the same PR (contract-first).
