package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hurtener/dockyard/runtime/tool"

	"github.com/hurtener/go-video-mcp/internal/contracts"
	"github.com/hurtener/go-video-mcp/internal/kernel"
)

// maxListItems caps a media listing so a large root can't produce an unbounded
// response.
const maxListItems = 1000

// maxIngestBytes caps a single uploaded file (64 MiB) — large enough for a
// full-resolution photo or a song, small enough to bound memory.
const maxIngestBytes = 64 << 20

// skipDirs are directories never worth walking for media.
var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, ".frameline-work": true,
}

// mediaKind classifies a file extension into "image" | "audio" | "video" | "".
func mediaKind(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".webp", ".heic", ".heif", ".bmp", ".tiff", ".tif", ".gif":
		return "image"
	case ".mp3", ".m4a", ".aac", ".wav", ".flac", ".ogg", ".opus":
		return "audio"
	case ".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v":
		return "video"
	default:
		return ""
	}
}

// ListMedia browses media files under the allowed roots (or a given subdir).
func (h *Handlers) ListMedia(_ context.Context, in contracts.ListMediaInput) (tool.Result[contracts.ListMediaOutput], error) {
	fail := func(err error) (tool.Result[contracts.ListMediaOutput], error) {
		return tool.Result[contracts.ListMediaOutput]{}, fmt.Errorf("list_media: %w", err)
	}

	var roots []string
	if strings.TrimSpace(in.Dir) != "" {
		d, err := h.K.ValidateDir(in.Dir)
		if err != nil {
			return fail(err)
		}
		roots = []string{d}
	} else {
		roots = h.K.Roots()
	}

	want := kindFilter(in.Kinds)
	var items []contracts.MediaItem
	truncated := false

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries rather than abort the walk
			}
			name := d.Name()
			if d.IsDir() {
				if path != root && (strings.HasPrefix(name, ".") || skipDirs[name]) {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(name, ".") {
				return nil
			}
			ext := filepath.Ext(name)
			kind := mediaKind(ext)
			if kind == "" || (want != nil && !want[kind]) {
				return nil
			}
			if len(items) >= maxListItems {
				truncated = true
				return fs.SkipAll
			}
			info, e := d.Info()
			var size int64
			if e == nil {
				size = info.Size()
			}
			items = append(items, contracts.MediaItem{
				Path: path, Name: name, Kind: kind, Ext: strings.ToLower(ext), SizeBytes: size,
			})
			return nil
		})
		if err != nil {
			return fail(err)
		}
		if truncated {
			break
		}
	}

	out := contracts.ListMediaOutput{Items: items, Roots: roots, Truncated: truncated}
	return tool.Result[contracts.ListMediaOutput]{
		Text:       fmt.Sprintf("Found %d media file(s) across %d root(s)%s", len(items), len(roots), truncatedNote(truncated)),
		Structured: out,
	}, nil
}

