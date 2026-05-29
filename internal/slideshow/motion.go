package slideshow

import (
	"fmt"
	"strconv"
)

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
	case MotionKenBurns, MotionSlowPush, MotionParallax:
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

// zoomSegment builds a centred zoom-in segment via zoompan.
func zoomSegment(style MotionStyle, w, h, fps, frames int) []string {
	zmax := 1.12
	if style == MotionSlowPush {
		zmax = 1.20
	}
	// Per-frame zoom increment so z climbs from 1.0 to zmax across the segment.
	denom := frames - 1
	if denom < 1 {
		denom = 1
	}
	rate := (zmax - 1.0) / float64(denom)

	bw, bh := even(w*2), even(h*2) // 2× working frame to suppress jitter
	z := fmt.Sprintf("min(1+%s*on,%s)", f(rate), f(zmax))
	return []string{
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase", bw, bh),
		fmt.Sprintf("crop=%d:%d", bw, bh),
		fmt.Sprintf("fps=%d", fps),
		fmt.Sprintf("zoompan=z='%s':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=1:s=%dx%d:fps=%d", z, w, h, fps),
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
