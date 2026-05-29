// Package templates holds the cinematic-template registry for the flagship
// create_cinematic_image_video tool. A template is a named bundle of sensible
// look-and-timing defaults — canvas, fps, motion, transition, grade, timing —
// so a caller gets a polished reel in one shot ("wedding_reel", "travel_diary").
//
// The registry is pure (no I/O, no FFmpeg, no host state) so it stays trivially
// testable. The values here are deliberately plain strings/numbers that mirror
// the contract vocabularies (MotionStyle, TransitionStyle, ColorGrade); the
// handler maps them onto the typed Spec. A template only *fills unset fields* —
// an explicit user-supplied value always wins (see handler resolution).
package templates

import "strings"

// Preset is the set of defaults a template contributes. A zero-valued field
// means "this template has no opinion here" — the handler falls back to its own
// hardcoded default for that field.
type Preset struct {
	// Canvas is the output frame size, "WxH" (e.g. "1920x1080"). Empty = no opinion.
	Canvas string
	// FPS is the output frame rate. Zero = no opinion.
	FPS int
	// Motion is the per-image camera motion (a contracts.MotionStyle value).
	Motion string
	// Transition is the join style between images (a contracts.TransitionStyle value).
	Transition string
	// TransitionSeconds is the crossfade duration. Zero = no opinion.
	TransitionSeconds float64
	// Grade is the final colour look (a contracts.ColorGrade value).
	Grade string
	// SecondsPerImage is the default on-screen time per image. Zero = no opinion.
	SecondsPerImage float64
}

// Name is a template identifier. The empty string and "none" both mean "no
// template" — the handler applies its own defaults.
type Name string

const (
	None          Name = "none"
	WeddingReel   Name = "wedding_reel"
	ProductLaunch Name = "product_launch"
	MemoryMontage Name = "memory_montage"
	TravelDiary   Name = "travel_diary"
)

// registry maps a template name to its Preset. Adding a template is a one-line
// entry here plus its allowed value in the contract's Template doc comment.
var registry = map[Name]Preset{
	// A warm, unhurried anniversary/ceremony reel: gentle Ken Burns, long holds,
	// a soft film dissolve, warm grade.
	WeddingReel: {
		Canvas:            "1920x1080",
		FPS:               30,
		Motion:            "ken_burns",
		Transition:        "film_dissolve",
		TransitionSeconds: 1.2,
		Grade:             "warm",
		SecondsPerImage:   5,
	},
	// A punchy vertical product teaser: snappy slow-push, quick slide cuts, high
	// contrast — built for a 9:16 reel.
	ProductLaunch: {
		Canvas:            "1080x1920",
		FPS:               30,
		Motion:            "slow_push",
		Transition:        "slide",
		TransitionSeconds: 0.6,
		Grade:             "high_contrast",
		SecondsPerImage:   3,
	},
	// A nostalgic memory piece: easy Ken Burns, plain crossfades, vintage grade.
	MemoryMontage: {
		Canvas:            "1920x1080",
		FPS:               30,
		Motion:            "ken_burns",
		Transition:        "fade",
		TransitionSeconds: 1.0,
		Grade:             "vintage",
		SecondsPerImage:   4,
	},
	// A breezy travelogue: lateral pans, directional wipes, cinematic grade.
	TravelDiary: {
		Canvas:            "1920x1080",
		FPS:               30,
		Motion:            "pan_right",
		Transition:        "wipe",
		TransitionSeconds: 0.8,
		Grade:             "cinematic",
		SecondsPerImage:   4,
	},
}

// Lookup returns the Preset for a template name and whether one exists. The
// lookup is case-insensitive and trims surrounding whitespace; "" and "none"
// (any case) report ok=false so the caller uses its own defaults.
func Lookup(name string) (Preset, bool) {
	n := Name(strings.ToLower(strings.TrimSpace(name)))
	if n == "" || n == None {
		return Preset{}, false
	}
	p, ok := registry[n]
	return p, ok
}