// IngestMedia persists a base64-uploaded file into the work directory and
// returns a path other tools can read.
func (h *Handlers) IngestMedia(_ context.Context, in contracts.IngestMediaInput) (tool.Result[contracts.IngestMediaOutput], error) {
	fail := func(err error) (tool.Result[contracts.IngestMediaOutput], error) {
		return tool.Result[contracts.IngestMediaOutput]{}, fmt.Errorf("ingest_media: %w", err)
	}

	if h.WorkDir == "" {
		return fail(fmt.Errorf("no work directory configured for uploads"))
	}
	name := filepath.Base(strings.TrimSpace(in.Filename))
	if name == "" || name == "." || name == string(os.PathSeparator) {
		return fail(fmt.Errorf("invalid filename %q", in.Filename))
	}
	if mediaKind(filepath.Ext(name)) == "" {
		return fail(fmt.Errorf("unsupported media type %q", filepath.Ext(name)))
	}

	// Decode with a hard size cap to bound memory.
	if len(in.DataBase64) == 0 {
		return fail(fmt.Errorf("empty upload data"))
	}
	data, err := base64.StdEncoding.DecodeString(in.DataBase64)
	if err != nil {
		return fail(fmt.Errorf("decode base64: %w", err))
	}
	if len(data) > maxIngestBytes {
		return fail(fmt.Errorf("file too large: %d bytes (max %d)", len(data), maxIngestBytes))
	}

	dst := uniquePath(h.WorkDir, name)
	// Confirm the destination is inside an allowed root before writing.
	validated, err := h.K.ValidatePath(dst, kernel.ModeWrite)
	if err != nil {
		return fail(err)
	}
	if err := os.WriteFile(validated, data, 0o644); err != nil {
		return fail(fmt.Errorf("write upload: %w", err))
	}

	out := contracts.IngestMediaOutput{
		Path:      validated,
		Name:      filepath.Base(validated),
		Kind:      mediaKind(filepath.Ext(validated)),
		SizeBytes: int64(len(data)),
	}
	return tool.Result[contracts.IngestMediaOutput]{
		Text:       fmt.Sprintf("Ingested %s (%s, %d bytes)", out.Name, out.Kind, out.SizeBytes),
		Structured: out,
	}, nil
}

// maxReadBytes caps a read_media inline payload (~25 MiB). Larger files report
// Truncated so the UI falls back to a poster/path rather than shipping a huge
// base64 blob through the MCP pipe.
const maxReadBytes = 25 << 20

// ReadMedia returns a media file under the allowed roots as a data URI, so a
// sandboxed UI can play/display it. Media-only and size-capped.
func (h *Handlers) ReadMedia(_ context.Context, in contracts.ReadMediaInput) (tool.Result[contracts.ReadMediaOutput], error) {
	fail := func(err error) (tool.Result[contracts.ReadMediaOutput], error) {
		return tool.Result[contracts.ReadMediaOutput]{}, fmt.Errorf("read_media: %w", err)
	}
	path, err := h.K.ValidatePath(in.Path, kernel.ModeRead)
	if err != nil {
		return fail(err)
	}
	mime := mimeFor(path)
	if mime == "" {
		return fail(fmt.Errorf("unsupported media type %q", filepath.Ext(path)))
	}
	info, err := os.Stat(path)
	if err != nil {
		return fail(err)
	}
	if info.Size() > maxReadBytes {
		out := contracts.ReadMediaOutput{Mime: mime, SizeBytes: info.Size(), Truncated: true}
		return tool.Result[contracts.ReadMediaOutput]{
			Text:       fmt.Sprintf("%s is %d bytes — too large to inline (cap %d); use the path directly", filepath.Base(path), info.Size(), maxReadBytes),
			Structured: out,
		}, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // path validated + confined to allowed roots
	if err != nil {
		return fail(err)
	}
	out := contracts.ReadMediaOutput{
		DataURI:   "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
		Mime:      mime,
		SizeBytes: info.Size(),
	}
	return tool.Result[contracts.ReadMediaOutput]{
		Text:       fmt.Sprintf("Read %s (%s, %d bytes)", filepath.Base(path), mime, info.Size()),
		Structured: out,
	}, nil
}

// mimeFor maps a file extension to a MIME type, or "" for non-media.
func mimeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".ogg", ".opus":
		return "audio/ogg"
	default:
		return ""
	}
}

// kindFilter builds a set from the requested kinds, or nil for "all".
func kindFilter(kinds []string) map[string]bool {
	if len(kinds) == 0 {
		return nil
	}
	m := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		if k = strings.ToLower(strings.TrimSpace(k)); k != "" {
			m[k] = true
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// uniquePath returns dir/name, appending "-1", "-2", … before the extension if
// the file already exists, so concurrent uploads of same-named files don't
// clobber each other.
func uniquePath(dir, name string) string {
	candidate := filepath.Join(dir, name)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func truncatedNote(t bool) string {
	if t {
		return " (truncated — more files exist)"
	}
	return ""
}
