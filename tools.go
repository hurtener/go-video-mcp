package main

import (
	"github.com/hurtener/dockyard/runtime/server"
	"github.com/hurtener/dockyard/runtime/tool"

	"github.com/hurtener/go-video-mcp/internal/contracts"
	"github.com/hurtener/go-video-mcp/internal/handlers"
)

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

	if err := tool.New[contracts.CreateCinematicImageVideoInput, contracts.CreateCinematicImageVideoOutput]("create_cinematic_image_video").
		Describe("Compile a sequence of images into a cinematic slideshow video: canvas preset, per-image Ken Burns motion, crossfade/wipe/slide transitions, a colour grade, and an optional faded music bed — all in one render. Returns the produced file and the compiled FFmpeg filtergraph.").
		Handler(h.CreateCinematicImageVideo).
		Register(srv); err != nil {
		return err
	}

	return nil
}
