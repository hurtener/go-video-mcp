package slideshow_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hurtener/go-video-mcp/internal/kernel"
	"github.com/hurtener/go-video-mcp/internal/slideshow"
)

// TestEndToEndRender compiles a real slideshow spec and renders it with FFmpeg,
// then probes the result. It is gated behind FFMPEG_E2E so the default unit
// suite stays hermetic and fast (CLAUDE.md §6). Run with:
//
//	FFMPEG_E2E=1 go test ./internal/slideshow/ -run EndToEnd -v
func TestEndToEndRender(t *testing.T) {
	if os.Getenv("FFMPEG_E2E") == "" {
		t.Skip("set FFMPEG_E2E=1 to run the real-FFmpeg render test")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg not on PATH: %v", err)
	}

	root := t.TempDir()
	imgs := genImages(t, root, 3, 1280, 720)
	audio := genAudio(t, root, 12)
	out := filepath.Join(root, "reel.mp4")

	k, err := kernel.New(kernel.Config{AllowedRoots: []string{root}, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("kernel.New: %v", err)
	}

	// Validate every path through the kernel, exactly as a handler would.
	var validImgs []string
	for _, p := range imgs {
		v, err := k.ValidatePath(p, kernel.ModeRead)
		if err != nil {
			t.Fatalf("validate %s: %v", p, err)
		}
		validImgs = append(validImgs, v)
	}
	validAudio, err := k.ValidatePath(audio, kernel.ModeRead)
	if err != nil {
		t.Fatalf("validate audio: %v", err)
	}
	validOut, err := k.ValidatePath(out, kernel.ModeWrite)
	if err != nil {
		t.Fatalf("validate out: %v", err)
	}

	spec := slideshow.Spec{
		Images:              validImgs,
		Width:               1280,
		Height:              720,
		FPS:                 30,
		SecondsPerImage:     3,
		Transition:          slideshow.TransitionFade,
		TransitionSeconds:   1,
		Motion:              slideshow.MotionKenBurns,
		Grade:               slideshow.GradeCinematic,
		AudioPath:           validAudio,
		AudioFadeInSeconds:  1,
		AudioFadeOutSeconds: 2,
		Output:              validOut,
	}
	plan, err := slideshow.Compile(spec)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	t.Logf("command: %s", plan.String())

	var updates atomic.Int64
	res, err := k.RunPlan(context.Background(), plan, func(p kernel.Progress) {
		updates.Add(1)
	})
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if updates.Load() == 0 {
		t.Error("expected at least one progress update")
	}

	info, err := k.Probe(context.Background(), res.Output)
	if err != nil {
		t.Fatalf("Probe rendered file: %v", err)
	}
	if !info.HasVideo || !info.HasAudio {
		t.Errorf("expected video+audio, got %+v", info)
	}
	if info.Width != 1280 || info.Height != 720 {
		t.Errorf("expected 1280x720, got %dx%d", info.Width, info.Height)
	}
	// total = 3*3 - 2*1 = 7s. Allow generous tolerance for encoder rounding.
	if info.DurationSec < 6 || info.DurationSec > 8 {
		t.Errorf("expected ~7s duration, got %.2fs", info.DurationSec)
	}
}

func genImages(t *testing.T, dir string, n, w, h int) []string {
	t.Helper()
	colors := []string{"red", "green", "blue", "orange", "purple"}
	var out []string
	for i := 0; i < n; i++ {
		p := filepath.Join(dir, fmt.Sprintf("img%d.png", i))
		// A coloured test pattern so each image is visibly distinct.
		args := []string{
			"-y", "-f", "lavfi",
			"-i", fmt.Sprintf("color=c=%s:s=%dx%d", colors[i%len(colors)], w, h),
			"-frames:v", "1", p,
		}
		if b, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
			t.Fatalf("generate image %d: %v\n%s", i, err, b)
		}
		out = append(out, p)
	}
	return out
}

func genAudio(t *testing.T, dir string, seconds int) string {
	t.Helper()
	p := filepath.Join(dir, "song.wav")
	args := []string{
		"-y", "-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=440:duration=%d", seconds),
		p,
	}
	if b, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("generate audio: %v\n%s", err, b)
	}
	return p
}
