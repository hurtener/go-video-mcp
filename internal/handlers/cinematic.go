package handlers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hurtener/dockyard/runtime/tool"

	"github.com/hurtener/go-video-mcp/internal/captions"
	"github.com/hurtener/go-video-mcp/internal/contracts"
	"github.com/hurtener/go-video-mcp/internal/kernel"
	"github.com/hurtener/go-video-mcp/internal/slideshow"
	"github.com/hurtener/go-video-mcp/internal/templates"
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

	// Resolve canvas/fps/motion/transition/grade/timing, applying any V6
	// template and the override precedence (explicit > template > default).
	set, err := resolveSettings(in, len(images))
	if err != nil {
		return fail(err)
	}
	w, hh := set.Width, set.Height
	fps := set.FPS
	transition := set.Transition
	transSecs := set.TransitionSeconds
	perImage := set.PerImage
	motion := set.Motion
	grade := set.Grade

	// Optional audio bed.
	var audioPath string
	if in.BackgroundAudio != "" {
		audioPath, err = h.K.ResolveArtifact(in.BackgroundAudio)
		if err != nil {
			return fail(fmt.Errorf("background_audio: %w", err))
		}
	}
	// V5 loudness normalize: defaults on; a *bool lets the caller turn it off.
	normalizeAudio := true
	if in.NormalizeAudio != nil {
		normalizeAudio = *in.NormalizeAudio
	}
	// V5 beat-sync applies only with a positive BPM (no onset detection).
	beatSync := in.BeatSync && in.BPM > 0
	blended := slideshow.TransitionStyle(transition) != slideshow.TransitionNone && len(images) > 1

	// V3 per-clip overrides: split the sparse Clips slice into the parallel
	// override slices the compiler consumes. Empty/zero entries fall back to the
	// global settings inside the compiler, so we pass them through verbatim.
	clipMotions, clipDurations, clipTransitions, clipFits := splitClips(in.Clips)

	// Captions (V4): rasterise each to a full-canvas overlay PNG (pure Go) and
	// hand the compiler the paths + time windows. captionWarn surfaces any
	// reason captions were skipped.
	capOverlays, captionWarn, err := h.buildCaptions(in.Captions, w, hh)
	if err != nil {
		return fail(err)
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
		Codec:               slideshow.Codec(in.Codec),
		Fit:                 slideshow.Fit(in.Fit),
		ClipMotions:         clipMotions,
		ClipFits:            clipFits,
		ClipDurations:       clipDurations,
		ClipTransitions:     clipTransitions,
		Grade:               slideshow.ColorGrade(grade),
		AudioPath:           audioPath,
		AudioFadeInSeconds:  in.AudioFadeInSeconds,
		AudioFadeOutSeconds: in.AudioFadeOutSeconds,
		NormalizeAudio:      audioPath != "" && normalizeAudio,
		BeatSync:            beatSync,
		BPM:                 in.BPM,
		Captions:            capOverlays,
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

	warnings := plannedWarnings(in)
	if captionWarn != "" {
		warnings = append(warnings, captionWarn)
	}
	if !blended && hasClipTransition(in.Clips) {
		warnings = append(warnings, "per-clip transitions are ignored on a hard-cut reel; set a global transition_style other than \"none\" to use them")
	}

	// Report the effective per-image time — beat-sync rounds it to whole beats.
	effectivePerImage := perImage
	if beatSync {
		effectivePerImage = slideshow.BeatSnappedDuration(perImage, transSecs, in.BPM, blended)
	}
	out := contracts.CreateCinematicImageVideoOutput{
		Render:          ro,
		ImageCount:      len(images),
		PerImageSeconds: effectivePerImage,
		FilterComplex:   plan.Graph.String(),
		Warnings:        warnings,
	}
	text := fmt.Sprintf("Rendered a %.1fs cinematic reel from %d images (%dx%d @ %dfps) → %s",
		ro.DurationSec, out.ImageCount, ro.Width, ro.Height, fps, ro.OutputPath)
	return tool.Result[contracts.CreateCinematicImageVideoOutput]{Text: text, Structured: out}, nil
}

// settings is the fully-resolved render configuration after applying the V6
// template and the override precedence. Pure output of resolveSettings.
type settings struct {
	Width, Height     int
	FPS               int
	Transition        contracts.TransitionStyle
	TransitionSeconds float64
	Motion            contracts.MotionStyle
	Grade             contracts.ColorGrade
	PerImage          float64
}

