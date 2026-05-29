package captions

import (
	"bytes"
	"image/png"
	"os"
	"testing"

	"golang.org/x/image/font/opentype"
)

// a font that exists on the dev/CI machine; skip if none is present.
func testFont(t *testing.T) *opentype.Font {
	t.Helper()
	for _, p := range []string{
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	} {
		if _, err := os.Stat(p); err == nil {
			f, err := LoadFont(p)
			if err != nil {
				t.Fatalf("LoadFont(%s): %v", p, err)
			}
			return f
		}
	}
	t.Skip("no test font available")
	return nil
}

func TestRender_ProducesCanvasSizedPNG(t *testing.T) {
	f := testFont(t)
	for _, pos := range []Position{PositionTop, PositionCenter, PositionLowerThird} {
		raw, err := Render(f, Spec{Text: "Big Sur, 2024", Position: pos, CanvasW: 1280, CanvasH: 720})
		if err != nil {
			t.Fatalf("Render(%s): %v", pos, err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("decode png: %v", err)
		}
		if cfg.Width != 1280 || cfg.Height != 720 {
			t.Errorf("png is %dx%d, want 1280x720", cfg.Width, cfg.Height)
		}
	}
}

func TestRender_EmptyTextRejected(t *testing.T) {
	f := testFont(t)
	if _, err := Render(f, Spec{Text: "   ", Position: PositionTop, CanvasW: 1920, CanvasH: 1080}); err == nil {
		t.Error("expected empty text to be rejected")
	}
}
