// Package slideshow compiles a structured slideshow specification into a single
// FFmpeg filter_complex graph wrapped in a kernel.Plan. This is the engineering
// core of go-video-mcp's cinematic tool: the Spec is small and declarative; the
// compiler turns it into labeled streams, per-image motion chains, xfade joins,
// a colour grade, and an audio bed — the composed graph FFmpeg needs.
//
// The compiler is pure: Spec in, Plan out, no I/O. Paths on the Spec are assumed
// already validated by the caller (the kernel's ValidatePath). Purity is what
// lets the graph be golden-tested without ever invoking FFmpeg.
package slideshow

import (
	"errors"
	"fmt"
	"math"

	"github.com/hurtener/go-video-mcp/internal/kernel"
)

// Spec is the declarative description of a slideshow. The handler builds it from
// the tool contract; the compiler turns it into a Plan.
type Spec struct {
	// Images are the ordered, already-validated source image paths (≥1).
	Images []string
	// Width, Height are the output canvas dimensions (even, > 0).
	Width, Height int
	// FPS is the output frame rate.
	FPS int
	// SecondsPerImage is the on-screen time per image (must exceed
	// TransitionSeconds when a blended transition is used).
	SecondsPerImage float64
	// Transition selects the join style between images.
	Transition TransitionStyle
	// TransitionSeconds is the crossfade duration (ignored for TransitionNone).
	TransitionSeconds float64
	// Motion selects the default per-image camera motion.
	Motion MotionStyle
	// ClipMotions, when non-empty, overrides Motion per image by index. An empty
	// entry (or a slice shorter than Images) falls back to Motion. (V3)
	ClipMotions []MotionStyle
	// Fit selects how each image is placed on the canvas (cover / contain /
	// blur). Empty defaults to cover.
	Fit Fit
	// ClipFits, when non-empty, overrides Fit per image by index. An empty entry
	// (or a short slice) falls back to Fit.
	ClipFits []Fit
	// ClipDurations, when non-empty, overrides SecondsPerImage per image by
	// index. A zero entry (or a short slice) falls back to SecondsPerImage. (V3)
	ClipDurations []float64
	// ClipTransitions, when non-empty, overrides the transition style per JOIN
	// by index (join k is between image k and k+1). An empty/"none" entry (or a
	// short slice) falls back to Transition. Applies only when the reel is
	// blended (Transition != none); per-join hard cuts are not yet supported. (V3)
	ClipTransitions []TransitionStyle
	// Grade selects the final colour look.
	Grade ColorGrade
	// Codec selects the output video codec (default h264 when empty).
	Codec Codec
	// AudioPath, when non-empty, is an already-validated background audio file.
	AudioPath string
	// AudioFadeInSeconds / AudioFadeOutSeconds add fades to the audio bed.
	AudioFadeInSeconds  float64
	AudioFadeOutSeconds float64
	// NormalizeAudio loudness-normalises the bed via a single-pass loudnorm
	// (EBU R128, ~-16 LUFS). Single-pass, not two-pass — good for a music bed.
	NormalizeAudio bool
	// BeatSync, with BPM > 0, snaps the per-image advance to a whole number of
	// beats so every transition (or hard cut) lands on the beat.
	BeatSync bool
	// BPM is the music tempo used for beat snapping (ignored unless BeatSync).
	BPM float64
	// Captions are pre-rendered, full-canvas overlay PNGs with time windows.
	// Each is overlaid at 0:0 gated by `between(t, start, end)`. The handler
	// rasterises the text (pure Go) and writes the PNGs; the compiler only
	// composes them — keeping it pure and golden-testable.
	Captions []CaptionOverlay
	// Output is the destination file path (already validated for writing).
	Output string
}

// CaptionOverlay is one rendered caption: a full-canvas PNG and the time window
// it is shown for.
type CaptionOverlay struct {
	// Path is the already-validated overlay PNG (full canvas, transparent).
	Path string
	// StartSeconds / EndSeconds bound when the overlay is visible.
	StartSeconds, EndSeconds float64
}

// ErrNoImages is returned when a Spec carries no images.
var ErrNoImages = errors.New("slideshow: at least one image is required")

