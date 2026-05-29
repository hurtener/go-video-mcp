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
	// Motion selects per-image camera motion.
	Motion MotionStyle
	// Grade selects the final colour look.
	Grade ColorGrade
	// AudioPath, when non-empty, is an already-validated background audio file.
	AudioPath string
	// AudioFadeInSeconds / AudioFadeOutSeconds add fades to the audio bed.
	AudioFadeInSeconds  float64
	AudioFadeOutSeconds float64
	// Output is the destination file path (already validated for writing).
	Output string
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

	dur := s.SecondsPerImage
	trans := s.TransitionSeconds
	blended := s.Transition != TransitionNone && n > 1
	if blended {
		if trans <= 0 {
			trans = 1.0
		}
		if trans >= dur {
			return kernel.Plan{}, fmt.Errorf("slideshow: transition (%.2fs) must be shorter than per-image duration (%.2fs)", trans, dur)
		}
	}

	plan := kernel.Plan{Overwrite: true, Output: s.Output}
	graph := &kernel.FilterGraph{}

	// One looped-image input per photo, each held for `dur` seconds.
	for _, img := range s.Images {
		plan.Inputs = append(plan.Inputs, kernel.Input{Path: img, Loop: true, Duration: dur})
	}

	// Per-image motion/preprocessing chains: [i:v] … [v{i}].
	for i := range s.Images {
		graph.Add(kernel.FilterChain{
			Inputs:  []string{fmt.Sprintf("%d:v", i)},
			Filters: segmentFilters(s.Motion, s.Width, s.Height, s.FPS, dur),
			Outputs: []string{fmt.Sprintf("v%d", i)},
		})
	}

	// Merge the segments into a single video stream.
	merged := mergeSegments(graph, n, s.Transition, dur, trans, blended)

	// Optional colour grade, then a final normalisation to a stable [vout].
	last := merged
	if gf := gradeFilters(s.Grade); len(gf) > 0 {
		graph.Add(kernel.FilterChain{Inputs: []string{last}, Filters: gf, Outputs: []string{"vgraded"}})
		last = "vgraded"
	}
	graph.Add(kernel.FilterChain{
		Inputs:  []string{last},
		Filters: []string{"setsar=1", "format=yuv420p"},
		Outputs: []string{"vout"},
	})

	plan.Maps = []string{"[vout]"}
	plan.Out = []string{
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "20",
		"-pix_fmt", "yuv420p",
		"-r", fmt.Sprintf("%d", s.FPS),
		"-movflags", "+faststart",
	}

	// Optional audio bed.
	if s.AudioPath != "" {
		total := totalDuration(n, dur, trans, blended)
		audioIdx := len(plan.Inputs)
		plan.Inputs = append(plan.Inputs, kernel.Input{Path: s.AudioPath})
		graph.Add(audioChain(audioIdx, total, s.AudioFadeInSeconds, s.AudioFadeOutSeconds))
		plan.Maps = append(plan.Maps, "[aout]")
		plan.Out = append(plan.Out, "-c:a", "aac", "-b:a", "192k", "-shortest")
	}

	plan.Graph = graph
	return plan, nil
}

// mergeSegments joins v0..v{n-1} into one stream and returns its pad label.
// With a blended transition it chains xfade joins; otherwise it concats.
func mergeSegments(graph *kernel.FilterGraph, n int, style TransitionStyle, dur, trans float64, blended bool) string {
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
	for i := 1; i < n; i++ {
		out := fmt.Sprintf("x%d", i)
		if i == n-1 {
			out = "vmerged"
		}
		offset := float64(i) * (dur - trans)
		name := xfadeName(style, i-1)
		graph.Add(kernel.FilterChain{
			Inputs:  []string{cur, fmt.Sprintf("v%d", i)},
			Filters: []string{fmt.Sprintf("xfade=transition=%s:duration=%s:offset=%s", name, f(trans), f(offset))},
			Outputs: []string{out},
		})
		cur = out
	}
	return cur
}

// audioChain builds the background-audio chain: [idx:a] → fades → [aout].
func audioChain(idx int, total, fadeIn, fadeOut float64) kernel.FilterChain {
	filters := []string{"aresample=async=1"}
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
	return kernel.FilterChain{
		Inputs:  []string{fmt.Sprintf("%d:a", idx)},
		Filters: filters,
		Outputs: []string{"aout"},
	}
}

// totalDuration is the rendered length of the reel.
func totalDuration(n int, dur, trans float64, blended bool) float64 {
	if !blended || n < 2 {
		return float64(n) * dur
	}
	return float64(n)*dur - float64(n-1)*trans
}
