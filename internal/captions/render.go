// Package captions renders timed caption overlays as PNG images in pure Go,
// so go-video-mcp can burn titles / lower-thirds into a reel via FFmpeg's
// `overlay` filter — without depending on FFmpeg being built with libfreetype
// (`drawtext`) or libass. The text rasterizer is golang.org/x/image (pure Go,
// CGo-free), which keeps the shipped binary CGo-free.
//
// Each caption is drawn onto a full-canvas transparent PNG at its requested
// vertical position; the compiler overlays it at 0:0 gated by an `enable`
// time window. Drawing the position into the PNG keeps the filtergraph trivial
// (overlay=0:0) and the layout logic here, in testable Go.
package captions

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Position is where a caption sits on the canvas.
type Position string

const (
	PositionTop        Position = "top"
	PositionCenter     Position = "center"
	PositionLowerThird Position = "lower_third"
)

// Spec describes one caption to render.
type Spec struct {
	// Text is the caption text (a single line; newlines are collapsed to spaces).
	Text string
	// Position places the caption vertically.
	Position Position
	// CanvasW, CanvasH are the output frame dimensions the PNG must match.
	CanvasW, CanvasH int
}

// LoadFont reads and parses a TrueType/OpenType font from path. The caller is
// responsible for sourcing path from an allowlist — this never resolves fonts
// itself.
func LoadFont(path string) (*opentype.Font, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path comes from the server's font allowlist
	if err != nil {
		return nil, fmt.Errorf("read font %q: %w", path, err)
	}
	f, err := opentype.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("parse font %q: %w", path, err)
	}
	return f, nil
}

// Render draws one caption onto a full-canvas transparent PNG and returns the
// encoded bytes. The text is white on a semi-transparent dark plate for
// legibility over any imagery.
func Render(f *opentype.Font, s Spec) ([]byte, error) {
	if s.CanvasW <= 0 || s.CanvasH <= 0 {
		return nil, fmt.Errorf("captions: invalid canvas %dx%d", s.CanvasW, s.CanvasH)
	}
	text := strings.TrimSpace(strings.ReplaceAll(s.Text, "\n", " "))
	if text == "" {
		return nil, fmt.Errorf("captions: empty text")
	}

	// Font size scales with canvas height; clamp to a legible minimum.
	size := float64(s.CanvasH) / 20.0
	if size < 20 {
		size = 20
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("captions: new face: %w", err)
	}
	defer face.Close()

	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	lineH := ascent + descent
	textW := font.MeasureString(face, text).Ceil()

	pad := int(size * 0.5)
	boxW := textW + 2*pad
	boxH := lineH + 2*pad
	if boxW > s.CanvasW {
		boxW = s.CanvasW
	}

	boxX := (s.CanvasW - boxW) / 2
	boxY := verticalY(s.Position, s.CanvasH, boxH)

	img := image.NewRGBA(image.Rect(0, 0, s.CanvasW, s.CanvasH))
	// Semi-transparent dark plate behind the text.
	plate := color.RGBA{R: 0x0b, G: 0x0c, B: 0x0e, A: 0xb0}
	draw.Draw(img, image.Rect(boxX, boxY, boxX+boxW, boxY+boxH), &image.Uniform{C: plate}, image.Point{}, draw.Over)

	// White text, baseline-aligned inside the plate, horizontally centred.
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{R: 0xf4, G: 0xf1, B: 0xea, A: 0xff}),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(boxX + (boxW-textW)/2),
			Y: fixed.I(boxY + pad + ascent),
		},
	}
	drawer.DrawString(text)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("captions: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// verticalY returns the top Y for a box of height boxH at the given position,
// clamped inside the canvas.
func verticalY(p Position, canvasH, boxH int) int {
	var y int
	switch p {
	case PositionTop:
		y = int(float64(canvasH) * 0.08)
	case PositionCenter:
		y = (canvasH - boxH) / 2
	default: // lower_third
		y = int(float64(canvasH) * 0.80)
	}
	if y < 0 {
		y = 0
	}
	if y+boxH > canvasH {
		y = canvasH - boxH
	}
	return y
}
