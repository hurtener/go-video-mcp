// Package handlers implements go-video-mcp's tool handlers. Each handler is a
// thin shell over the ffmpeg kernel: it validates the user-supplied paths,
// builds a typed kernel.Plan (or compiles a slideshow spec), runs it, and maps
// the result back to a contract. No handler shells out or builds a shell string
// itself — that lives in the kernel (CLAUDE.md §5).
package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hurtener/dockyard/runtime/tool"

	"github.com/hurtener/go-video-mcp/internal/contracts"
	"github.com/hurtener/go-video-mcp/internal/kernel"
)

// Handlers carries the shared kernel every tool handler runs against, plus the
// work directory where uploaded media is persisted (ingest_media writes here;
// it must be inside an allowed root).
type Handlers struct {
	K       *kernel.Kernel
	WorkDir string
}

// New constructs the handler set bound to a kernel and an ingest work dir.
func New(k *kernel.Kernel, workDir string) *Handlers {
	return &Handlers{K: k, WorkDir: workDir}
}

// ProbeMedia inspects a media file and returns typed facts.
func (h *Handlers) ProbeMedia(ctx context.Context, in contracts.ProbeMediaInput) (tool.Result[contracts.ProbeMediaOutput], error) {
	path, err := h.K.ValidatePath(in.Path, kernel.ModeRead)
	if err != nil {
		return tool.Result[contracts.ProbeMediaOutput]{}, fmt.Errorf("probe_media: %w", err)
	}
	mi, err := h.K.Probe(ctx, path)
	if err != nil {
		return tool.Result[contracts.ProbeMediaOutput]{}, fmt.Errorf("probe_media: %w", err)
	}
	out := contracts.ProbeMediaOutput{
		Path:        mi.Path,
		FormatName:  mi.FormatName,
		DurationSec: mi.DurationSec,
		SizeBytes:   mi.SizeBytes,
		BitRate:     mi.BitRate,
		HasVideo:    mi.HasVideo,
		HasAudio:    mi.HasAudio,
		Width:       mi.Width,
		Height:      mi.Height,
		FPS:         mi.FPS,
		VideoCodec:  mi.VideoCodec,
		AudioCodec:  mi.AudioCodec,
		Streams:     mi.Streams,
	}
	text := fmt.Sprintf("%s — %s, %.2fs", filepath.Base(out.Path), out.FormatName, out.DurationSec)
	if out.HasVideo {
		text += fmt.Sprintf(", %dx%d @ %.2gfps (%s)", out.Width, out.Height, out.FPS, out.VideoCodec)
	}
	if out.HasAudio {
		text += fmt.Sprintf(", audio %s", out.AudioCodec)
	}
	return tool.Result[contracts.ProbeMediaOutput]{Text: text, Structured: out}, nil
}

// ConvertVideo re-encodes a video to a new container / codec / size.
func (h *Handlers) ConvertVideo(ctx context.Context, in contracts.ConvertVideoInput) (tool.Result[contracts.RenderOutput], error) {
	src, dst, err := h.validateIO(in.InputPath, in.OutputPath)
	if err != nil {
		return tool.Result[contracts.RenderOutput]{}, fmt.Errorf("convert_video: %w", err)
	}

	out := []string{}
	if in.Scale != "" {
		w, hh, perr := parseWxH(in.Scale)
		if perr != nil {
			return tool.Result[contracts.RenderOutput]{}, fmt.Errorf("convert_video: %w", perr)
		}
		out = append(out, "-vf", fmt.Sprintf("scale=%d:%d", w, hh))
	}
	if in.FPS > 0 {
		out = append(out, "-r", strconv.Itoa(in.FPS))
	}
	if in.VideoCodec != "" {
		out = append(out, "-c:v", in.VideoCodec)
	}
	if in.AudioCodec != "" {
		out = append(out, "-c:a", in.AudioCodec)
	}
	if in.CRF > 0 {
		out = append(out, "-crf", strconv.Itoa(in.CRF))
	}
	if in.Preset != "" {
		out = append(out, "-preset", in.Preset)
	}

	plan := kernel.Plan{
		Inputs:    []kernel.Input{{Path: src}},
		Out:       out,
		Output:    dst,
		Overwrite: true,
	}
	return h.runRender(ctx, "convert_video", plan)
}

// TrimVideo cuts a [start, end) range out of a video.
func (h *Handlers) TrimVideo(ctx context.Context, in contracts.TrimVideoInput) (tool.Result[contracts.RenderOutput], error) {
	src, dst, err := h.validateIO(in.InputPath, in.OutputPath)
	if err != nil {
		return tool.Result[contracts.RenderOutput]{}, fmt.Errorf("trim_video: %w", err)
	}
	if in.EndSeconds <= in.StartSeconds {
		return tool.Result[contracts.RenderOutput]{}, fmt.Errorf("trim_video: end_seconds (%.2f) must exceed start_seconds (%.2f)", in.EndSeconds, in.StartSeconds)
	}

	input := kernel.Input{Path: src}
	if in.StartSeconds > 0 {
		// Fast input-side seek; FFmpeg places -ss before -i.
		input.Pre = []string{"-ss", ftoa(in.StartSeconds)}
	}
	out := []string{"-c", "copy"}
	if in.ReEncode {
		out = []string{"-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p", "-c:a", "aac"}
	}
	plan := kernel.Plan{
		Inputs:    []kernel.Input{input},
		Out:       out,
		Duration:  in.EndSeconds - in.StartSeconds,
		Output:    dst,
		Overwrite: true,
	}
	return h.runRender(ctx, "trim_video", plan)
}

