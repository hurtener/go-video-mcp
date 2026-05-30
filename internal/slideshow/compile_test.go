package slideshow

import (
	"strings"
	"testing"
)

// Test the simplest deterministic graph end-to-end (golden). A hard-cut, no
// motion, no audio reel — locks the chain format and the concat path.
func TestCompile_GoldenConcatNoMotion(t *testing.T) {
	spec := Spec{
		Images:          []string{"a.jpg", "b.jpg"},
		Width:           1920,
		Height:          1080,
		FPS:             30,
		SecondsPerImage: 4,
		Transition:      TransitionNone,
		Motion:          MotionNone,
		Grade:           GradeNeutral,
		Output:          "out.mp4",
	}
	plan, err := Compile(spec)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	const wantGraph = "[0:v]scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,fps=30,setsar=1,format=yuv420p[v0];" +
		"[1:v]scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,fps=30,setsar=1,format=yuv420p[v1];" +
		"[v0][v1]concat=n=2:v=1:a=0[vcat];" +
		"[vcat]setsar=1,format=yuv420p[vout]"
	if got := plan.Graph.String(); got != wantGraph {
		t.Errorf("graph mismatch:\n got: %s\nwant: %s", got, wantGraph)
	}
	if plan.Output != "out.mp4" || !plan.Overwrite {
		t.Errorf("unexpected output/overwrite: %q %v", plan.Output, plan.Overwrite)
	}
}

// xfade joins must use the derived offsets i*(dur-trans) and the mapped
// transition name.
func TestCompile_XfadeOffsets(t *testing.T) {
	spec := Spec{
		Images:            []string{"1.png", "2.png", "3.png"},
		Width:             1080,
		Height:            1920,
		FPS:               25,
		SecondsPerImage:   5,
		Transition:        TransitionFade,
		TransitionSeconds: 1,
		Motion:            MotionNone,
		Output:            "v.mp4",
	}
	plan, err := Compile(spec)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	g := plan.Graph.String()
	// First join offset = 1*(5-1) = 4; second = 2*(5-1) = 8.
	if !strings.Contains(g, "xfade=transition=fade:duration=1:offset=4") {
		t.Errorf("missing first xfade join in:\n%s", g)
	}
	if !strings.Contains(g, "xfade=transition=fade:duration=1:offset=8") {
		t.Errorf("missing second xfade join in:\n%s", g)
	}
	if !strings.Contains(g, "[vmerged]") {
		t.Errorf("expected [vmerged] label in:\n%s", g)
	}
}

// Ken Burns motion must emit a zoompan on a 2× working frame.
func TestCompile_KenBurnsZoompan(t *testing.T) {
	spec := Spec{
		Images:          []string{"a.jpg"},
		Width:           1920,
		Height:          1080,
		FPS:             30,
		SecondsPerImage: 4,
		Transition:      TransitionNone,
		Motion:          MotionKenBurns,
		Output:          "k.mp4",
	}
	plan, err := Compile(spec)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	g := plan.Graph.String()
	if !strings.Contains(g, "scale=3840:2160:force_original_aspect_ratio=increase") {
		t.Errorf("expected 2x pre-scale in:\n%s", g)
	}
	if !strings.Contains(g, "zoompan=z='min(1+") || !strings.Contains(g, "s=1920x1080") {
		t.Errorf("expected zoompan to canvas in:\n%s", g)
	}
}

// An audio bed adds an input, an [aout] map, and aac output options.
func TestCompile_AudioBed(t *testing.T) {
	spec := Spec{
		Images:              []string{"a.jpg", "b.jpg"},
		Width:               1280,
		Height:              720,
		FPS:                 30,
		SecondsPerImage:     3,
		Transition:          TransitionFade,
		TransitionSeconds:   1,
		Motion:              MotionNone,
		AudioPath:           "song.mp3",
		AudioFadeInSeconds:  1,
		AudioFadeOutSeconds: 2,
		Output:              "wedding.mp4",
	}
	plan, err := Compile(spec)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if n := len(plan.Inputs); n != 3 {
		t.Fatalf("want 3 inputs (2 img + audio), got %d", n)
	}
	g := plan.Graph.String()
	if !strings.Contains(g, "[2:a]aresample=async=1,afade=t=in:st=0:d=1") {
		t.Errorf("expected audio fade-in chain in:\n%s", g)
	}
	// total = 2*3 - 1*1 = 5; fade-out starts at 5-2 = 3; apad closes the chain.
	if !strings.Contains(g, "afade=t=out:st=3:d=2,apad[aout]") {
		t.Errorf("expected audio fade-out at st=3 + apad in:\n%s", g)
	}
	// NormalizeAudio defaulted off here → no loudnorm.
	if strings.Contains(g, "loudnorm") {
		t.Errorf("did not expect loudnorm when NormalizeAudio is false:\n%s", g)
	}
	if !contains(plan.Maps, "[aout]") {
		t.Errorf("expected [aout] map, got %v", plan.Maps)
	}
	if !argsContain(plan.Out, "-shortest") {
		t.Errorf("expected -shortest in out args: %v", plan.Out)
	}
}