// resolveSettings applies the V6 template (if any) and the override precedence
// — explicit user field > template > hardcoded default — to produce the final
// render settings. It is pure (no I/O) so the precedence is unit-testable
// without invoking FFmpeg. n is the image count (affects timing + blending).
func resolveSettings(in contracts.CreateCinematicImageVideoInput, n int) (settings, error) {
	// A named preset contributes defaults for any field the caller left unset.
	preset, _ := templates.Lookup(string(in.Template))

	canvas := firstNonEmpty(in.Canvas, preset.Canvas, defaultCanvas)
	w, hh, err := parseWxH(canvas)
	if err != nil {
		return settings{}, err
	}
	w, hh = evenInt(w), evenInt(hh)

	fps := in.FPS
	if fps <= 0 {
		fps = preset.FPS
	}
	if fps <= 0 {
		fps = defaultFPS
	}

	transition := contracts.TransitionStyle(firstNonEmpty(
		string(in.TransitionStyle), preset.Transition, string(slideshow.TransitionFade)))

	transSecs := in.TransitionSeconds
	if transSecs <= 0 {
		transSecs = preset.TransitionSeconds
	}
	if transSecs <= 0 {
		transSecs = defaultTransitionSecs
	}
	blended := slideshow.TransitionStyle(transition) != slideshow.TransitionNone && n > 1

	perImage, err := resolveDuration(in, n, transSecs, blended, preset.SecondsPerImage)
	if err != nil {
		return settings{}, err
	}
	// Keep the transition strictly shorter than the per-image duration.
	if blended && transSecs >= perImage {
		transSecs = perImage / 2
	}

	motion := contracts.MotionStyle(firstNonEmpty(
		string(in.MotionStyle), preset.Motion, string(slideshow.MotionKenBurns)))
	grade := contracts.ColorGrade(firstNonEmpty(
		string(in.ColorGrade), preset.Grade, string(slideshow.GradeNeutral)))

	return settings{
		Width: w, Height: hh, FPS: fps,
		Transition: transition, TransitionSeconds: transSecs,
		Motion: motion, Grade: grade, PerImage: perImage,
	}, nil
}

// firstNonEmpty returns the first non-empty string of its arguments.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// resolveDuration derives the per-image on-screen time from the request.
// presetPerImage is the template's per-image default (0 when none); it is used
// only when the caller supplies neither TotalDuration nor DurationPerImage.
func resolveDuration(in contracts.CreateCinematicImageVideoInput, n int, transSecs float64, blended bool, presetPerImage float64) (float64, error) {
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
	if presetPerImage > 0 {
		return presetPerImage, nil
	}
	return defaultSecondsPerImage, nil
}

// plannedWarnings reports options accepted by the contract but not yet rendered
// in this layer, so the caller knows they were ignored rather than silently
// dropped.
func plannedWarnings(in contracts.CreateCinematicImageVideoInput) []string {
	var w []string
	if in.Watermark != "" {
		w = append(w, "watermark is accepted but not yet rendered (planned)")
	}
	if in.BeatSync && in.BPM <= 0 {
		w = append(w, "beat_sync needs a positive bpm to snap to; ignored (no onset detection)")
	}
	if in.SafeArea {
		w = append(w, "safe_area is accepted but not yet enforced (planned)")
	}
	return w
}

// splitClips turns the sparse per-clip override slice into the parallel slices
// the compiler consumes (motions, durations, transitions). Each is allocated
// only when at least one entry sets that field, so an all-empty Clips slice
// leaves every override nil and the render is identical to the global path.
func splitClips(clips []contracts.PerClip) (motions []slideshow.MotionStyle, durations []float64, transitions []slideshow.TransitionStyle, fits []slideshow.Fit) {
	if len(clips) == 0 {
		return nil, nil, nil, nil
	}
	var anyMotion, anyDur, anyTrans, anyFit bool
	for _, c := range clips {
		anyMotion = anyMotion || c.Motion != ""
		anyDur = anyDur || c.DurationSeconds > 0
		anyTrans = anyTrans || c.Transition != ""
		anyFit = anyFit || c.Fit != ""
	}
	if anyMotion {
		motions = make([]slideshow.MotionStyle, len(clips))
	}
	if anyDur {
		durations = make([]float64, len(clips))
	}
	if anyTrans {
		transitions = make([]slideshow.TransitionStyle, len(clips))
	}
	if anyFit {
		fits = make([]slideshow.Fit, len(clips))
	}
	for i, c := range clips {
		if anyMotion {
			motions[i] = slideshow.MotionStyle(c.Motion)
		}
		if anyDur {
			durations[i] = c.DurationSeconds
		}
		if anyTrans {
			transitions[i] = slideshow.TransitionStyle(c.Transition)
		}
		if anyFit {
			fits[i] = slideshow.Fit(c.Fit)
		}
	}
	return motions, durations, transitions, fits
}

