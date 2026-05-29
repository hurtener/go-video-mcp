package handlers_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hurtener/go-video-mcp/internal/contracts"
	"github.com/hurtener/go-video-mcp/internal/handlers"
	"github.com/hurtener/go-video-mcp/internal/kernel"
)

// TestCinematicHandlerE2E exercises the full flagship handler path against real
// FFmpeg: path validation, total-duration derivation, portrait canvas, planned
// warnings, render, and probe. Gated behind FFMPEG_E2E (CLAUDE.md §6).
func TestCinematicHandlerE2E(t *testing.T) {
	if os.Getenv("FFMPEG_E2E") == "" {
		t.Skip("set FFMPEG_E2E=1 to run the real-FFmpeg handler test")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg not on PATH: %v", err)
	}

	root := t.TempDir()
	imgs := makeImages(t, root, 4, 1080, 1080)
	out := filepath.Join(root, "reel.mp4")

	k, err := kernel.New(kernel.Config{AllowedRoots: []string{root}, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("kernel.New: %v", err)
	}
	h := handlers.New(k)

	res, err := h.CreateCinematicImageVideo(context.Background(), contracts.CreateCinematicImageVideoInput{
		Images:          imgs,
		OutputPath:      out,
		Canvas:          "1080x1920", // portrait reel
		FPS:             24,
		TotalDuration:   8, // derive per-image to hit ~8s total
		TransitionStyle: "slide",
		MotionStyle:     "pan_right",
		ColorGrade:      "warm",
		Watermark:       "logo.png", // should surface a planned warning
	})
	if err != nil {
		t.Fatalf("CreateCinematicImageVideo: %v", err)
	}

	got := res.Structured
	if got.ImageCount != 4 {
		t.Errorf("ImageCount = %d, want 4", got.ImageCount)
	}
	if got.Render.Width != 1080 || got.Render.Height != 1920 {
		t.Errorf("dims = %dx%d, want 1080x1920", got.Render.Width, got.Render.Height)
	}
	if got.Render.DurationSec < 7 || got.Render.DurationSec > 9 {
		t.Errorf("duration = %.2fs, want ~8s", got.Render.DurationSec)
	}
	if len(got.Warnings) == 0 {
		t.Error("expected a planned warning for watermark")
	}
	if got.FilterComplex == "" {
		t.Error("expected the compiled filter_complex to be surfaced")
	}
}

func makeImages(t *testing.T, dir string, n, w, h int) []string {
	t.Helper()
	colors := []string{"red", "green", "blue", "orange"}
	var out []string
	for i := 0; i < n; i++ {
		p := filepath.Join(dir, fmt.Sprintf("p%d.png", i))
		args := []string{"-y", "-f", "lavfi", "-i",
			fmt.Sprintf("color=c=%s:s=%dx%d", colors[i%len(colors)], w, h),
			"-frames:v", "1", p}
		if b, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
			t.Fatalf("gen image: %v\n%s", err, b)
		}
		out = append(out, p)
	}
	return out
}
