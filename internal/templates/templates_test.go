package templates

import "testing"

// Every named template resolves to a fully-populated preset (no zero fields),
// so a one-shot call with just a template yields a complete, polished config.
func TestLookup_KnownTemplatesArePopulated(t *testing.T) {
	for _, name := range []Name{WeddingReel, ProductLaunch, MemoryMontage, TravelDiary} {
		p, ok := Lookup(string(name))
		if !ok {
			t.Errorf("%s: expected a preset, got none", name)
			continue
		}
		if p.Canvas == "" || p.FPS == 0 || p.Motion == "" || p.Transition == "" ||
			p.Grade == "" || p.TransitionSeconds == 0 || p.SecondsPerImage == 0 {
			t.Errorf("%s: preset has an unset field: %+v", name, p)
		}
	}
}

// "", "none", and unknown names report ok=false (caller uses its own defaults).
func TestLookup_NoneAndUnknown(t *testing.T) {
	for _, name := range []string{"", "none", "None", " none ", "does_not_exist"} {
		if _, ok := Lookup(name); ok {
			t.Errorf("Lookup(%q): expected ok=false", name)
		}
	}
}

// Lookup is case-insensitive and trims surrounding whitespace.
func TestLookup_CaseInsensitive(t *testing.T) {
	want, _ := Lookup("wedding_reel")
	got, ok := Lookup("  WEDDING_REEL ")
	if !ok || got != want {
		t.Errorf("case-insensitive lookup mismatch: got %+v ok=%v, want %+v", got, ok, want)
	}
}

// Spot-check a couple of distinctive preset values so an accidental edit to the
// registry is caught.
func TestLookup_KnownValues(t *testing.T) {
	if p, _ := Lookup("product_launch"); p.Canvas != "1080x1920" || p.Grade != "high_contrast" {
		t.Errorf("product_launch preset drifted: %+v", p)
	}
	if p, _ := Lookup("wedding_reel"); p.Grade != "warm" || p.Transition != "film_dissolve" {
		t.Errorf("wedding_reel preset drifted: %+v", p)
	}
}
