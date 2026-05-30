package slideshow

import (
	"fmt"
	"strconv"

	"github.com/hurtener/go-video-mcp/internal/kernel"
)

// segmentChains builds the filtergraph chain(s) that turn image input idx into a
// canvas-sized segment [v{idx}], honouring the fit mode:
//   - cover: scale-to-fill + crop, with the requested camera motion (the
//     original behaviour) — a single chain.
//   - contain: scale-to-fit + black bars (letterbox/pillarbox), static.
//   - blur: scale-to-fit over a blurred zoomed copy filling the bars, static.
//
// contain/blur are static (no zoompan/pan): combining a moving crop window with
// runtime-unknown letterbox geometry is fragile, and the point of those modes
// is to show the whole frame. Every mode ends in W×H / yuv420p / setsar=1 / fps
// so the xfade joins (which demand matching streams) work across mixed fits.
func segmentChains(idx int, motion MotionStyle, fit Fit, w, h, fps int, dur float64) []kernel.FilterChain {
	in := fmt.Sprintf("%d:v", idx)
	out := fmt.Sprintf("v%d", idx)

	switch fit {
	case FitContain:
		return []kernel.FilterChain{{
			Inputs: []string{in},
			Filters: []string{
				fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", w, h),
				fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black", w, h),
				fmt.Sprintf("fps=%d", fps),
				"setsar=1",
				"format=yuv420p",
			},
			Outputs: []string{out},
		}}

	case FitBlur:
		bg := fmt.Sprintf("bg%d", idx)
		fg := fmt.Sprintf("fg%d", idx)
		bgo := fmt.Sprintf("bgo%d", idx)
		fgo := fmt.Sprintf("fgo%d", idx)
		return []kernel.FilterChain{
			{Inputs: []string{in}, Filters: []string{"split=2"}, Outputs: []string{bg, fg}},
			{Inputs: []string{bg}, Filters: []string{
				coverScale(w, h),
				fmt.Sprintf("crop=%d:%d", w, h),
				"boxblur=20:1",
				"eq=brightness=-0.06",
				fmt.Sprintf("fps=%d", fps),
			}, Outputs: []string{bgo}},
			{Inputs: []string{fg}, Filters: []string{
				fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", w, h),
				fmt.Sprintf("fps=%d", fps),
			}, Outputs: []string{fgo}},
			{Inputs: []string{bgo, fgo}, Filters: []string{
				"overlay=(W-w)/2:(H-h)/2",
				"setsar=1",
				"format=yuv420p",
			}, Outputs: []string{out}},
		}

	default: // FitCover / "" / unknown — fill + crop, with motion.
		return []kernel.FilterChain{{
			Inputs:  []string{in},
			Filters: segmentFilters(motion, w, h, fps, dur),
			Outputs: []string{out},
		}}
	}
}

// segmentFilters builds the per-image preprocessing chain that turns one input
// image into a canvas-sized, fps-normalised, yuv420p video segment of `dur`
// seconds with the requested camera motion. Every segment ends in the same
// format/size/sar/fps so xfade (which demands matching streams) just works.
//
// Two motion families:
//   - zoom (ken_burns / slow_push / parallax_like): zoompan with d=1 over an
//     fps-normalised stream — `on` runs 0..frames-1, so the zoom ramp is exact.
//     The image is pre-scaled to 2× canvas to suppress zoompan's integer-crop
//     jitter.
//   - pan (pan_left / pan_right): an animated crop across a frame pre-scaled
//     wider than the canvas; no zoompan, so no jitter.
func segmentFilters(style MotionStyle, w, h, fps int, dur float64) []string {
	frames := int(dur*float64(fps) + 0.5)
	if frames < 1 {
		frames = 1
	}

	switch style {
	case MotionPanLeft, MotionPanRight:
		return panSegment(style, w, h, fps, dur)
	case MotionKenBurns, MotionSlowPush, MotionParallax, MotionDiagonal:
		return zoomSegment(style, w, h, fps, frames)
	default: // MotionNone / unknown
		return []string{
			coverScale(w, h),
			fmt.Sprintf("crop=%d:%d", w, h),
			fmt.Sprintf("fps=%d", fps),
			"setsar=1",
			"format=yuv420p",
		}
	}
}

