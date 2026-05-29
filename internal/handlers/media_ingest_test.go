package handlers

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/hurtener/go-video-mcp/internal/contracts"
	"github.com/hurtener/go-video-mcp/internal/kernel"
)

func testKernel(t *testing.T) (*kernel.Kernel, string) {
	t.Helper()
	root := t.TempDir()
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	k, err := kernel.New(kernel.Config{AllowedRoots: []string{root}})
	if err != nil {
		t.Fatalf("kernel.New: %v", err)
	}
	return k, root
}

func TestListMedia_FindsAndClassifies(t *testing.T) {
	k, root := testKernel(t)
	write(t, filepath.Join(root, "a.jpg"))
	write(t, filepath.Join(root, "b.mp3"))
	write(t, filepath.Join(root, "notes.txt")) // ignored — not media
	write(t, filepath.Join(root, "sub", "c.mov"))
	write(t, filepath.Join(root, "node_modules", "skip.png")) // skipped dir

	h := New(k, root)
	res, err := h.ListMedia(context.Background(), contracts.ListMediaInput{})
	if err != nil {
		t.Fatalf("ListMedia: %v", err)
	}
	kinds := map[string]string{}
	for _, it := range res.Structured.Items {
		kinds[it.Name] = it.Kind
	}
	if kinds["a.jpg"] != "image" || kinds["b.mp3"] != "audio" || kinds["c.mov"] != "video" {
		t.Errorf("misclassified: %+v", kinds)
	}
	if _, ok := kinds["notes.txt"]; ok {
		t.Error("non-media file should not be listed")
	}
	if _, ok := kinds["skip.png"]; ok {
		t.Error("node_modules should be skipped")
	}
}

func TestListMedia_KindFilter(t *testing.T) {
	k, root := testKernel(t)
	write(t, filepath.Join(root, "a.jpg"))
	write(t, filepath.Join(root, "b.mp3"))
	h := New(k, root)
	res, err := h.ListMedia(context.Background(), contracts.ListMediaInput{Kinds: []string{"audio"}})
	if err != nil {
		t.Fatalf("ListMedia: %v", err)
	}
	if len(res.Structured.Items) != 1 || res.Structured.Items[0].Kind != "audio" {
		t.Errorf("kind filter failed: %+v", res.Structured.Items)
	}
}

func TestIngestMedia_WritesIntoWorkDir(t *testing.T) {
	k, root := testKernel(t)
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	h := New(k, work)

	data := base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfake"))
	res, err := h.IngestMedia(context.Background(), contracts.IngestMediaInput{
		Filename: "photo.png", DataBase64: data,
	})
	if err != nil {
		t.Fatalf("IngestMedia: %v", err)
	}
	if res.Structured.Kind != "image" {
		t.Errorf("kind = %q, want image", res.Structured.Kind)
	}
	if filepath.Dir(res.Structured.Path) != work {
		t.Errorf("wrote outside work dir: %s", res.Structured.Path)
	}
	if _, err := os.Stat(res.Structured.Path); err != nil {
		t.Errorf("file not written: %v", err)
	}
}

func TestIngestMedia_RejectsTraversalAndNonMedia(t *testing.T) {
	k, root := testKernel(t)
	work := filepath.Join(root, "work")
	_ = os.MkdirAll(work, 0o755)
	h := New(k, work)
	data := base64.StdEncoding.EncodeToString([]byte("x"))

	// A traversal filename is reduced to its base name, so it can't escape.
	res, err := h.IngestMedia(context.Background(), contracts.IngestMediaInput{
		Filename: "../../etc/evil.png", DataBase64: data,
	})
	if err != nil {
		t.Fatalf("IngestMedia: %v", err)
	}
	if filepath.Dir(res.Structured.Path) != work || filepath.Base(res.Structured.Path) != "evil.png" {
		t.Errorf("traversal not contained: %s", res.Structured.Path)
	}

	// A non-media extension is rejected.
	if _, err := h.IngestMedia(context.Background(), contracts.IngestMediaInput{
		Filename: "script.sh", DataBase64: data,
	}); err == nil {
		t.Error("expected non-media extension to be rejected")
	}
}

func TestIngestMedia_Deduplicates(t *testing.T) {
	k, root := testKernel(t)
	work := filepath.Join(root, "work")
	_ = os.MkdirAll(work, 0o755)
	h := New(k, work)
	data := base64.StdEncoding.EncodeToString([]byte("x"))

	r1, _ := h.IngestMedia(context.Background(), contracts.IngestMediaInput{Filename: "p.jpg", DataBase64: data})
	r2, _ := h.IngestMedia(context.Background(), contracts.IngestMediaInput{Filename: "p.jpg", DataBase64: data})
	if r1.Structured.Path == r2.Structured.Path {
		t.Errorf("duplicate names should not collide: %s", r1.Structured.Path)
	}
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