// Compile turns a Spec into a ready-to-run kernel.Plan.
func Compile(s Spec) (kernel.Plan, error) {
	n := len(s.Images)
	if n == 0 {
		return kernel.Plan{}, ErrNoImages
	}
	if s.Width <= 0 || s.Height <= 0 {
		return kernel.Plan{}, fmt.Errorf("slideshow: invalid canvas %dx%d", s.Width, s.Height)
	}
	if s.FPS <= 0 {
		return kernel.Plan{}, fmt.Errorf("slideshow: invalid fps %d", s.FPS)
	}
	if s.SecondsPerImage <= 0 {
		return kernel.Plan{}, fmt.Errorf("slideshow: invalid seconds-per-image %v", s.SecondsPerImage)
	}

	trans := s.TransitionSeconds
	blended := s.Transition != TransitionNone && n > 1
	if blended && trans <= 0 {
		trans = 1.0
	}

	// V3 per-clip resolution: each image's on-screen duration and motion is its
	// override (when set) else the global; V5 beat-sync then snaps each clip's
	// advance to whole beats (a no-op when off). Variable durations are why the
	// merge below uses cumulative offsets rather than a single uniform stride.
	durs := make([]float64, n)
	motions := make([]MotionStyle, n)
	fits := make([]Fit, n)
	for i := 0; i < n; i++ {
		d := s.SecondsPerImage
		if i < len(s.ClipDurations) && s.ClipDurations[i] > 0 {
			d = s.ClipDurations[i]
		}
		if s.BeatSync && s.BPM > 0 {
			d = BeatSnappedDuration(d, trans, s.BPM, blended)
		}
		durs[i] = d
		m := s.Motion
		if i < len(s.ClipMotions) && s.ClipMotions[i] != "" {
			m = s.ClipMotions[i]
		}
		motions[i] = m
		f := s.Fit
		if f == "" {
			f = FitCover
		}
		if i < len(s.ClipFits) && s.ClipFits[i] != "" {
			f = s.ClipFits[i]
		}
		fits[i] = f
	}
	if blended {
		for i, d := range durs {
			if trans >= d {
				return kernel.Plan{}, fmt.Errorf("slideshow: transition (%.2fs) must be shorter than image %d's duration (%.2fs)", trans, i, d)
			}
		}
	}

	plan := kernel.Plan{Overwrite: true, Output: s.Output}
	graph := &kernel.FilterGraph{}

	// One looped-image input per photo, each held for its resolved duration.
	for i, img := range s.Images {
		plan.Inputs = append(plan.Inputs, kernel.Input{Path: img, Loop: true, Duration: durs[i]})
	}

	// Per-image preprocessing chains: [i:v] … [v{i}], honouring the fit mode
	// (cover applies motion; contain/blur are static framings).
	for i := range s.Images {
		for _, c := range segmentChains(i, motions[i], fits[i], s.Width, s.Height, s.FPS, durs[i]) {
			graph.Add(c)
		}
	}

	total := totalDuration(durs, trans, blended)

	// Merge the segments into a single video stream, then colour grade.
	last := mergeSegments(graph, durs, s.Transition, s.ClipTransitions, trans, blended)
	if gf := gradeFilters(s.Grade); len(gf) > 0 {
		graph.Add(kernel.FilterChain{Inputs: []string{last}, Filters: gf, Outputs: []string{"vgraded"}})
		last = "vgraded"
	}

	// Audio bed input is appended first so caption input indices are
	// deterministic regardless of whether audio is present.
	hasAudio := s.AudioPath != ""
	if hasAudio {
		audioIdx := len(plan.Inputs)
		plan.Inputs = append(plan.Inputs, kernel.Input{Path: s.AudioPath})
		graph.Add(audioChain(audioIdx, total, s.AudioFadeInSeconds, s.AudioFadeOutSeconds, s.NormalizeAudio))
	}

	// Caption overlays: each pre-rendered PNG is a looped full-canvas input,
	// overlaid at 0:0 and gated to its time window.
	for i, c := range s.Captions {
		capIdx := len(plan.Inputs)
		plan.Inputs = append(plan.Inputs, kernel.Input{Path: c.Path, Loop: true, Duration: total})
		capLabel := fmt.Sprintf("cap%d", i)
		graph.Add(kernel.FilterChain{
			Inputs:  []string{fmt.Sprintf("%d:v", capIdx)},
			Filters: []string{"format=rgba"},
			Outputs: []string{capLabel},
		})
		out := fmt.Sprintf("cov%d", i)
		graph.Add(kernel.FilterChain{
			Inputs:  []string{last, capLabel},
			Filters: []string{fmt.Sprintf("overlay=0:0:enable='between(t,%s,%s)'", f(c.StartSeconds), f(c.EndSeconds))},
			Outputs: []string{out},
		})
		last = out
	}

	// Final normalisation to a stable [vout].
	graph.Add(kernel.FilterChain{
		Inputs:  []string{last},
		Filters: []string{"setsar=1", "format=yuv420p"},
		Outputs: []string{"vout"},
	})

	plan.Maps = []string{"[vout]"}
	plan.Out = append(videoEncodeArgs(s.Codec, s.FPS), "-movflags", "+faststart")
	if hasAudio {
		plan.Maps = append(plan.Maps, "[aout]")
		plan.Out = append(plan.Out, "-c:a", "aac", "-b:a", "192k", "-shortest")
	}

	plan.Graph = graph
	return plan, nil
}

