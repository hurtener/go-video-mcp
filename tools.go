package main

import (
	"context"

	"github.com/hurtener/dockyard/runtime/server"
	"github.com/hurtener/dockyard/runtime/tool"

	"github.com/hurtener/go-video-mcp/internal/contracts"
	"github.com/hurtener/go-video-mcp/internal/handlers"
)

// uiToolMeta is the tool-definition _meta that links a tool to the Frameline
// App. It carries BOTH the nested `_meta.ui.resourceUri` AND the flat
// `_meta["ui/resourceUri"]` key.
//
// Why both: Dockyard's .UI() emits only the nested form (it treats the flat key
// as deprecated). But Claude Desktop — like the official @modelcontextprotocol/
// ext-apps SDK that renders there — reads the flat `ui/resourceUri` key to
// render the App. A nested-only tool is recognised but never painted. Emitting
// both matches the working reference exactly. (Dockyard issue: .UI() should emit
// both for host compatibility.)
func uiToolMeta() map[string]any {
	return map[string]any{
		"ui": map[string]any{
			"resourceUri": appURI,
			"visibility":  []string{"model", "app"},
		},
		"ui/resourceUri": appURI,
	}
}

// registerUITool registers a UI-driving tool through AddToolWithSchemas so the
// tool definition carries uiToolMeta (both _meta.ui key forms) — which the
// builder's .UI() cannot emit today. It reuses the builder only to derive the
// generated input/output schemas, and preserves the handler's Text + Structured.
func registerUITool[In, Out any](
	srv *server.Server,
	name, desc string,
	h func(context.Context, In) (tool.Result[Out], error),
) error {
	in, out, err := tool.New[In, Out](name).Describe(desc).Schemas()
	if err != nil {
		return err
	}
	return server.AddToolWithSchemas[In, Out](srv,
		server.ToolDef{Name: name, Description: desc, Meta: uiToolMeta()},
		in, out,
		func(ctx context.Context, arg In) (server.ToolOutput[Out], error) {
			res, herr := h(ctx, arg)
			return server.ToolOutput[Out]{Text: res.Text, Structured: res.Structured}, herr
		},
	)
}

// registerTools declares and registers every tool this server exposes. The
// handlers all run against one shared ffmpeg kernel, so the path-safety and
// process-execution rules live in exactly one place.
func registerTools(srv *server.Server, h *handlers.Handlers) error {
	if err := tool.New[contracts.ProbeMediaInput, contracts.ProbeMediaOutput]("probe_media").
		Describe("Inspect a media file with ffprobe and return typed facts: format, duration, dimensions, frame rate, codecs, and stream presence.").
		Handler(h.ProbeMedia).
		Register(srv); err != nil {
		return err
	}

	if err := tool.New[contracts.ConvertVideoInput, contracts.RenderOutput]("convert_video").
		Describe("Re-encode a video to a new container, codec, size, or frame rate. The output extension selects the container.").
		Handler(h.ConvertVideo).
		Register(srv); err != nil {
		return err
	}

	if err := tool.New[contracts.TrimVideoInput, contracts.RenderOutput]("trim_video").
		Describe("Cut a [start, end) time range out of a video. Fast stream-copy by default; set re_encode for a frame-accurate cut.").
		Handler(h.TrimVideo).
		Register(srv); err != nil {
		return err
	}

	if err := tool.New[contracts.ExtractAudioInput, contracts.ExtractAudioOutput]("extract_audio").
		Describe("Extract the audio track from a media file. The output extension selects the audio format (mp3, m4a, wav, flac).").
		Handler(h.ExtractAudio).
		Register(srv); err != nil {
		return err
	}

	if err := tool.New[contracts.ListMediaInput, contracts.ListMediaOutput]("list_media").
		Describe("Browse media files (images, audio, video) under the server's allowed roots, optionally filtered by directory or kind.").
		Handler(h.ListMedia).
		Register(srv); err != nil {
		return err
	}

	if err := tool.New[contracts.IngestMediaInput, contracts.IngestMediaOutput]("ingest_media").
		Describe("Persist a file uploaded through the UI (base64 bytes) into the server work directory and return a path other tools can read.").
		Handler(h.IngestMedia).
		Register(srv); err != nil {
		return err
	}

	if err := tool.New[contracts.ReadMediaInput, contracts.ReadMediaOutput]("read_media").
		Describe("Read a media file under the allowed roots as a size-capped data URI, so a sandboxed UI can play or display it.").
		Handler(h.ReadMedia).
		Register(srv); err != nil {
		return err
	}

	// UI-driving tools go through registerUITool so their _meta carries both
	// `_meta.ui.resourceUri` forms Claude Desktop needs to render the App.
	if err := registerUITool[contracts.OpenStudioInput, contracts.OpenStudioOutput](srv,
		"open_studio",
		"Open the Frameline Studio composer card — the interactive surface to arrange stills, set the look, and render a cinematic reel. Call with no arguments to open an empty composer the user can drop stills into.",
		h.OpenStudio); err != nil {
		return err
	}

	if err := registerUITool[contracts.OpenMediaUploaderInput, contracts.OpenMediaUploaderOutput](srv,
		"open_media_uploader",
		"Open the Media Uploader card so the user can drag in photos and music. Use this when the user wants to upload or add media. The card ingests dropped files onto the server and shows their paths, ready to compose into a reel.",
		h.OpenMediaUploader); err != nil {
		return err
	}

	if err := registerUITool[contracts.CreateCinematicImageVideoInput, contracts.CreateCinematicImageVideoOutput](srv,
		"create_cinematic_image_video",
		"Compile a sequence of images into a cinematic slideshow video: canvas preset, per-image Ken Burns motion, crossfade/wipe/slide transitions, a colour grade, and an optional faded music bed — all in one render. Returns the produced file and the compiled FFmpeg filtergraph.",
		h.CreateCinematicImageVideo); err != nil {
		return err
	}

	return nil
}