// zoomSegment builds a zoom-in segment via zoompan. The zoom ramp is shared by
// the whole family; the crop-window path (x,y) is what distinguishes them:
//   - ken_burns / slow_push: a centred push (no drift).
//   - diagonal_drift: the window drifts across both axes (a true diagonal move).
//   - parallax_like: a pronounced zoom with a horizontal slide (foreground-
//     parallax feel) — distinct from the centred ken_burns it used to alias.
//
// x/y reference zoompan's per-output-frame `on` (0..frames-1). The available
// slack at the current zoom is (iw - iw/zoom) horizontally and (ih - ih/zoom)
// vertically; a centred window sits at half that. Drifts move a fraction of the
// slack across the segment so the subject never leaves frame.
func zoomSegment(style MotionStyle, w, h, fps, frames int) []string {
	zmax := 1.12
	switch style {
	case MotionSlowPush:
		zmax = 1.20
	case MotionParallax:
		zmax = 1.18
	}
	// Per-frame zoom increment so z climbs from 1.0 to zmax across the segment.
	denom := frames - 1
	if denom < 1 {
		denom = 1
	}
	rate := (zmax - 1.0) / float64(denom)
	// p is the normalised progress 0..1 across the segment's output frames.
	p := fmt.Sprintf("(on/%d)", denom)

	xCenter := "iw/2-(iw/zoom/2)"
	yCenter := "ih/2-(ih/zoom/2)"
	x, y := xCenter, yCenter
	switch style {
	case MotionDiagonal:
		// Drift across 70% of the slack on both axes (upper-left → lower-right).
		x = fmt.Sprintf("(iw-iw/zoom)*(0.15+0.7*%s)", p)
		y = fmt.Sprintf("(ih-ih/zoom)*(0.15+0.7*%s)", p)
	case MotionParallax:
		// Horizontal slide right→left over 60% of the slack, vertically centred —
		// the slide against the zoom reads as parallax.
		x = fmt.Sprintf("(iw-iw/zoom)*(0.8-0.6*%s)", p)
	}

	bw, bh := even(w*2), even(h*2) // 2× working frame to suppress jitter
	z := fmt.Sprintf("min(1+%s*on,%s)", f(rate), f(zmax))
	return []string{
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase", bw, bh),
		fmt.Sprintf("crop=%d:%d", bw, bh),
		fmt.Sprintf("fps=%d", fps),
		fmt.Sprintf("zoompan=z='%s':x='%s':y='%s':d=1:s=%dx%d:fps=%d", z, x, y, w, h, fps),
		"setsar=1",
		"format=yuv420p",
	}
}

// panSegment builds a horizontal drift via an animated crop over a wider frame.
func panSegment(style MotionStyle, w, h, fps int, dur float64) []string {
	bw := even(int(float64(w)*1.25 + 0.5)) // 25% horizontal headroom to pan into
	var x string
	if style == MotionPanLeft {
		x = fmt.Sprintf("(iw-ow)*(1-t/%s)", f(dur))
	} else {
		x = fmt.Sprintf("(iw-ow)*t/%s", f(dur))
	}
	return []string{
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase", bw, h),
		fmt.Sprintf("crop=%d:%d", bw, h),
		fmt.Sprintf("fps=%d", fps),
		fmt.Sprintf("crop=%d:%d:x='%s':y='(ih-oh)/2'", w, h, x),
		"setsar=1",
		"format=yuv420p",
	}
}

// coverScale scales an image to fully cover w×h, preserving aspect (the excess
// is cropped by the following crop filter).
func coverScale(w, h int) string {
	return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase", w, h)
}

// even rounds up to the nearest even integer — libx264 (yuv420p) requires even
// dimensions.
func even(n int) int {
	if n%2 != 0 {
		return n + 1
	}
	return n
}

// f formats a float compactly for embedding in a filter expression.
func f(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
