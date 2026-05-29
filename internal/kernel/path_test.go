package kernel

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newTestKernel(t *testing.T, root string) *Kernel {
	t.Helper()
	k, err := New(Config{AllowedRoots: []string{root}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return k
}

// realTempDir returns a symlink-resolved temp dir, so paths returned by
// ValidatePath (which resolves symlinks, e.g. macOS /tmp → /private/tmp) match
// the paths the test constructs.
func realTempDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if real, err := filepath.EvalSymlinks(d); err == nil {
		return real
	}
	return d
}

func TestValidatePath_ReadInsideRoot(t *testing.T) {
	root := realTempDir(t)
	f := filepath.Join(root, "in.jpg")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	k := newTestKernel(t, root)
	got, err := k.ValidatePath(f, ModeRead)
	if err != nil {
		t.Fatalf("ValidatePath: %v", err)
	}
	if got != f {
		t.Errorf("got %q want %q", got, f)
	}
}

func TestValidatePath_TraversalRejected(t *testing.T) {
	root := realTempDir(t)
	k := newTestKernel(t, root)
	// A path that climbs out of the root must be rejected as an escape.
	_, err := k.ValidatePath(filepath.Join(root, "..", "etc", "passwd"), ModeRead)
	if !errors.Is(err, ErrPathEscape) && !errors.Is(err, ErrPathMissing) {
		t.Errorf("expected escape/missing error, got %v", err)
	}
}

func TestValidatePath_AbsoluteOutsideRejected(t *testing.T) {
	root := realTempDir(t)
	k := newTestKernel(t, root)
	_, err := k.ValidatePath("/etc/hosts", ModeRead)
	if !errors.Is(err, ErrPathEscape) {
		t.Errorf("expected ErrPathEscape, got %v", err)
	}
}

func TestValidatePath_MissingRead(t *testing.T) {
	root := realTempDir(t)
	k := newTestKernel(t, root)
	_, err := k.ValidatePath(filepath.Join(root, "nope.jpg"), ModeRead)
	if !errors.Is(err, ErrPathMissing) {
		t.Errorf("expected ErrPathMissing, got %v", err)
	}
}

func TestValidatePath_WriteParentMustExist(t *testing.T) {
	root := realTempDir(t)
	k := newTestKernel(t, root)
	// Writing a new file in an existing root is allowed.
	if _, err := k.ValidatePath(filepath.Join(root, "out.mp4"), ModeWrite); err != nil {
		t.Errorf("write in root should be allowed: %v", err)
	}
	// Writing into a non-existent subdir is rejected.
	if _, err := k.ValidatePath(filepath.Join(root, "missing", "out.mp4"), ModeWrite); !errors.Is(err, ErrPathMissing) {
		t.Errorf("expected ErrPathMissing for missing parent, got %v", err)
	}
}

func TestValidatePath_SymlinkEscapeRejected(t *testing.T) {
	root := realTempDir(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	k := newTestKernel(t, root)
	if _, err := k.ValidatePath(link, ModeRead); !errors.Is(err, ErrPathEscape) {
		t.Errorf("expected symlink escape to be rejected, got %v", err)
	}
}

func TestResolveArtifact(t *testing.T) {
	root := realTempDir(t)
	f := filepath.Join(root, "a.mp4")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	k := newTestKernel(t, root)

	if _, err := k.ResolveArtifact("file://" + f); err != nil {
		t.Errorf("file:// URI should resolve: %v", err)
	}
	if _, err := k.ResolveArtifact("https://example.com/a.mp4"); !errors.Is(err, ErrUnsupportedURI) {
		t.Errorf("expected ErrUnsupportedURI, got %v", err)
	}
}