// ExtractAudio pulls the audio track out of a media file.
func (h *Handlers) ExtractAudio(ctx context.Context, in contracts.ExtractAudioInput) (tool.Result[contracts.ExtractAudioOutput], error) {
	src, dst, err := h.validateIO(in.InputPath, in.OutputPath)
	if err != nil {
		return tool.Result[contracts.ExtractAudioOutput]{}, fmt.Errorf("extract_audio: %w", err)
	}

	codec, lossless := audioCodecFor(dst)
	out := []string{"-vn", "-c:a", codec}
	if !lossless {
		bitrate := in.Bitrate
		if bitrate == "" {
			bitrate = "192k"
		}
		out = append(out, "-b:a", bitrate)
	}
	plan := kernel.Plan{
		Inputs:    []kernel.Input{{Path: src}},
		Out:       out,
		Output:    dst,
		Overwrite: true,
	}
	res, err := h.K.RunPlan(ctx, plan, nil)
	if err != nil {
		return tool.Result[contracts.ExtractAudioOutput]{}, fmt.Errorf("extract_audio: %w", err)
	}
	mi, err := h.K.Probe(ctx, res.Output)
	if err != nil {
		return tool.Result[contracts.ExtractAudioOutput]{}, fmt.Errorf("extract_audio: probe output: %w", err)
	}
	out2 := contracts.ExtractAudioOutput{
		OutputPath:  res.Output,
		JobID:       res.JobID,
		DurationSec: mi.DurationSec,
		SizeBytes:   mi.SizeBytes,
	}
	return tool.Result[contracts.ExtractAudioOutput]{
		Text:       fmt.Sprintf("Extracted %s (%.2fs) to %s", codec, out2.DurationSec, filepath.Base(out2.OutputPath)),
		Structured: out2,
	}, nil
}

// --- shared helpers --------------------------------------------------------

// validateIO validates a read input and a write output in one step.
func (h *Handlers) validateIO(in, out string) (src, dst string, err error) {
	src, err = h.K.ValidatePath(in, kernel.ModeRead)
	if err != nil {
		return "", "", err
	}
	dst, err = h.K.ValidatePath(out, kernel.ModeWrite)
	if err != nil {
		return "", "", err
	}
	return src, dst, nil
}

// runRender runs a plan and maps the result to a RenderOutput, probing the
// produced file for its dimensions/duration/size.
func (h *Handlers) runRender(ctx context.Context, tool0 string, plan kernel.Plan) (tool.Result[contracts.RenderOutput], error) {
	res, err := h.K.RunPlan(ctx, plan, nil)
	if err != nil {
		return tool.Result[contracts.RenderOutput]{}, fmt.Errorf("%s: %w", tool0, err)
	}
	ro, err := h.finalize(ctx, res)
	if err != nil {
		return tool.Result[contracts.RenderOutput]{}, fmt.Errorf("%s: %w", tool0, err)
	}
	return tool.Result[contracts.RenderOutput]{
		Text:       fmt.Sprintf("Wrote %s (%.2fs, %dx%d)", filepath.Base(ro.OutputPath), ro.DurationSec, ro.Width, ro.Height),
		Structured: ro,
	}, nil
}

// finalize probes a rendered file and builds the common RenderOutput.
func (h *Handlers) finalize(ctx context.Context, res kernel.RunResult) (contracts.RenderOutput, error) {
	mi, err := h.K.Probe(ctx, res.Output)
	if err != nil {
		return contracts.RenderOutput{}, fmt.Errorf("probe output: %w", err)
	}
	return contracts.RenderOutput{
		OutputPath:  res.Output,
		JobID:       res.JobID,
		DurationSec: mi.DurationSec,
		Width:       mi.Width,
		Height:      mi.Height,
		SizeBytes:   mi.SizeBytes,
		Command:     res.Command,
	}, nil
}

// audioCodecFor picks an audio codec from the output extension and reports
// whether it is lossless (so bitrate is omitted).
func audioCodecFor(path string) (codec string, lossless bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav":
		return "pcm_s16le", true
	case ".flac":
		return "flac", true
	case ".m4a", ".aac", ".mp4":
		return "aac", false
	case ".opus":
		return "libopus", false
	default: // .mp3 and anything else
		return "libmp3lame", false
	}
}

// parseWxH parses a "WIDTHxHEIGHT" string into even, positive dimensions.
func parseWxH(s string) (w, hh int, err error) {
	a, b, ok := strings.Cut(strings.ToLower(strings.TrimSpace(s)), "x")
	if !ok {
		return 0, 0, fmt.Errorf("invalid size %q (want WxH, e.g. 1920x1080)", s)
	}
	w, err = strconv.Atoi(strings.TrimSpace(a))
	if err != nil || w <= 0 {
		return 0, 0, fmt.Errorf("invalid width in %q", s)
	}
	hh, err = strconv.Atoi(strings.TrimSpace(b))
	if err != nil || hh <= 0 {
		return 0, 0, fmt.Errorf("invalid height in %q", s)
	}
	return w, hh, nil
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
