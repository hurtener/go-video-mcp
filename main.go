// Command go-video-mcp is an MCP server for video editing, built with Dockyard.
//
// It exposes a curated set of tools over a single FFmpeg kernel — probe,
// convert, trim, extract audio, and the flagship create_cinematic_image_video,
// which compiles an image sequence into a cinematic slideshow filtergraph. The
// transport is chosen by the DOCKYARD_TRANSPORT environment variable — "stdio"
// (the default) or "http". GO_VIDEO_MCP_ROOTS (os-path-list-separated) confines
// every file read and write; it defaults to the working directory.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/hurtener/dockyard/runtime/apps"
	"github.com/hurtener/dockyard/runtime/obs"
	"github.com/hurtener/dockyard/runtime/server"

	"github.com/hurtener/go-video-mcp/internal/handlers"
	"github.com/hurtener/go-video-mcp/internal/kernel"
)

// uiBundle holds the Vite-built single-file Frameline Studio bundle. `dockyard
// build` runs the web build before `go build`, so web/dist/index.html exists at
// embed time. The `all:` prefix preserves files Vite emits with leading `_`/`.`.
//
//go:embed all:web/dist
var uiBundle embed.FS

// httpAddr is the address the HTTP transport listens on when
// DOCKYARD_TRANSPORT=http. DOCKYARD_HTTP_ADDR overrides it.
const httpAddr = "127.0.0.1:8080"

// The Frameline Studio MCP App — the inline UI for create_cinematic_image_video.
const (
	appURI  = "ui://go-video-mcp/frameline"
	appName = "frameline"
)

