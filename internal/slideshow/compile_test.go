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
	// total = 2*3 - 1*1 = 5; fade-out starts at 5-2 = 3.
	if !strings.Contains(g, "afade=t=out:st=3:d=2[aout]") {
		t.Errorf("expected audio fade-out at st=3 in:\n%s", g)
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
