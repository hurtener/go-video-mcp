# Dockyard MCP-Apps integration feedback

Findings from building an inline MCP App (Frameline Studio) on Dockyard and
trying to render it in **Claude Desktop** (local connector, via
`npx mcp-remote http://127.0.0.1:8080/mcp`). Reference baseline:
**pengui-slides**, which uses the official `@modelcontextprotocol/ext-apps`
SDK and renders in Claude Desktop.

**Key outcome:** the App renders correctly in a **spec-compliant local MCP-Apps
playground**, so Dockyard's Apps stack (resource serving, `_meta.ui` link,
`dockyard-bridge` `ui/` postMessage dialect, single-file bundle) is
fundamentally sound. The remaining Claude-Desktop-specific non-render is tracked
as OPEN below. The items are labelled **[CONFIRMED]** (a real defect we
observed), **[ALIGN]** (a divergence from the official ext-apps SDK we matched
to, not proven strictly required), or **[OPEN]** (unresolved).

---

## 1. `.UI(appName)` silently emitted no `_meta.ui` — [CONFIRMED, fixed in v1.5.0]

Pre-1.5.0, `tool.New[...].UI(appName).Register(srv)` registered the tool but the
definition went out with **no `_meta.ui.resourceUri`**. `runtime/tool/builder.go`
`Register` built `server.ToolDef{Name, Description}` and never consumed
`b.uiResource`; `.UI()` only recorded the name (the doc comment said "the Apps
layer consumes it" — nothing did). A host with only the tool result to go on
(Claude Desktop) saw no App link and rendered text.

- Observed: raw `tools/list` returned `_meta: {}` for every UI tool.
- Fixed in **v1.5.0 (D-173)** — `.UI()` now emits `_meta.ui.resourceUri`. ✓
- Lingering: it was a *silent* no-op with green `validate` — see §5.

## 2. `ui.domain` is invalid for local connectors, but Dockyard makes it a static, registration-time choice — [CONFIRMED]

When the App declares `Domain` + `HostProfile: "claude"` (+ `ServerURL`),
`resources/read` emits `_meta.ui.domain = <sha256>.claudemcpcontent.com` (the
signed origin, RFC §7.5 / D-062/063). Claude Desktop **rejects** that on a local
connector:

> `ui.domain cannot be used with local connectors. Stable sandbox origins require a remote connector with a verified URL.`

A local stdio command — **including `npx mcp-remote <url>`** — is a local
connector, so any server that sets the domain breaks there.

The deeper friction: **`HostProfile`/`Domain` are static `apps.Register` fields,
but local-vs-remote is a per-connection fact** negotiated at `initialize`. No
single static value is correct — set it and local breaks; omit it and remote
gets no signed origin. There is also no guard or guidance that the domain is
remote-only.

Recommendations:
- Select the host profile / emit `_meta.ui.domain` **per connection**, from the
  negotiated connector type (local vs remote+verified), not a static field.
- `validate` should flag `Domain`/`HostProfile: "claude"` on a server whose only
  transport is local/stdio.
- Document that `ui.domain` is remote-connector-only.

(Our resolution: omit the domain entirely — correct for a local connector.)

## 3. Tool `_meta` emits only the nested `ui.resourceUri`; the official ext-apps SDK emits BOTH nested and flat `ui/resourceUri` — [ALIGN]

Diffing `tools/list` over raw MCP:

```jsonc
// pengui (official @modelcontextprotocol/ext-apps, renders in Claude Desktop)
"_meta": {
  "ui": { "resourceUri": "ui://deck-editor/index.html", "visibility": ["model"] },
  "ui/resourceUri": "ui://deck-editor/index.html"          // ← flat key ALSO present
}

// Dockyard .UI() (v1.5.0)
"_meta": { "ui": { "resourceUri": "ui://go-video-mcp/frameline" } }   // nested only
```

`apps.ToolMetaFor` emits only the nested form, and `runtime/apps/apps_test.go`
**asserts the flat `ui/resourceUri` key is absent** ("never the deprecated flat
key"). But the official SDK still emits both, deliberately, "for backward
compatibility" — i.e. because some hosts read the flat key.

Recommendation: emit **both** forms from `.UI()` / `apps.ToolMetaFor` (or make it
opt-in), and reconsider the test that forbids the flat key. (We worked around it
by registering the UI tools via `server.AddToolWithSchemas` with a hand-built
`_meta` carrying both keys — which means dropping back to the low-level API and
losing the builder ergonomics.)

Not proven strictly required (Claude Desktop did not render even with both keys;
the playground renders either way), but it is a real divergence from the
reference SDK.

## 4. Resource URI convention — [ALIGN]

The official SDK uses an html-style resource URI (`ui://deck-editor/index.html`).
Dockyard accepts any `ui://authority/path` and the scaffold produces
`ui://<server>/<app>` (no extension). We changed ours to
`ui://frameline/index.html` to match. Worth documenting / scaffolding the
`…/index.html` convention; some hosts may key off the html-style path.

## 5. Silent misconfig passes `validate` — [process]

Both §1 (the `.UI()` no-op) and §2 (domain-on-local) produced a **green
`dockyard validate`**. `validate` checks the manifest `ui:` ↔ `apps[]` wiring,
but not that the registered tool actually emits a usable `_meta.ui.resourceUri`,
nor that a declared domain is compatible with the server's transport.

Recommendation: add validate checks that (a) a UI-driving tool emits
`_meta.ui.resourceUri` on the wire, and (b) `App.Domain`/`HostProfile: "claude"`
isn't set on a local-only server. Same loud-failure principle the v1.5.0 `.UI()`
fix applied (a `.UI("typo")` now errors).

## 6. Web packages were checkout-only — [CONFIRMED, fixed in v1.5.0]

`@dockyard/bridge` / `@dockyard/ui` were workspace-only, so `web/` needed the
hidden `--dockyard-path` flag + a local checkout to `npm install`. Fixed in
**v1.5.0 (D-174)**: published as `dockyard-bridge` / `dockyard-ui` (^1.5.0).
Migration was a clean dep + import swap. ✓

## 8. No way to declare `data:` / `blob:` media in the App CSP — [CONFIRMED]

An App that displays server-produced media (here: playing a rendered `.mp4`
fetched via `read_media` as a `data:` URI) is blocked by the host's App-iframe
CSP:

> `Refused to load data:video/mp4;base64,… because it appears in neither the media-src directive nor the default-src directive of the Content Security Policy.`

`apps.CSP` only models **origin allowlists** (`Connect`/`Resource`/`Frame`/
`BaseURI` → connect-/img-/script-/style-/font-/media-/frame-/base-src as
*domains*). There is **no field to declare `data:` or `blob:` media-src intent**,
so a deny-by-default App cannot legitimately show inline `data:`/`blob:` media.
(Matches the `attach-a-ui-resource` skill's own note + the V2-backlog item
"Apps media-src / data: / blob: declaration".)

Workaround on our side: convert the `data:` URI to a `blob:` URL before setting
`<video src>` — `blob:` is permitted by the hosts we tested where `data:` is not
(it's the same scheme image previews already use) — and degrade to a "saved to
<path>" message via the `<video>` error handler if even `blob:` is blocked.

Recommendation: add a manifest knob to declare `data:`/`blob:` media-src (or a
media-intent flag), so an App can legitimately render inline media a host CSP
would otherwise block.

## 7. Positive findings (no change needed)

- **`dockyard-bridge` speaks the same `ui/` postMessage dialect as ext-apps** —
  `ui/initialize`, `ui/message`, `ui/request-display-mode`,
  `ui/resource-teardown`, `ui/update-model-context`, `tools/call` all match
  (RFC §7.2). The View-side handshake is spec-compliant; it renders in the
  playground.
- The single-file iife bundle (with the `stripModuleType` plugin) executes in a
  strict opaque-origin sandbox (proven in `dockyard inspect` and the playground).

---

## OPEN — Claude Desktop renders pengui (local) but not us (local), with matched wire

After §1–§4 our wire is structurally identical to pengui's (both `_meta.ui` key
forms + visibility, `…/index.html` URI, MIME `text/html;profile=mcp-app`, same
bridge dialect), and the App renders in a compliant playground — yet Claude
Desktop still shows text. pengui is *also* a local `mcp-remote` connector and
*does* render, so it is not a blanket local-connector block. Residual,
unconfirmed differences vs pengui still to investigate:

- Resource `name` is a human title in pengui ("Deck Editor") vs our id
  ("frameline"); pengui sets `description`, we set `title`.
- Opener tool `visibility`: pengui `["model"]` vs our `["model","app"]`.
- pengui ships `type="module"` + a second plain `<script>`; ours is one classic
  iife script (more sandbox-portable, but a structural difference).
- Possible Claude-Desktop tool-`_meta` caching per connector (a plain reopen may
  reuse the first connection's cached definition).

This looks Claude-Desktop-specific rather than a Dockyard defect, but the exact
trigger is not yet isolated.