// Captions become looped overlay inputs with time-gated overlay=0:0 chains,
// composited after the grade and before the final [vout].
func TestCompile_CaptionOverlays(t *testing.T) {
	spec := Spec{
		Images:          []string{"a.jpg", "b.jpg"},
		Width:           1280,
		Height:          720,
		FPS:             30,
		SecondsPerImage: 3,
		Transition:      TransitionNone,
		Motion:          MotionNone,
		Captions: []CaptionOverlay{
			{Path: "cap0.png", StartSeconds: 0, EndSeconds: 2.5},
		},
		Output: "o.mp4",
	}
	plan, err := Compile(spec)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// 2 images + 1 caption overlay input.
	if len(plan.Inputs) != 3 {
		t.Fatalf("want 3 inputs (2 img + 1 caption), got %d", len(plan.Inputs))
	}
	if !plan.Inputs[2].Loop || plan.Inputs[2].Path != "cap0.png" {
		t.Errorf("caption input not looped/correct: %+v", plan.Inputs[2])
	}
	g := plan.Graph.String()
	if !strings.Contains(g, "[2:v]format=rgba[cap0]") {
		t.Errorf("missing caption format chain in:\n%s", g)
	}
	if !strings.Contains(g, "overlay=0:0:enable='between(t,0,2.5)'") {
		t.Errorf("missing time-gated overlay in:\n%s", g)
	}
	if !strings.Contains(g, "[cov0]setsar=1,format=yuv420p[vout]") {
		t.Errorf("captions should compose before the final [vout] in:\n%s", g)
	}
}

// NormalizeAudio inserts a single-pass loudnorm right after the resample, ahead
// of the fades and the closing apad.
func TestCompile_AudioLoudnorm(t *testing.T) {
	spec := Spec{
		Images:          []string{"a.jpg", "b.jpg"},
		Width:           1280,
		Height:          720,
		FPS:             30,
		SecondsPerImage: 3,
		Transition:      TransitionNone,
		Motion:          MotionNone,
		AudioPath:       "song.mp3",
		NormalizeAudio:  true,
		Output:          "o.mp4",
	}
	plan, err := Compile(spec)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	g := plan.Graph.String()
	if !strings.Contains(g, "aresample=async=1,loudnorm=I=-16:TP=-1.5:LRA=11") {
		t.Errorf("expected loudnorm after resample in:\n%s", g)
	}
	if !strings.Contains(g, "apad[aout]") {
		t.Errorf("expected closing apad in:\n%s", g)
	}
}

// BeatSnappedDuration rounds the per-image advance to whole beats.
func TestBeatSnappedDuration(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		name            string
		dur, trans, bpm float64
		blended         bool
		want            float64
	}{
		// 120 BPM → 0.5s/beat. advance 2.7 → 5 beats (2.5) → +trans = 3.5.
		{"blended-round-down", 3.7, 1, 120, true, 3.5},
		// concat advance 3.7 → round(7.4)=7 beats → 3.5.
		{"concat", 3.7, 1, 120, false, 3.5},
		// bpm 0 is a no-op.
		{"no-bpm", 4, 1, 0, true, 4},
		// at least one beat even for a tiny advance.
		{"min-one-beat", 0.2, 0, 120, false, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BeatSnappedDuration(tc.dur, tc.trans, tc.bpm, tc.blended)
			if got < tc.want-eps || got > tc.want+eps {
				t.Errorf("BeatSnappedDuration(%v,%v,%v,%v) = %v, want %v",
					tc.dur, tc.trans, tc.bpm, tc.blended, got, tc.want)
			}
		})
	}
}