func main() {
	// A text slog handler — readable local logs (Dockyard convention).
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Serve until the process is interrupted (Ctrl-C) or the host closes the
	// transport.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// The obs/v1 SSE sink is the out-of-band, localhost-bound event stream the
	// inspector subscribes to; mounting it lets `dockyard inspect` relay live
	// events. Stdio servers keep it strictly out-of-band — it never touches the
	// MCP pipe.
	obsSink, err := obs.NewSSESink("")
	if err != nil {
		logger.Error("create obs sink", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = obsSink.Close() }()

	srv, err := server.New(server.Info{
		Name:    "go-video-mcp",
		Title:   "Go Video Mcp",
		Version: "0.1.0",
	}, &server.Options{Logger: logger, Obs: obsSink})
	if err != nil {
		logger.Error("create server", slog.String("error", err.Error()))
		os.Exit(1)
	}

	k, err := kernel.New(kernel.Config{AllowedRoots: allowedRoots()})
	if err != nil {
		logger.Error("create ffmpeg kernel", slog.String("error", err.Error()))
		os.Exit(1)
	}

	workDir, err := ensureWorkDir(k)
	if err != nil {
		logger.Error("prepare work directory", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := registerApp(srv); err != nil {
		logger.Error("register app", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := registerTools(srv, handlers.New(k, workDir)); err != nil {
		logger.Error("register tools", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := serve(ctx, srv, obsSink, logger); err != nil {
		logger.Error("serve", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// registerApp installs the embedded single-file Frameline Studio bundle as the
// ui://go-video-mcp/frameline MCP App resource (inline, deny-by-default CSP).
// The tools declare .UI(appName) so their results render in this App in the
// host's chat surface.
//
// Domain + HostProfile "claude" make the resources/read response carry
// _meta.ui.domain — the dedicated, signed <hash>.claudemcpcontent.com origin
// Claude renders an App iframe from (RFC §7.5). Without it, Claude recognises
// the tool's _meta.ui link but has no origin to load the iframe into, so the
// App never paints (it falls back to text). The signed origin is derived from
// the public server URL; GO_VIDEO_MCP_PUBLIC_URL overrides it for the bridged
// HTTP setup.
func registerApp(srv *server.Server) error {
	html, err := fs.ReadFile(uiBundle, "web/dist/index.html")
	if err != nil {
		return err
	}
	prefersBorder := true
	return apps.Register(srv, apps.App{
		URI:           appURI,
		Name:          appName,
		Title:         "Frameline Studio",
		HTML:          html,
		Domain:        appName, // host-agnostic label → signed dedicated origin
		HostProfile:   "claude",
		ServerURL:     publicURL(),
		PrefersBorder: &prefersBorder,
	})
}

// publicURL is the externally-reachable MCP endpoint, used to derive the App's
// signed Claude content origin. It is GO_VIDEO_MCP_PUBLIC_URL when set, else the
// HTTP endpoint this server listens on (the address mcp-remote connects to).
func publicURL() string {
	if v := strings.TrimSpace(os.Getenv("GO_VIDEO_MCP_PUBLIC_URL")); v != "" {
		return v
	}
	addr := httpAddr
	if override := strings.TrimSpace(os.Getenv("DOCKYARD_HTTP_ADDR")); override != "" {
		addr = override
	}
	return "http://" + addr + "/mcp"
}

// allowedRoots reads the filesystem confinement policy from GO_VIDEO_MCP_ROOTS
// (an OS-path-list-separated set of directories). When unset, the kernel
// defaults to the working directory — reads and writes outside it are rejected
// by the kernel's ValidatePath.
func allowedRoots() []string {
	v := strings.TrimSpace(os.Getenv("GO_VIDEO_MCP_ROOTS"))
	if v == "" {
		return nil // kernel.New falls back to the working directory
	}
	var roots []string
	for _, r := range strings.Split(v, string(os.PathListSeparator)) {
		if r = strings.TrimSpace(r); r != "" {
			roots = append(roots, filepath.Clean(r))
		}
	}
	return roots
}

// ensureWorkDir returns (creating if needed) the directory where ingest_media
// persists uploaded files. It is GO_VIDEO_MCP_WORK_DIR when set, otherwise a
// "frameline-work" folder inside the first allowed root — so it is always
// inside the kernel's confinement and writable by the ingest tool.
func ensureWorkDir(k *kernel.Kernel) (string, error) {
	dir := strings.TrimSpace(os.Getenv("GO_VIDEO_MCP_WORK_DIR"))
	if dir == "" {
		roots := k.Roots()
		if len(roots) == 0 {
			return "", errors.New("no allowed roots configured")
		}
		dir = filepath.Join(roots[0], "frameline-work")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// serve brings up the transport named by DOCKYARD_TRANSPORT. An unset or
// "stdio" value serves stdio; "http" serves the streamable-HTTP transport. An
// unrecognised value is a clean, explained failure rather than a silent
// fallback.
func serve(ctx context.Context, srv *server.Server, obsSink *obs.SSESink, logger *slog.Logger) error {
	switch transport := os.Getenv("DOCKYARD_TRANSPORT"); transport {
	case "", "stdio":
		return srv.ServeStdio(ctx)
	case "http":
		return serveHTTP(ctx, srv, obsSink, logger)
	default:
		return errors.New("unsupported DOCKYARD_TRANSPORT " + transport + " (want \"stdio\" or \"http\")")
	}
}

// serveHTTP serves the streamable-HTTP transport. The HTTP security posture is
// the runtime's secure default — DNS-rebinding and cross-origin protection both
// on (runtime/server.DefaultHTTPSecurity). The listen address is httpAddr,
// overridable with DOCKYARD_HTTP_ADDR.
func serveHTTP(ctx context.Context, srv *server.Server, obsSink *obs.SSESink, logger *slog.Logger) error {
	handler, err := srv.HTTPHandler(nil)
	if err != nil {
		return err
	}
	// Mount the obs/v1 SSE stream on the same listener so the inspector can
	// subscribe with just --url <server>.
	mux := http.NewServeMux()
	mux.Handle("/obs/v1/stream", obsSink.Handler())
	mux.Handle("/", handler)
	addr := httpAddr
	if override := os.Getenv("DOCKYARD_HTTP_ADDR"); override != "" {
		addr = override
	}
	httpSrv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = httpSrv.Close()
	}()
	logger.Info("serving streamable-HTTP transport", slog.String("addr", addr))
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
