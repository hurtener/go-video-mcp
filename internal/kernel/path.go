package kernel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Errors returned by path validation. Callers can branch with errors.Is.
var (
	// ErrPathEscape means the resolved path is outside every allowed root.
	ErrPathEscape = errors.New("path is outside the allowed roots")
	// ErrPathMissing means a read path does not exist.
	ErrPathMissing = errors.New("path does not exist")
	// ErrNotRegular means the path exists but is not a regular file.
	ErrNotRegular = errors.New("path is not a regular file")
	// ErrUnsupportedURI means ResolveArtifact was given a scheme it cannot
	// resolve locally (e.g. http://). V1 resolves local files only.
	ErrUnsupportedURI = errors.New("unsupported artifact URI scheme")
)

// PathMode selects the validation rules for a path.
type PathMode int

const (
	// ModeRead requires the path to exist and be a regular file.
	ModeRead PathMode = iota
	// ModeWrite requires the parent directory to exist; the file itself need
	// not exist yet.
	ModeWrite
)

// ValidatePath resolves p to an absolute, symlink-free path confined to one of
// the kernel's allowed roots, applying the rules for the given mode. It is the
// single chokepoint every user-influenced path passes through before it can
// reach FFmpeg. It defends against `..` traversal, absolute-path escapes, and
// symlinks that point outside an allowed root.
//
// It returns the cleaned absolute path to use.
func (k *Kernel) ValidatePath(p string, mode PathMode) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", fmt.Errorf("%w: empty path", ErrPathMissing)
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", p, err)
	}
	abs = filepath.Clean(abs)

	// Resolve symlinks on the part of the path that exists, so a symlink cannot
	// smuggle the real target outside an allowed root. For a write target whose
	// file does not exist yet, resolve the deepest existing ancestor.
	resolved, err := resolveExisting(abs)
	if err != nil {
		return "", err
	}

	if !k.withinAllowedRoot(resolved) {
		return "", fmt.Errorf("%w: %s", ErrPathEscape, abs)
	}

	switch mode {
	case ModeRead:
		info, err := os.Stat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("%w: %s", ErrPathMissing, abs)
			}
			return "", fmt.Errorf("stat %q: %w", abs, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("%w: %s", ErrNotRegular, abs)
		}
	case ModeWrite:
		parent := filepath.Dir(resolved)
		info, err := os.Stat(parent)
		if err != nil {
			return "", fmt.Errorf("%w: output directory %s", ErrPathMissing, parent)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("output parent %s is not a directory", parent)
		}
	}

	return resolved, nil
}

// resolveExisting evaluates symlinks on the longest existing prefix of abs and
// re-appends the non-existent tail. This lets a write target that does not yet
// exist still be confined to an allowed root via its existing parent.
func resolveExisting(abs string) (string, error) {
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real, nil
	}
	// Walk up to the deepest existing ancestor, peeling non-existent components
	// onto `tail` (deepest first) so they can be re-appended to the resolved
	// ancestor afterwards.
	dir := abs
	var tail []string
	for {
		tail = append(tail, filepath.Base(dir))
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root; nothing resolved — return clean abs.
			return abs, nil
		}
		if real, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{real}, reverse(tail)...)
			return filepath.Join(parts...), nil
		}
		dir = parent
	}
}

func reverse(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}

// withinAllowedRoot reports whether resolved is inside any configured allowed
// root. An empty root set means "no restriction" — only used in tests; the
// constructed kernel always has at least one root.
func (k *Kernel) withinAllowedRoot(resolved string) bool {
	if len(k.cfg.AllowedRoots) == 0 {
		return true
	}
	for _, root := range k.cfg.AllowedRoots {
		root = filepath.Clean(root)
		if resolved == root {
			return true
		}
		if strings.HasPrefix(resolved, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// ResolveArtifact turns a user-supplied artifact reference (a plain path or a
// file:// URI) into a concrete local path, then validates it for reading. V1
// resolves local artifacts only; a remote scheme is a clear, explained error
// rather than a silent fetch.
func (k *Kernel) ResolveArtifact(uri string) (string, error) {
	p := strings.TrimSpace(uri)
	switch {
	case p == "":
		return "", fmt.Errorf("%w: empty artifact reference", ErrPathMissing)
	case strings.HasPrefix(p, "file://"):
		p = strings.TrimPrefix(p, "file://")
	case strings.Contains(p, "://"):
		scheme := p[:strings.Index(p, "://")]
		return "", fmt.Errorf("%w: %s", ErrUnsupportedURI, scheme)
	}
	return k.ValidatePath(p, ModeRead)
}