// With beat-sync the xfade offsets land on beat multiples: per-image advance is
// snapped, so offset_k = k*snappedAdvance is a whole number of beats.
func TestCompile_BeatSyncOffsets(t *testing.T) {
	spec := Spec{
		Images:            []string{"a.jpg", "b.jpg", "c.jpg"},
		Width:             1280,
		Height:            720,
		FPS:               30,
		SecondsPerImage:   3.7, // advance 2.7 → snaps to 2.5 (5 beats @120)
		Transition:        TransitionFade,
		TransitionSeconds: 1,
		Motion:            MotionNone,
		BeatSync:          true,
		BPM:               120,
		Output:            "b.mp4",
	}
	plan, err := Compile(spec)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	g := plan.Graph.String()
	// snappedAdvance = 2.5 → offsets 2.5 and 5.
	if !strings.Contains(g, "duration=1:offset=2.5") {
		t.Errorf("expected first beat-snapped offset 2.5 in:\n%s", g)
	}
	if !strings.Contains(g, "duration=1:offset=5") {
		t.Errorf("expected second beat-snapped offset 5 in:\n%s", g)
	}
	// Each looped image input is held for the snapped per-image duration (3.5).
	args := strings.Join(plan.ToArgs(), " ")
	if !strings.Contains(args, "-loop 1 -t 3.5 -i a.jpg") {
		t.Errorf("expected snapped per-image input duration 3.5 in:\n%s", args)
	}
}

// V3: the new motion presets emit distinct zoompan crop-window paths.
func TestCompile_MotionPresets(t *testing.T) {
	cases := []struct {
		motion MotionStyle
		// substrings the zoompan x/y expression must contain to be distinct.
		wantX, wantY string
	}{
		// diagonal_drift moves on both axes.
		{MotionDiagonal, "(iw-iw/zoom)*(0.15+0.7*", "(ih-ih/zoom)*(0.15+0.7*"},
		// parallax_like slides horizontally, stays vertically centred, zooms to 1.18.
		{MotionParallax, "(iw-iw/zoom)*(0.8-0.6*", "ih/2-(ih/zoom/2)"},
	}
	for _, tc := range cases {
		t.Run(string(tc.motion), func(t *testing.T) {
			plan, err := Compile(Spec{
				Images: []string{"a.jpg"}, Width: 1920, Height: 1080, FPS: 30,
				SecondsPerImage: 4, Transition: TransitionNone, Motion: tc.motion, Output: "m.mp4",
			})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			g := plan.Graph.String()
			if !strings.Contains(g, tc.wantX) {
				t.Errorf("motion %s missing x path %q in:\n%s", tc.motion, tc.wantX, g)
			}
			if !strings.Contains(g, tc.wantY) {
				t.Errorf("motion %s missing y path %q in:\n%s", tc.motion, tc.wantY, g)
			}
		})
	}
	// parallax_like must use its distinct stronger zoom (1.18), unlike ken_burns (1.12).
	plan, _ := Compile(Spec{
		Images: []string{"a.jpg"}, Width: 1920, Height: 1080, FPS: 30,
		SecondsPerImage: 4, Transition: TransitionNone, Motion: MotionParallax, Output: "m.mp4",
	})
	if !strings.Contains(plan.Graph.String(), ",1.18)") {
		t.Errorf("parallax_like should zoom to 1.18, graph:\n%s", plan.Graph.String())
	}
}

