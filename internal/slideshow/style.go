package slideshow

// TransitionStyle selects how one image gives way to the next.
type TransitionStyle string

const (
	TransitionNone     TransitionStyle = "none"          // hard cut (concat, no blend)
	TransitionFade     TransitionStyle = "fade"          // cross-dissolve
	TransitionWipe     TransitionStyle = "wipe"          // directional wipe
	TransitionSlide    TransitionStyle = "slide"         // push/slide
	TransitionZoomBlur TransitionStyle = "zoom_blur"     // zoom-in transition
	TransitionDissolve TransitionStyle = "film_dissolve" // grainy dissolve
	TransitionRandom   TransitionStyle = "random_safe"   // deterministic mix of safe ones
)

// MotionStyle selects per-image camera motion.
type MotionStyle string

const (
	MotionNone     MotionStyle = "none"
	MotionKenBurns MotionStyle = "ken_burns"     // gentle centred zoom-in
	MotionSlowPush MotionStyle = "slow_push"     // stronger centred zoom-in
	MotionPanLeft  MotionStyle = "pan_left"      // drift right→left
	MotionPanRight MotionStyle = "pan_right"     // drift left→right
	MotionParallax MotionStyle = "parallax_like" // first cut: aliased to ken_burns
)

// ColorGrade selects a final look applied to the whole reel.
type ColorGrade string

const (
	GradeNeutral      ColorGrade = "neutral"
	GradeWarm         ColorGrade = "warm"
	GradeCinematic    ColorGrade = "cinematic"
	GradeVintage      ColorGrade = "vintage"
	GradeHighContrast ColorGrade = "high_contrast"
)

// safeTransitions is the deterministic pool TransitionRandom cycles through.
// Deterministic (indexed, not random) so the compiler stays golden-testable.
var safeTransitions = []string{
	"fade", "dissolve", "smoothleft", "wipeleft", "slideleft", "zoomin",
}

// xfadeName maps a TransitionStyle (and the join index, for random_safe) to an
// FFmpeg xfade transition name. Unknown styles fall back to a plain fade.
func xfadeName(style TransitionStyle, joinIdx int) string {
	switch style {
	case TransitionFade:
		return "fade"
	case TransitionWipe:
		return "wipeleft"
	case TransitionSlide:
		return "slideleft"
	case TransitionZoomBlur:
		return "zoomin"
	case TransitionDissolve:
		return "dissolve"
	case TransitionRandom:
		return safeTransitions[joinIdx%len(safeTransitions)]
	default:
		return "fade"
	}
}

// gradeFilters returns the filter list for a colour grade, or nil for neutral.
// These are appended as a chain on the merged video stream.
func gradeFilters(g ColorGrade) []string {
	switch g {
	case GradeWarm:
		return []string{"eq=saturation=1.06", "colorbalance=rs=0.04:gs=0.01:bs=-0.05"}
	case GradeCinematic:
		return []string{"eq=contrast=1.08:saturation=0.97", "colorbalance=rs=-0.03:gh=0.02:bh=-0.03:bs=0.04"}
	case GradeVintage:
		return []string{"curves=preset=vintage", "eq=saturation=0.82"}
	case GradeHighContrast:
		return []string{"eq=contrast=1.3:saturation=1.08:brightness=0.02"}
	default: // GradeNeutral / unknown
		return nil
	}
}
