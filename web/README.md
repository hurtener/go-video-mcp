# Frameline Studio — web

The Svelte MCP App for `create_cinematic_image_video`. Built by Vite into a
single-file `dist/index.html` and embedded into the Go binary via
`//go:embed all:web/dist`.

## Building

`@dockyard/bridge` and `@dockyard/ui` are not yet published to npm — they are
workspace packages inside the [Dockyard](https://github.com/hurtener/dockyard)
repo. `package.json` references them with **relative** `file:` specs:

```
"@dockyard/bridge": "file:../../dockyard/web/bridge",
"@dockyard/ui":     "file:../../dockyard/web/ui",
```

So this expects **`dockyard` checked out as a sibling** of `go-video-mcp`:

```
parent/
├── go-video-mcp/
└── dockyard/
```

If your layout differs, adjust the two `file:` paths.

```sh
npm install
npm run build      # → dist/index.html (single-file iife bundle)
```

`dockyard build` runs `npm run build` automatically before `go build`. The Go
module itself depends on the **published** `github.com/hurtener/dockyard` — only
this web bundle needs the local checkout.

## Notes

- The bundle must be a **classic iife** (see `vite.config.ts`): the host renders
  the App in a sandboxed iframe without `allow-same-origin`, where module
  scripts won't execute.
- CSP is deny-by-default; everything (wavesurfer, icons) is bundled — no
  external origins.