// V3: per-clip motion overrides apply to the right segment; unset clips inherit.
func TestCompile_PerClipMotion(t *testing.T) {
	plan, err := Compile(Spec{
		Images:          []string{"a.jpg", "b.jpg", "c.jpg"},
		Width:           1280,
		Height:          720,
		FPS:             30,
		SecondsPerImage: 4,
		Transition:      TransitionNone,
		Motion:          MotionKenBurns,
		ClipMotions:     []MotionStyle{"", MotionPanLeft}, // only image 1 overridden
		Output:          "o.mp4",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	g := plan.Graph.String()
	// Image 1 (v1) uses the pan path (animated crop, no zoompan).
	if !strings.Contains(g, "crop=1280:720:x='(iw-ow)*(1-t/4)':y='(ih-oh)/2',setsar=1,format=yuv420p[v1]") {
		t.Errorf("expected pan_left override on v1 in:\n%s", g)
	}
	// Images 0 and 2 keep ken_burns (zoompan).
	if !strings.Contains(g, "zoompan=z='min(1+") {
		t.Errorf("expected ken_burns zoompan on the non-overridden clips in:\n%s", g)
	}
}

// V3: per-clip durations drive both the input -t and the cumulative xfade
// offsets (offset_k = sum(durs[:k]) - k*trans).
func TestCompile_PerClipDurations(t *testing.T) {
	plan, err := Compile(Spec{
		Images:            []string{"a.jpg", "b.jpg", "c.jpg"},
		Width:             1280,
		Height:            720,
		FPS:               30,
		SecondsPerImage:   4,
		Transition:        TransitionFade,
		TransitionSeconds: 1,
		Motion:            MotionNone,
		ClipDurations:     []float64{2, 5, 0}, // image 2 falls back to the global 4
		Output:            "o.mp4",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	args := strings.Join(plan.ToArgs(), " ")
	if !strings.Contains(args, "-loop 1 -t 2 -i a.jpg") ||
		!strings.Contains(args, "-loop 1 -t 5 -i b.jpg") ||
		!strings.Contains(args, "-loop 1 -t 4 -i c.jpg") {
		t.Errorf("per-clip input durations wrong in:\n%s", args)
	}
	g := plan.Graph.String()
	// offset_1 = dur0 - 1*trans = 2 - 1 = 1.
	if !strings.Contains(g, "duration=1:offset=1[") {
		t.Errorf("expected first offset 1 in:\n%s", g)
	}
	// offset_2 = (dur0+dur1) - 2*trans = (2+5) - 2 = 5.
	if !strings.Contains(g, "duration=1:offset=5[vmerged]") {
		t.Errorf("expected second offset 5 in:\n%s", g)
	}
}

// V3: per-clip transition style overrides the join; unset joins keep the global.
func TestCompile_PerClipTransition(t *testing.T) {
	plan, err := Compile(Spec{
		Images:            []string{"a.jpg", "b.jpg", "c.jpg"},
		Width:             1280,
		Height:            720,
		FPS:               30,
		SecondsPerImage:   4,
		Transition:        TransitionFade,
		TransitionSeconds: 1,
		Motion:            MotionNone,
		ClipTransitions:   []TransitionStyle{TransitionWipe, ""}, // join 0 → wipe, join 1 → global fade
		Output:            "o.mp4",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	g := plan.Graph.String()
	if !strings.Contains(g, "xfade=transition=wipeleft:duration=1:offset=3") {
		t.Errorf("expected first join overridden to wipe in:\n%s", g)
	}
	if !strings.Contains(g, "xfade=transition=fade:duration=1:offset=6") {
		t.Errorf("expected second join to keep global fade in:\n%s", g)
	}
}

// Codec selects the output encoder; h264 is the default, av1/hevc are opt-in.
func TestCompile_Codec(t *testing.T) {
	base := Spec{
		Images: []string{"a.jpg"}, Width: 1920, Height: 1080, FPS: 30,
		SecondsPerImage: 4, Transition: TransitionNone, Motion: MotionNone, Output: "o.mp4",
	}
	cases := []struct {
		codec Codec
		want  string // a distinctive substring of the expected encode args
	}{
		{"", "-c:v libx264 -preset veryfast -crf 20"},          // default
		{CodecH264, "-c:v libx264 -preset veryfast -crf 20"},   // explicit
		{CodecHEVC, "-c:v libx265 -preset medium -crf 24 -tag:v hvc1"},
		{CodecAV1, "-c:v libsvtav1 -preset 8 -crf 30"},
	}
	for _, tc := range cases {
		spec := base
		spec.Codec = tc.codec
		plan, err := Compile(spec)
		if err != nil {
			t.Fatalf("Compile(%q): %v", tc.codec, err)
		}
		args := strings.Join(plan.ToArgs(), " ")
		if !strings.Contains(args, tc.want) {
			t.Errorf("codec %q: missing %q in:\n%s", tc.codec, tc.want, args)
		}
		if !strings.Contains(args, "-movflags +faststart") {
			t.Errorf("codec %q: expected +faststart", tc.codec)
		}
	}
}

func TestCompile_Errors(t *testing.T) {
	if _, err := Compile(Spec{Width: 1920, Height: 1080, FPS: 30, SecondsPerImage: 4}); err == nil {
		t.Error("expected error for no images")
	}
	// Transition not shorter than per-image duration.
	_, err := Compile(Spec{
		Images: []string{"a", "b"}, Width: 1920, Height: 1080, FPS: 30,
		SecondsPerImage: 1, Transition: TransitionFade, TransitionSeconds: 1, Output: "o.mp4",
	})
	if err == nil {
		t.Error("expected error for transition >= per-image duration")
	}
}

// ToArgs must place the looped-image pre-input options before each -i.
func TestCompile_InputArgs(t *testing.T) {
	plan, err := Compile(Spec{
		Images: []string{"a.jpg"}, Width: 1920, Height: 1080, FPS: 30,
		SecondsPerImage: 4, Transition: TransitionNone, Output: "o.mp4",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	args := strings.Join(plan.ToArgs(), " ")
	if !strings.Contains(args, "-loop 1 -t 4 -i a.jpg") {
		t.Errorf("expected looped image input args, got: %s", args)
	}
	if !strings.Contains(args, "-filter_complex") || !strings.Contains(args, "-map [vout]") {
		t.Errorf("expected filter_complex + map, got: %s", args)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func argsContain(ss []string, want string) bool { return contains(ss, want) }
