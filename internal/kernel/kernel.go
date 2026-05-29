// Package kernel is ffmpeg.kernel — the single place in go-video-mcp that
// touches FFmpeg. Tools never shell out themselves and never build a shell
// string; they describe work as a typed Plan and hand it to the kernel, which
// validates paths, renders an argv slice, runs FFmpeg as a child process with a
// cancellable context, and parses progress. Keeping every external-process
// concern here is what makes the tool handlers small and the safety rules
// (CLAUDE.md §5) enforceable in one audited spot.
package kernel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config configures a Kernel. The zero value is not usable — use New.
type Config struct {
	// FFmpegPath is the ffmpeg binary (default "ffmpeg", resolved on PATH).
	FFmpegPath string
	// FFprobePath is the ffprobe binary (default "ffprobe").
	FFprobePath string
	// AllowedRoots confines every read and write. A path outside all roots is
	// rejected by ValidatePath. Defaults to the process working directory.
	AllowedRoots []string
	// Timeout bounds a single RunPlan / Probe invocation. Zero means the
	// DefaultTimeout.
	Timeout time.Duration
}

// DefaultTimeout caps a single FFmpeg/ffprobe invocation when Config.Timeout is
// unset. Rendering a long slideshow can be slow; this is a backstop, not a
// target.
const DefaultTimeout = 30 * time.Minute

// Kernel runs FFmpeg work safely. It is safe for concurrent use.
type Kernel struct {
	cfg Config

	mu   sync.Mutex
	jobs map[string]context.CancelFunc
}

// New constructs a Kernel, filling defaults. At least one allowed root is always
// set (the working directory when none is given) so paths are never unconfined
// in production.
func New(cfg Config) (*Kernel, error) {
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = "ffmpeg"
	}
	if cfg.FFprobePath == "" {
		cfg.FFprobePath = "ffprobe"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if len(cfg.AllowedRoots) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("kernel: determine working directory: %w", err)
		}
		cfg.AllowedRoots = []string{wd}
	}
	// Resolve roots through symlinks so comparisons against symlink-resolved
	// candidate paths are apples-to-apples (e.g. macOS /tmp → /private/tmp).
	for i, r := range cfg.AllowedRoots {
		abs, err := filepath.Abs(r)
		if err != nil {
			return nil, fmt.Errorf("kernel: resolve allowed root %q: %w", r, err)
		}
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			abs = real
		}
		cfg.AllowedRoots[i] = filepath.Clean(abs)
	}
	return &Kernel{cfg: cfg, jobs: make(map[string]context.CancelFunc)}, nil
}

