package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/hurtener/dockyard/runtime/tool"

	"github.com/hurtener/go-video-mcp/internal/contracts"
	"github.com/hurtener/go-video-mcp/internal/kernel"
	"github.com/hurtener/go-video-mcp/internal/slideshow"
)

// Defaults for the cinematic tool when the caller leaves a field unset.
const (
	defaultCanvas          = "1920x1080"
	defaultFPS             = 30
	defaultSecondsPerImage = 4.0
	defaultTransitionSecs  = 1.0
)

// CreateCinematicImageVideo is the flagship tool. It validates every path,
// resolves timing, compiles a slideshow.Spec into a single FFmpeg filtergraph,
// renders it, and reports the produced file plus the compiled graph.
func (h *Handlers) CreateCinematicImageVideo(ctx context.Context, in contracts.CreateCinematicImageVideoInput) (tool.Result[contracts.CreateCinematicImageVideoOutput], error) {
	fail := func(err error) (tool.Result[contracts.CreateCinematicImageVideoOutput], error) {
		return tool.Result[contracts.CreateCinematicImageVideoOutput]{}, fmt.Errorf("create_cinematic_image_video: %w", err)
	}

	if len(in.Images) == 0 {
		return fail(fmt.Errorf("at least one image is required"))
	}

	// Validate every input image and the output destination.
	images := make([]string, 0, len(in.Images))
	for _, img := range in.Images {
		v, err := h.K.ResolveArtifact(img)
		if err != nil {
			return fail(fmt.Errorf("image %q: %w", img, err))
		}
		images = append(images, v)
	}
	// Output path is optional: default a uniquely-named reel into the work dir
	// so a UI caller need not know server paths.
	outPath := strings.TrimSpace(in.OutputPath)
	if outPath == "" {
		if h.WorkDir == "" {
			return fail(fmt.Errorf("output_path is empty and no work directory is configured"))
		}
		outPath = uniquePath(h.WorkDir, "reel.mp4")
	}
	output, err := h.K.ValidatePath(outPath, kernel.ModeWrite)
	if err != nil {
		return fail(err)
	}

	// Canvas.
	canvas := in.Canvas
	if canvas == "" {
		canvas = defaultCanvas
	}
	w, hh, err := parseWxH(canvas)
	if err != nil {
		return fail(err)
	}
	w, hh = evenInt(w), evenInt(hh)

	fps := in.FPS
	if fps <= 0 {
		fps = defaultFPS
	}

	transition := in.TransitionStyle
	if transition == "" {
		transition = contracts.TransitionStyle(slideshow.TransitionFade)
	}
	transSecs := in.TransitionSeconds
	if transSecs <= 0 {
		transSecs = defaultTransitionSecs
	}
	blended := slideshow.TransitionStyle(transition) != slideshow.TransitionNone && len(images) > 1

	// Resolve per-image duration. TotalDuration wins when set: derive the
	// per-image on-screen time from the requested reel length, accounting for
	// the transition overlap (total = n*d - (n-1)*t  ⇒  d = (total + (n-1)*t)/n).
	perImage, err := resolveDuration(in, len(images), transSecs, blended)
	if err != nil {
		return fail(err)
	}
	// Keep the transition strictly shorter than the per-image duration.
	if blended && transSecs >= perImage {
		transSecs = perImage / 2
	}

	motion := in.MotionStyle
	if motion == "" {
		motion = contracts.MotionStyle(slideshow.MotionKenBurns)
	}
	grade := in.ColorGrade
	if grade == "" {
		grade = contracts.ColorGrade(slideshow.GradeNeutral)
	}

	// Optional audio bed.
	var audioPath string
	if in.BackgroundAudio != "" {
		audioPath, err = h.K.ResolveArtifact(in.BackgroundAudio)
		if err != nil {
			return fail(fmt.Errorf("background_audio: %w", err))
		}
	}

	spec := slideshow.Spec{
		Images:              images,
		Width:               w,
		Height:              hh,
		FPS:                 fps,
		SecondsPerImage:     perImage,
		Transition:          slideshow.TransitionStyle(transition),
		TransitionSeconds:   transSecs,
		Motion:              slideshow.MotionStyle(motion),
		Grade:               slideshow.ColorGrade(grade),
		AudioPath:           audioPath,
		AudioFadeInSeconds:  in.AudioFadeInSeconds,
		AudioFadeOutSeconds: in.AudioFadeOutSeconds,
		Output:              output,
	}
	plan, err := slideshow.Compile(spec)
	if err != nil {
		return fail(err)
	}

	res, err := h.K.RunPlan(ctx, plan, nil)
	if err != nil {
		return fail(err)
	}
	ro, err := h.finalize(ctx, res)
	if err != nil {
		return fail(err)
	}

	out := contracts.CreateCinematicImageVideoOutput{
		Render:          ro,
		ImageCount:      len(images),
		PerImageSeconds: perImage,
		FilterComplex:   plan.Graph.String(),
		Warnings:        plannedWarnings(in),
	}
	text := fmt.Sprintf("Rendered a %.1fs cinematic reel from %d images (%dx%d @ %dfps) → %s",
		ro.DurationSec, out.ImageCount, ro.Width, ro.Height, fps, ro.OutputPath)
	return tool.Result[contracts.CreateCinematicImageVideoOutput]{Text: text, Structured: out}, nil
}

// resolveDuration derives the per-image on-screen time from the request.
func resolveDuration(in contracts.CreateCinematicImageVideoInput, n int, transSecs float64, blended bool) (float64, error) {
	if in.TotalDuration > 0 {
		var d float64
		if blended {
			d = (in.TotalDuration + float64(n-1)*transSecs) / float64(n)
		} else {
			d = in.TotalDuration / float64(n)
		}
		if d <= 0 {
			return 0, fmt.Errorf("total_duration %.2fs is too short for %d images", in.TotalDuration, n)
		}
		return d, nil
	}
	if in.DurationPerImage > 0 {
		return in.DurationPerImage, nil
	}
	return defaultSecondsPerImage, nil
}

// plannedWarnings reports options accepted by the contract but not yet rendered
// in this layer, so the caller knows they were ignored rather than silently
// dropped.
func plannedWarnings(in contracts.CreateCinematicImageVideoInput) []string {
	var w []string
	if len(in.Captions) > 0 {
		w = append(w, fmt.Sprintf("captions (%d) are accepted but not yet burned in (planned: caption layer)", len(in.Captions)))
	}
	if in.Watermark != "" {
		w = append(w, "watermark is accepted but not yet rendered (planned)")
	}
	if in.BeatSync {
		w = append(w, "beat_sync is accepted but not yet applied (planned)")
	}
	if in.SafeArea {
		w = append(w, "safe_area is accepted but not yet enforced (planned)")
	}
	return w
}

// evenInt rounds up to the nearest even integer (libx264 needs even dims).
func evenInt(n int) int {
	if n%2 != 0 {
		return n + 1
	}
	return n
}
