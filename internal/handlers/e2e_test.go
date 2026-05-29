package handlers_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	h := handlers.New(k, root)

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

// TestCinematicCaptionsE2E renders a reel with burned-in captions (pure-Go
// rendered overlays composited via FFmpeg's overlay filter) and verifies the
// output. Gated behind FFMPEG_E2E.
func TestCinematicCaptionsE2E(t *testing.T) {
	if os.Getenv("FFMPEG_E2E") == "" {
		t.Skip("set FFMPEG_E2E=1 to run the real-FFmpeg captions test")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg not on PATH: %v", err)
	}

	root := t.TempDir()
	imgs := makeImages(t, root, 2, 1280, 720)
	out := filepath.Join(root, "captioned.mp4")

	k, err := kernel.New(kernel.Config{AllowedRoots: []string{root}, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("kernel.New: %v", err)
	}
	h := handlers.New(k, root)

	res, err := h.CreateCinematicImageVideo(context.Background(), contracts.CreateCinematicImageVideoInput{
		Images:           imgs,
		OutputPath:       out,
		Canvas:           "1280x720",
		FPS:              30,
		DurationPerImage: 2,
		Captions: []contracts.Caption{
			{Text: "Big Sur — 2024", StartSeconds: 0, EndSeconds: 2, Position: "lower_third"},
			{Text: "The drive home", StartSeconds: 2, EndSeconds: 4, Position: "top"},
		},
	})
	if err != nil {
		t.Fatalf("CreateCinematicImageVideo: %v", err)
	}
	got := res.Structured
	// If a font was found, captions render → no caption warning + overlay in graph.
	for _, w := range got.Warnings {
		if strings.Contains(w, "captions") {
			t.Skipf("no font available on this machine: %s", w)
		}
	}
	if !strings.Contains(got.FilterComplex, "overlay=0:0:enable='between(t,0,2)'") {
		t.Errorf("expected first caption overlay in graph:\n%s", got.FilterComplex)
	}
	if got.Render.Width != 1280 || got.Render.Height != 720 {
		t.Errorf("dims = %dx%d, want 1280x720", got.Render.Width, got.Render.Height)
	}
	if got.Render.DurationSec < 3 || got.Render.DurationSec > 5 {
		t.Errorf("duration = %.2fs, want ~4s", got.Render.DurationSec)
	}
}

// TestCinematicTemplatesE2E renders each V6 cinematic template one-shot (images
// + template only) against real FFmpeg and verifies the template's canvas and
// grade reached the output/graph — the override-precedence contract end-to-end.
// Gated behind FFMPEG_E2E (CLAUDE.md §6).
func TestCinematicTemplatesE2E(t *testing.T) {
	if os.Getenv("FFMPEG_E2E") == "" {
		t.Skip("set FFMPEG_E2E=1 to run the real-FFmpeg templates test")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg not on PATH: %v", err)
	}

	cases := []struct {
		template   string
		wantW      int
		wantH      int
		gradeMatch string // a distinctive substring of the grade chain
	}{
		{"wedding_reel", 1920, 1080, "saturation=1.06"},
		{"product_launch", 1080, 1920, "contrast=1.3"},
		{"memory_montage", 1920, 1080, "curves=preset=vintage"},
		{"travel_diary", 1920, 1080, "eq=contrast=1.08"},
	}

	for _, tc := range cases {
		t.Run(tc.template, func(t *testing.T) {
			root := t.TempDir()
			imgs := makeImages(t, root, 3, 800, 800)
			out := filepath.Join(root, "reel.mp4")

			k, err := kernel.New(kernel.Config{AllowedRoots: []string{root}, Timeout: 2 * time.Minute})
			if err != nil {
				t.Fatalf("kernel.New: %v", err)
			}
			h := handlers.New(k, root)

			res, err := h.CreateCinematicImageVideo(context.Background(), contracts.CreateCinematicImageVideoInput{
				Images:        imgs,
				OutputPath:    out,
				Template:      contracts.Template(tc.template),
				TotalDuration: 6, // keep each render quick
			})
			if err != nil {
				t.Fatalf("CreateCinematicImageVideo: %v", err)
			}
			got := res.Structured
			if got.Render.Width != tc.wantW || got.Render.Height != tc.wantH {
				t.Errorf("dims = %dx%d, want %dx%d", got.Render.Width, got.Render.Height, tc.wantW, tc.wantH)
			}
			if !strings.Contains(got.FilterComplex, tc.gradeMatch) {
				t.Errorf("expected grade %q in graph:\n%s", tc.gradeMatch, got.FilterComplex)
			}
			if got.Render.DurationSec < 5 || got.Render.DurationSec > 7 {
				t.Errorf("duration = %.2fs, want ~6s", got.Render.DurationSec)
			}
		})
	}
}

// TestCinematicAudioV5E2E renders a reel over a music bed shorter than the reel
// with loudness-normalize + beat-sync on, and verifies (a) audio is present,
// (b) the closing apad keeps the reel at its full video length instead of being
// truncated to the short bed, and (c) loudnorm is in the graph. Gated by
// FFMPEG_E2E.
func TestCinematicAudioV5E2E(t *testing.T) {
	if os.Getenv("FFMPEG_E2E") == "" {
		t.Skip("set FFMPEG_E2E=1 to run the real-FFmpeg V5 audio test")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg not on PATH: %v", err)
	}

	root := t.TempDir()
	imgs := makeImages(t, root, 3, 800, 800)
	audio := makeAudio(t, root, 2) // deliberately shorter than the ~6s reel
	out := filepath.Join(root, "reel.mp4")

	k, err := kernel.New(kernel.Config{AllowedRoots: []string{root}, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("kernel.New: %v", err)
	}
	h := handlers.New(k, root)

	res, err := h.CreateCinematicImageVideo(context.Background(), contracts.CreateCinematicImageVideoInput{
		Images:              imgs,
		OutputPath:          out,
		Canvas:              "1280x720",
		TotalDuration:       6,
		TransitionStyle:     "fade",
		BackgroundAudio:     audio,
		AudioFadeInSeconds:  1,
		AudioFadeOutSeconds: 1,
		BeatSync:            true,
		BPM:                 120,
		// NormalizeAudio defaults on (nil pointer).
	})
	if err != nil {
		t.Fatalf("CreateCinematicImageVideo: %v", err)
	}
	got := res.Structured
	if !strings.Contains(got.FilterComplex, "loudnorm") {
		t.Errorf("expected loudnorm in graph:\n%s", got.FilterComplex)
	}
	mi, err := k.Probe(context.Background(), got.Render.OutputPath)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !mi.HasVideo || !mi.HasAudio {
		t.Errorf("expected video+audio, got %+v", mi)
	}
	// apad must keep the reel at ~6s despite the 2s bed.
	if mi.DurationSec < 5 || mi.DurationSec > 7 {
		t.Errorf("duration = %.2fs, want ~6s (apad should prevent truncation to 2s)", mi.DurationSec)
	}
}

// TestCinematicPerClipE2E renders a reel mixing per-clip motion, duration, and
// transition overrides (V3) against real FFmpeg and verifies the output length
// reflects the variable per-clip durations. Gated by FFMPEG_E2E.
func TestCinematicPerClipE2E(t *testing.T) {
	if os.Getenv("FFMPEG_E2E") == "" {
		t.Skip("set FFMPEG_E2E=1 to run the real-FFmpeg per-clip test")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg not on PATH: %v", err)
	}

	root := t.TempDir()
	imgs := makeImages(t, root, 3, 800, 800)
	out := filepath.Join(root, "reel.mp4")

	k, err := kernel.New(kernel.Config{AllowedRoots: []string{root}, Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("kernel.New: %v", err)
	}
	h := handlers.New(k, root)

	res, err := h.CreateCinematicImageVideo(context.Background(), contracts.CreateCinematicImageVideoInput{
		Images:            imgs,
		OutputPath:        out,
		Canvas:            "1280x720",
		TransitionStyle:   "fade",
		TransitionSeconds: 1,
		MotionStyle:       "ken_burns",
		DurationPerImage:  3,
		Clips: []contracts.PerClip{
			{Motion: "diagonal_drift", DurationSeconds: 2},
			{Motion: "parallax_like", Transition: "wipe", DurationSeconds: 4},
			{Motion: "pan_right"}, // duration inherits the global 3
		},
	})
	if err != nil {
		t.Fatalf("CreateCinematicImageVideo: %v", err)
	}
	got := res.Structured
	// Per-clip motions in the graph.
	for _, want := range []string{"(iw-iw/zoom)*(0.15+0.7*", "(iw-iw/zoom)*(0.8-0.6*", "xfade=transition=wipeleft"} {
		if !strings.Contains(got.FilterComplex, want) {
			t.Errorf("expected %q in graph:\n%s", want, got.FilterComplex)
		}
	}
	// total = (2+4+3) - 2*1 = 7s.
	if got.Render.DurationSec < 6 || got.Render.DurationSec > 8 {
		t.Errorf("duration = %.2fs, want ~7s", got.Render.DurationSec)
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

// makeAudio generates a sine-wave WAV of the given length (a stand-in music bed).
func makeAudio(t *testing.T, dir string, seconds int) string {
	t.Helper()
	p := filepath.Join(dir, "bed.wav")
	args := []string{"-y", "-f", "lavfi", "-i",
		fmt.Sprintf("sine=frequency=440:duration=%d", seconds), p}
	if b, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("gen audio: %v\n%s", err, b)
	}
	return p
}