// MediaInfo is the typed result of probing a media file — the facts tools need
// without re-parsing ffprobe themselves.
type MediaInfo struct {
	Path        string  `json:"path"`
	FormatName  string  `json:"format_name"`
	DurationSec float64 `json:"duration_sec"`
	SizeBytes   int64   `json:"size_bytes"`
	BitRate     int64   `json:"bit_rate"`
	HasVideo    bool    `json:"has_video"`
	HasAudio    bool    `json:"has_audio"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FPS         float64 `json:"fps"`
	VideoCodec  string  `json:"video_codec"`
	AudioCodec  string  `json:"audio_codec"`
	Streams     int     `json:"streams"`
}

// Probe runs ffprobe against an already-validated path and returns typed facts.
func (k *Kernel) Probe(ctx context.Context, path string) (MediaInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, k.cfg.Timeout)
	defer cancel()

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	}
	out, err := exec.CommandContext(ctx, k.cfg.FFprobePath, args...).Output()
	if err != nil {
		return MediaInfo{}, fmt.Errorf("ffprobe %q: %w", path, runErr(err))
	}
	return parseProbe(path, out)
}

// ffprobeReport is the subset of ffprobe's JSON we consume.
type ffprobeReport struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
	} `json:"streams"`
}

func parseProbe(path string, raw []byte) (MediaInfo, error) {
	var rep ffprobeReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return MediaInfo{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	mi := MediaInfo{
		Path:        path,
		FormatName:  rep.Format.FormatName,
		DurationSec: atof(rep.Format.Duration),
		SizeBytes:   atoi(rep.Format.Size),
		BitRate:     atoi(rep.Format.BitRate),
		Streams:     len(rep.Streams),
	}
	for _, s := range rep.Streams {
		switch s.CodecType {
		case "video":
			// Skip cover-art / attached-pic streams that report 0 dims.
			if s.Width == 0 && s.Height == 0 {
				continue
			}
			mi.HasVideo = true
			mi.Width, mi.Height = s.Width, s.Height
			mi.VideoCodec = s.CodecName
			if r := parseRate(s.AvgFrameRate); r > 0 {
				mi.FPS = r
			} else {
				mi.FPS = parseRate(s.RFrameRate)
			}
		case "audio":
			mi.HasAudio = true
			mi.AudioCodec = s.CodecName
		}
	}
	return mi, nil
}

// RunResult reports the outcome of a RunPlan call.
type RunResult struct {
	// JobID is the crypto-strong id assigned to the run (also usable with Cancel
	// while it is in flight).
	JobID string
	// Output is the produced file path.
	Output string
	// Args is the rendered FFmpeg argv (for the storyboard / debugging).
	Args []string
	// Command is the human-readable, shell-quoted command line (never executed).
	Command string
}

// RunPlan executes a Plan: it assigns a job id, runs FFmpeg as a child process
// bound to a cancellable, timeout-scoped context, streams `-progress` to fn (if
// non-nil), and returns when FFmpeg exits. On a non-zero exit it returns an
// error wrapping FFmpeg's stderr tail. The Plan's paths are trusted — validate
// them before building the Plan.
func (k *Kernel) RunPlan(ctx context.Context, p Plan, fn ProgressFunc) (RunResult, error) {
	jobID, err := newJobID()
	if err != nil {
		return RunResult{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, k.cfg.Timeout)
	defer cancel()
	k.register(jobID, cancel)
	defer k.unregister(jobID)

	// Stream machine-readable progress to stdout so we never have to scrape the
	// human stderr log for timing.
	args := append([]string{"-progress", "pipe:1", "-nostats"}, p.ToArgs()...)
	cmd := exec.CommandContext(ctx, k.cfg.FFmpegPath, args...)

	res := RunResult{JobID: jobID, Output: p.Output, Args: args, Command: p.String()}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return res, fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	stderr := &tailBuffer{max: 8 * 1024}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return res, fmt.Errorf("start ffmpeg: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		parseProgress(stdout, fn)
	}()
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.Canceled {
			return res, fmt.Errorf("ffmpeg cancelled (job %s)", jobID)
		}
		if ctx.Err() == context.DeadlineExceeded {
			return res, fmt.Errorf("ffmpeg timed out after %s (job %s)", k.cfg.Timeout, jobID)
		}
		return res, fmt.Errorf("ffmpeg failed (job %s): %w\n%s", jobID, err, stderr.String())
	}
	return res, nil
}

// Cancel stops an in-flight job by id. It returns false if no such job is
// running (already finished or never existed).
func (k *Kernel) Cancel(jobID string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	cancel, ok := k.jobs[jobID]
	if ok {
		cancel()
		delete(k.jobs, jobID)
	}
	return ok
}

func (k *Kernel) register(id string, cancel context.CancelFunc) {
	k.mu.Lock()
	k.jobs[id] = cancel
	k.mu.Unlock()
}

func (k *Kernel) unregister(id string) {
	k.mu.Lock()
	delete(k.jobs, id)
	k.mu.Unlock()
}

// newJobID returns a 128-bit crypto-strong hex id.
func newJobID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate job id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// runErr unwraps an *exec.ExitError to include stderr when present.
func runErr(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, string(ee.Stderr))
	}
	return err
}

func atof(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
func atoi(s string) int64   { i, _ := strconv.ParseInt(s, 10, 64); return i }

// parseRate parses an ffprobe rational rate like "30000/1001" or "25/1".
func parseRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return atof(s)
	}
	d := atof(den)
	if d == 0 {
		return 0
	}
	return atof(num) / d
}
