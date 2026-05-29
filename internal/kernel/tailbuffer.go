package kernel

// tailBuffer is an io.Writer that keeps only the last `max` bytes written. It
// captures FFmpeg's stderr so a failed run can surface the tail of the log
// (where the actual error is) without retaining an unbounded buffer for a job
// that logs progress for an hour.
type tailBuffer struct {
	max int
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.buf) }