// hasClipTransition reports whether any per-clip entry sets a transition.
func hasClipTransition(clips []contracts.PerClip) bool {
	for _, c := range clips {
		if c.Transition != "" {
			return true
		}
	}
	return false
}

// evenInt rounds up to the nearest even integer (libx264 needs even dims).
func evenInt(n int) int {
	if n%2 != 0 {
		return n + 1
	}
	return n
}

// buildCaptions rasterises each requested caption to a full-canvas overlay PNG
// in the work dir and returns the overlays for the compiler. The second return
// is a non-fatal warning (e.g. no font available) so captions degrade
// gracefully rather than failing the whole render.
func (h *Handlers) buildCaptions(caps []contracts.Caption, w, hh int) ([]slideshow.CaptionOverlay, string, error) {
	if len(caps) == 0 {
		return nil, "", nil
	}
	if h.WorkDir == "" {
		return nil, "captions requested but no work directory is configured; captions skipped", nil
	}
	fontPath, ok := resolveFont()
	if !ok {
		return nil, "captions requested but no usable font was found (set GO_VIDEO_MCP_FONT to a .ttf/.otf); captions skipped", nil
	}
	font, err := captions.LoadFont(fontPath)
	if err != nil {
		return nil, fmt.Sprintf("captions skipped — could not load font %q: %v", fontPath, err), nil
	}

	var overlays []slideshow.CaptionOverlay
	for i, c := range caps {
		if strings.TrimSpace(c.Text) == "" || c.EndSeconds <= c.StartSeconds {
			continue // skip empty / zero-length captions
		}
		png, rerr := captions.Render(font, captions.Spec{
			Text:     c.Text,
			Position: capPosition(c.Position),
			CanvasW:  w,
			CanvasH:  hh,
		})
		if rerr != nil {
			continue
		}
		dst := uniquePath(h.WorkDir, fmt.Sprintf("caption-%d.png", i))
		validated, verr := h.K.ValidatePath(dst, kernel.ModeWrite)
		if verr != nil {
			return nil, "", verr
		}
		if werr := os.WriteFile(validated, png, 0o644); werr != nil {
			return nil, "", fmt.Errorf("write caption overlay: %w", werr)
		}
		overlays = append(overlays, slideshow.CaptionOverlay{
			Path:         validated,
			StartSeconds: c.StartSeconds,
			EndSeconds:   c.EndSeconds,
		})
	}
	return overlays, "", nil
}

// capPosition maps a contract position string to a captions.Position.
func capPosition(p string) captions.Position {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "top":
		return captions.PositionTop
	case "center", "centre", "middle":
		return captions.PositionCenter
	default:
		return captions.PositionLowerThird
	}
}

// fontCandidates is the default allowlist of system fonts, tried in order. The
// server never accepts an arbitrary user font path from a tool call; an
// operator widens the allowlist via GO_VIDEO_MCP_FONT.
var fontCandidates = []string{
	"/System/Library/Fonts/Supplemental/Arial.ttf",
	"/System/Library/Fonts/Supplemental/Helvetica.ttf",
	"/Library/Fonts/Arial.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
}

// resolveFont returns the first usable font path: GO_VIDEO_MCP_FONT if set and
// present, otherwise the first existing default candidate.
func resolveFont() (string, bool) {
	if p := strings.TrimSpace(os.Getenv("GO_VIDEO_MCP_FONT")); p != "" {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p, true
		}
	}
	for _, p := range fontCandidates {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p, true
		}
	}
	return "", false
}