// videoEncodeArgs returns the codec-specific `-c:v …` output flags. The preset
// only affects encode speed/size, never visual quality at a given CRF; the CRFs
// are matched (h264 20 ≈ hevc 24 ≈ av1 30) so the three look the same and only
// differ in size + playback support. `veryfast`/SVT preset 8 keep the encode
// quick — the zoompan+xfade filtergraph, not the encoder, dominates render time.
// An empty/unknown codec falls back to h264 (the universally-playable default).
func videoEncodeArgs(codec Codec, fps int) []string {
	r := fmt.Sprintf("%d", fps)
	switch codec {
	case CodecHEVC:
		// -tag:v hvc1 makes the HEVC stream playable in QuickTime/Safari.
		return []string{"-c:v", "libx265", "-preset", "medium", "-crf", "24", "-tag:v", "hvc1", "-pix_fmt", "yuv420p", "-r", r}
	case CodecAV1:
		return []string{"-c:v", "libsvtav1", "-preset", "8", "-crf", "30", "-pix_fmt", "yuv420p", "-r", r}
	default: // CodecH264 / "" / unknown
		return []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p", "-r", r}
	}
}

// mergeSegments joins v0..v{n-1} into one stream and returns its pad label.
// With a blended transition it chains xfade joins; otherwise it concats. durs
// holds each segment's resolved duration (they may differ, V3), so xfade
// offsets accumulate: join k starts trans before the end of the stream built so
// far, i.e. at (sum of durations 0..k-1) - k*trans. With uniform durations this
// reduces to k*(dur-trans). perJoin optionally overrides the transition style
// per join (empty/none → the global style).
func mergeSegments(graph *kernel.FilterGraph, durs []float64, style TransitionStyle, perJoin []TransitionStyle, trans float64, blended bool) string {
	n := len(durs)
	if n == 1 {
		return "v0"
	}
	if !blended {
		inputs := make([]string, n)
		for i := range inputs {
			inputs[i] = fmt.Sprintf("v%d", i)
		}
		graph.Add(kernel.FilterChain{
			Inputs:  inputs,
			Filters: []string{fmt.Sprintf("concat=n=%d:v=1:a=0", n)},
			Outputs: []string{"vcat"},
		})
		return "vcat"
	}

	cur := "v0"
	accum := durs[0] // length of the stream merged so far
	for i := 1; i < n; i++ {
		out := fmt.Sprintf("x%d", i)
		if i == n-1 {
			out = "vmerged"
		}
		offset := accum - trans
		joinStyle := style
		if k := i - 1; k < len(perJoin) && perJoin[k] != "" && perJoin[k] != TransitionNone {
			joinStyle = perJoin[k]
		}
		name := xfadeName(joinStyle, i-1)
		graph.Add(kernel.FilterChain{
			Inputs:  []string{cur, fmt.Sprintf("v%d", i)},
			Filters: []string{fmt.Sprintf("xfade=transition=%s:duration=%s:offset=%s", name, f(trans), f(offset))},
			Outputs: []string{out},
		})
		accum += durs[i] - trans
		cur = out
	}
	return cur
}

// audioChain builds the background-audio chain: [idx:a] → (loudnorm) → fades →
// apad → [aout]. loudnorm (when normalize) is single-pass EBU R128. The closing
// apad pads the bed with trailing silence so a music track shorter than the
// reel never truncates it — paired with the output's -shortest, the muxed
// length matches the (finite) video exactly.
func audioChain(idx int, total, fadeIn, fadeOut float64, normalize bool) kernel.FilterChain {
	filters := []string{"aresample=async=1"}
	if normalize {
		filters = append(filters, "loudnorm=I=-16:TP=-1.5:LRA=11")
	}
	if fadeIn > 0 {
		filters = append(filters, fmt.Sprintf("afade=t=in:st=0:d=%s", f(fadeIn)))
	}
	if fadeOut > 0 {
		st := total - fadeOut
		if st < 0 {
			st = 0
		}
		filters = append(filters, fmt.Sprintf("afade=t=out:st=%s:d=%s", f(st), f(fadeOut)))
	}
	filters = append(filters, "apad")
	return kernel.FilterChain{
		Inputs:  []string{fmt.Sprintf("%d:a", idx)},
		Filters: filters,
		Outputs: []string{"aout"},
	}
}

// BeatSnappedDuration rounds the per-image advance to a whole number of beats
// at the given BPM, returning the adjusted per-image on-screen duration so that
// every transition (or hard cut) lands on the beat. The advance is dur-trans
// for a blended transition (the spacing between xfade offsets) or dur for a
// hard concat cut. It is pure so both the compiler and the handler (which
// reports the effective per-image time) can agree on the snapped value.
func BeatSnappedDuration(dur, trans, bpm float64, blended bool) float64 {
	if bpm <= 0 {
		return dur
	}
	beat := 60.0 / bpm
	advance := dur
	if blended {
		advance = dur - trans
	}
	beats := math.Round(advance / beat)
	if beats < 1 {
		beats = 1
	}
	snapped := beats * beat
	if blended {
		return snapped + trans
	}
	return snapped
}

// totalDuration is the rendered length of the reel: the sum of the per-image
// durations, less the overlap reclaimed by each blended transition.
func totalDuration(durs []float64, trans float64, blended bool) float64 {
	sum := 0.0
	for _, d := range durs {
		sum += d
	}
	n := len(durs)
	if !blended || n < 2 {
		return sum
	}
	return sum - float64(n-1)*trans
}
