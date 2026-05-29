package handlers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hurtener/go-video-mcp/internal/contracts"
)

// open_media_uploader reports its view kind and the server's allowed roots.
func TestOpenMediaUploader(t *testing.T) {
	k, root := testKernel(t)
	h := New(k, root)
	res, err := h.OpenMediaUploader(context.Background(), contracts.OpenMediaUploaderInput{Note: "drop here"})
	if err != nil {
		t.Fatalf("OpenMediaUploader: %v", err)
	}
	out := res.Structured
	if out.Kind != "media_uploader" {
		t.Errorf("kind = %q, want media_uploader", out.Kind)
	}
	if out.Note != "drop here" {
		t.Errorf("note = %q, want \"drop here\"", out.Note)
	}
	if len(out.Roots) == 0 || out.Roots[0] != root {
		t.Errorf("roots = %v, want [%s]", out.Roots, root)
	}
}

// open_studio opens the composer view; with no images it just reports "studio".
func TestOpenStudio_Empty(t *testing.T) {
	k, root := testKernel(t)
	h := New(k, root)
	res, err := h.OpenStudio(context.Background(), contracts.OpenStudioInput{})
	if err != nil {
		t.Fatalf("OpenStudio: %v", err)
	}
	if res.Structured.Kind != "studio" {
		t.Errorf("kind = %q, want studio", res.Structured.Kind)
	}
	if len(res.Structured.Images) != 0 {
		t.Errorf("expected no images, got %v", res.Structured.Images)
	}
}

// open_studio validates pre-load images and echoes the resolved paths.
func TestOpenStudio_PreloadImages(t *testing.T) {
	k, root := testKernel(t)
	img := filepath.Join(root, "a.jpg")
	write(t, img)
	h := New(k, root)
	res, err := h.OpenStudio(context.Background(), contracts.OpenStudioInput{
		Images:   []string{img},
		Template: "wedding_reel",
	})
	if err != nil {
		t.Fatalf("OpenStudio: %v", err)
	}
	if len(res.Structured.Images) != 1 || res.Structured.Images[0] != img {
		t.Errorf("images = %v, want [%s]", res.Structured.Images, img)
	}
	if res.Structured.Template != "wedding_reel" {
		t.Errorf("template = %q, want wedding_reel", res.Structured.Template)
	}
}

// A pre-load path outside the allowed roots is rejected before the card opens.
func TestOpenStudio_RejectsBadPath(t *testing.T) {
	k, root := testKernel(t)
	h := New(k, root)
	if _, err := h.OpenStudio(context.Background(), contracts.OpenStudioInput{
		Images: []string{"/etc/passwd"},
	}); err == nil {
		t.Error("expected an error for a path outside the allowed roots")
	}
}
