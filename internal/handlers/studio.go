package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/hurtener/dockyard/runtime/tool"

	"github.com/hurtener/go-video-mcp/internal/contracts"
)

// OpenStudio opens the Frameline Studio composer card. It is a lightweight App
// entry point — "show me the studio" — so the UI can render even before any
// reel exists. Any pre-load images are validated (so the card never seeds a
// path the render tool would later reject); the App reads Kind to pick its
// composer view.
func (h *Handlers) OpenStudio(ctx context.Context, in contracts.OpenStudioInput) (tool.Result[contracts.OpenStudioOutput], error) {
	var images []string
	for _, img := range in.Images {
		v, err := h.K.ResolveArtifact(img)
		if err != nil {
			return tool.Result[contracts.OpenStudioOutput]{}, fmt.Errorf("open_studio: image %q: %w", img, err)
		}
		images = append(images, v)
	}

	msg := "Frameline Studio is open — drop stills, set the look, and render a reel."
	if len(images) > 0 {
		msg = fmt.Sprintf("Frameline Studio is open with %d image(s) loaded.", len(images))
	}
	out := contracts.OpenStudioOutput{
		Kind:     "studio",
		Message:  msg,
		Images:   images,
		Template: in.Template,
	}
	return tool.Result[contracts.OpenStudioOutput]{Text: msg, Structured: out}, nil
}

// OpenMediaUploader opens the Media Uploader card — the dedicated drop-zone for
// getting photos and music onto the server. The card ingests dropped files via
// ingest_media; this tool just renders it and reports where files will land.
func (h *Handlers) OpenMediaUploader(ctx context.Context, in contracts.OpenMediaUploaderInput) (tool.Result[contracts.OpenMediaUploaderOutput], error) {
	out := contracts.OpenMediaUploaderOutput{
		Kind:  "media_uploader",
		Note:  strings.TrimSpace(in.Note),
		Roots: h.K.Roots(),
	}
	text := "Media Uploader is open — drop photos and music to add them."
	if out.Note != "" {
		text = out.Note
	}
	return tool.Result[contracts.OpenMediaUploaderOutput]{Text: text, Structured: out}, nil
}
