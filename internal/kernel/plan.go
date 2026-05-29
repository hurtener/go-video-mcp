package kernel

import (
	"fmt"
	"strconv"
	"strings"
)

// Input is one FFmpeg input (`-i`) plus the pre-input options that must precede
// it on the command line (e.g. `-loop 1 -t 4` for a still image held for four
// seconds). Pre-input options are positional in FFmpeg — they apply to the next
// `-i` — so they live on the Input, not in a flat option slice.
type Input struct {
	// Path is the input file. It MUST already have passed ValidatePath; ToArgs
	// does no validation of its own.
	Path string
	// Loop, when true, emits `-loop 1` so a single image is decoded as an
	// endless stream (paired with Duration to bound it).
	Loop bool
	// Duration, when > 0, emits `-t <dur>` before the input — the on-disk read
	// duration. For a looped still this is how long the image is held.
	Duration float64
	// Pre holds any extra pre-input options (each already split into argv
	// tokens), inserted immediately before `-i`.
	Pre []string
}

func (in Input) args() []string {
	var a []string
	if in.Loop {
		a = append(a, "-loop", "1")
	}
	if in.Duration > 0 {
		a = append(a, "-t", ftoa(in.Duration))
	}
	a = append(a, in.Pre...)
	a = append(a, "-i", in.Path)
	return a
}

// FilterChain is a single chain in an FFmpeg filtergraph: zero or more input
// pad labels, a comma-separated list of filters, and zero or more output pad
// labels. Its String form is `[in0][in1]filter1,filter2[out0]` — the unit
// FFmpeg separates with commas within a chain.
type FilterChain struct {
	Inputs  []string // pad labels WITHOUT brackets, e.g. "0:v", "v1"
	Filters []string // each one filter, e.g. "scale=1920:1080"
	Outputs []string // pad labels WITHOUT brackets, e.g. "v0"
}

// String renders the chain in filtergraph syntax.
func (c FilterChain) String() string {
	var b strings.Builder
	for _, in := range c.Inputs {
		b.WriteString("[")
		b.WriteString(in)
		b.WriteString("]")
	}
	b.WriteString(strings.Join(c.Filters, ","))
	for _, out := range c.Outputs {
		b.WriteString("[")
		b.WriteString(out)
		b.WriteString("]")
	}
	return b.String()
}

// FilterGraph is a complete `-filter_complex` graph: chains separated by
// semicolons. This is the structured form the slideshow compiler produces;
// keeping it typed (rather than string-concatenated everywhere) is what makes
// the compiler golden-testable.
type FilterGraph struct {
	Chains []FilterChain
}

// Add appends a chain and returns the graph for fluent construction.
func (g *FilterGraph) Add(c FilterChain) *FilterGraph {
	g.Chains = append(g.Chains, c)
	return g
}

// String renders the whole graph: chains joined by ";".
func (g *FilterGraph) String() string {
	parts := make([]string, len(g.Chains))
	for i, c := range g.Chains {
		parts[i] = c.String()
	}
	return strings.Join(parts, ";")
}

// Plan is a fully structured, ready-to-execute FFmpeg invocation. It is the one
// thing RunPlan accepts. Every path on it MUST already have passed ValidatePath
// — the Plan is trusted by the time it reaches RunPlan. ToArgs renders it to an
// argv slice; FFmpeg is never invoked through a shell.
type Plan struct {
	// Global holds global options that precede all inputs (e.g. "-hide_banner").
	Global []string
	// Inputs are the ordered `-i` inputs.
	Inputs []Input
	// Graph, when non-nil, emits `-filter_complex <graph>`.
	Graph *FilterGraph
	// Maps are output stream selectors; each becomes `-map <value>`
	// (e.g. "[vout]", "0:a").
	Maps []string
	// Out holds output options (codec, pixfmt, quality), in order.
	Out []string
	// Duration, when > 0, caps the output with `-t <dur>`.
	Duration float64
	// Output is the destination file.
	Output string
	// Overwrite emits `-y` (overwrite without prompting) when true.
	Overwrite bool
}

// ToArgs renders the Plan to an FFmpeg argv slice (excluding the binary name).
// The order is the canonical FFmpeg order: global options, per-input options +
// inputs, filter_complex, maps, output options, then the output file.
func (p Plan) ToArgs() []string {
	var a []string
	a = append(a, "-hide_banner")
	if p.Overwrite {
		a = append(a, "-y")
	}
	a = append(a, p.Global...)
	for _, in := range p.Inputs {
		a = append(a, in.args()...)
	}
	if p.Graph != nil && len(p.Graph.Chains) > 0 {
		a = append(a, "-filter_complex", p.Graph.String())
	}
	for _, m := range p.Maps {
		a = append(a, "-map", m)
	}
	a = append(a, p.Out...)
	if p.Duration > 0 {
		a = append(a, "-t", ftoa(p.Duration))
	}
	a = append(a, p.Output)
	return a
}

// String renders the argv as a human-readable, shell-quoted line — for logs,
// errors, and the storyboard artifact. It is NOT used to execute anything.
func (p Plan) String() string {
	parts := append([]string{"ffmpeg"}, p.ToArgs()...)
	for i, s := range parts {
		if strings.ContainsAny(s, " \t'\"\\;|&$()[]") {
			parts[i] = strconv.Quote(s)
		}
	}
	return strings.Join(parts, " ")
}

// ftoa formats a float without a trailing ".000000" — FFmpeg accepts plain
// decimals and short forms keep the rendered graph readable in goldens.
func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// label is a tiny helper for building bracketed pad references in callers that
// want a string like "[v0]".
func label(name string) string { return fmt.Sprintf("[%s]", name) }
