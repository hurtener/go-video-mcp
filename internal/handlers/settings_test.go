package handlers

import (
	"testing"

	"github.com/hurtener/go-video-mcp/internal/contracts"
)

// A bare template fills every unset field from its preset.
func TestResolveSettings_TemplateFillsDefaults(t *testing.T) {
	set, err := resolveSettings(contracts.CreateCinematicImageVideoInput{
		Template: "wedding_reel",
	}, 3)
	if err != nil {
		t.Fatalf("resolveSettings: %v", err)
	}
	// wedding_reel: 1920x1080, 30fps, ken_burns, film_dissolve@1.2, warm, 5s.
	if set.Width != 1920 || set.Height != 1080 {
		t.Errorf("canvas: got %dx%d, want 1920x1080", set.Width, set.Height)
	}
	if set.FPS != 30 {
		t.Errorf("fps: got %d, want 30", set.FPS)
	}
	if set.Motion != "ken_burns" {
		t.Errorf("motion: got %q, want ken_burns", set.Motion)
	}
	if set.Transition != "film_dissolve" {
		t.Errorf("transition: got %q, want film_dissolve", set.Transition)
	}
	if set.TransitionSeconds != 1.2 {
		t.Errorf("transition seconds: got %v, want 1.2", set.TransitionSeconds)
	}
	if set.Grade != "warm" {
		t.Errorf("grade: got %q, want warm", set.Grade)
	}
	if set.PerImage != 5 {
		t.Errorf("per-image: got %v, want 5", set.PerImage)
	}
}

// Explicit user-supplied fields override the template; unset fields still come
// from the template — the core V6 precedence rule.
func TestResolveSettings_ExplicitOverridesTemplate(t *testing.T) {
	set, err := resolveSettings(contracts.CreateCinematicImageVideoInput{
		Template:    "wedding_reel",
		Canvas:      "1080x1920", // override
		ColorGrade:  "cinematic", // override
		FPS:         24,          // override
		MotionStyle: "pan_left",  // override
	}, 3)
	if err != nil {
		t.Fatalf("resolveSettings: %v", err)
	}
	if set.Width != 1080 || set.Height != 1920 {
		t.Errorf("canvas override lost: got %dx%d", set.Width, set.Height)
	}
	if set.FPS != 24 {
		t.Errorf("fps override lost: got %d", set.FPS)
	}
	if set.Motion != "pan_left" {
		t.Errorf("motion override lost: got %q", set.Motion)
	}
	if set.Grade != "cinematic" {
		t.Errorf("grade override lost: got %q", set.Grade)
	}
	// Transition was NOT overridden → still the template's film_dissolve.
	if set.Transition != "film_dissolve" {
		t.Errorf("unset transition should keep template value, got %q", set.Transition)
	}
}

// With no template and no fields, the hardcoded defaults apply.
func TestResolveSettings_NoTemplateDefaults(t *testing.T) {
	set, err := resolveSettings(contracts.CreateCinematicImageVideoInput{}, 2)
	if err != nil {
		t.Fatalf("resolveSettings: %v", err)
	}
	if set.Width != 1920 || set.Height != 1080 {
		t.Errorf("default canvas: got %dx%d, want 1920x1080", set.Width, set.Height)
	}
	if set.FPS != defaultFPS {
		t.Errorf("default fps: got %d, want %d", set.FPS, defaultFPS)
	}
	if set.Motion != "ken_burns" {
		t.Errorf("default motion: got %q, want ken_burns", set.Motion)
	}
	if set.Transition != "fade" {
		t.Errorf("default transition: got %q, want fade", set.Transition)
	}
	if set.Grade != "neutral" {
		t.Errorf("default grade: got %q, want neutral", set.Grade)
	}
	if set.PerImage != defaultSecondsPerImage {
		t.Errorf("default per-image: got %v, want %v", set.PerImage, defaultSecondsPerImage)
	}
}

// TotalDuration still wins over the template's per-image default, and the
// transition is clamped strictly shorter than the derived per-image duration.
func TestResolveSettings_TotalDurationAndClamp(t *testing.T) {
	// 4 images, total 8s, product_launch (slide@0.6). Blended per-image:
	// d = (8 + 3*0.6)/4 = 2.45s; transition 0.6 < 2.45 so no clamp.
	set, err := resolveSettings(contracts.CreateCinematicImageVideoInput{
		Template:      "product_launch",
		TotalDuration: 8,
	}, 4)
	if err != nil {
		t.Fatalf("resolveSettings: %v", err)
	}
	if got := set.PerImage; got < 2.4 || got > 2.5 {
		t.Errorf("derived per-image: got %v, want ~2.45", got)
	}
	if set.TransitionSeconds != 0.6 {
		t.Errorf("transition seconds: got %v, want 0.6", set.TransitionSeconds)
	}
}
