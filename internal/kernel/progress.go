package kernel

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// Progress is a single observation of a running FFmpeg job, derived from the
// `-progress` key=value stream. It is intentionally small — shape and timing,
// never raw frame data.
type Progress struct {
	// Frame is the number of frames processed so far.
	Frame int64
	// FPS is the current processing rate.
	FPS float64
	// OutTimeMS is the output timestamp in milliseconds — the basis for a
	// percentage when the total duration is known.
	OutTimeMS int64
	// Speed is the processing speed relative to realtime (e.g. 2.5 = 2.5x).
	Speed float64
	// Done is true on the terminal `progress=end` record.
	Done bool
}

// ProgressFunc receives each Progress update. It must not block — the kernel
// calls it on the goroutine draining FFmpeg's progress pipe.
type ProgressFunc func(Progress)

// parseProgress reads FFmpeg's `-progress` output (key=value lines, one metric
// per line, each block terminated by a `progress=continue|end` line) from r and
// invokes fn once per completed block. It returns when r is exhausted.
//
// FFmpeg's `-progress pipe:1` format is line-oriented and stable; we parse only
// the handful of keys we surface and ignore the rest, so a new upstream key
// never breaks us.
func parseProgress(r io.Reader, fn ProgressFunc) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var cur Progress
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		switch key {
		case "frame":
			cur.Frame, _ = strconv.ParseInt(val, 10, 64)
		case "fps":
			cur.FPS, _ = strconv.ParseFloat(val, 64)
		case "out_time_ms", "out_time_us":
			// FFmpeg has shipped this key as microseconds under the
			// out_time_ms name historically; treat the larger-resolution
			// value uniformly and store milliseconds.
			us, _ := strconv.ParseInt(val, 10, 64)
			cur.OutTimeMS = us / 1000
		case "speed":
			cur.Speed = parseSpeed(val)
		case "progress":
			cur.Done = val == "end"
			if fn != nil {
				fn(cur)
			}
			cur = Progress{}
		}
	}
}

// parseSpeed parses FFmpeg's speed field, which arrives like "2.53x" or "N/A".
func parseSpeed(v string) float64 {
	v = strings.TrimSuffix(strings.TrimSpace(v), "x")
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}
