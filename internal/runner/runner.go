package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thebrazenbeard/workbridge/internal/config"
	"github.com/thebrazenbeard/workbridge/internal/policy"
)

type grant struct {
	path   string
	sha256 string
}

type Runner struct {
	enabled bool
	grants  map[string]grant
	roots   *policy.RootPolicy
	timeout time.Duration
	maxOut  int64
	maxArgs int
	slots   chan struct{}
}

type Result struct {
	ExitCode        int    `json:"exit_code"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	OutputTruncated bool   `json:"output_truncated"`
	TimedOut        bool   `json:"timed_out"`
	DurationMS      int64  `json:"duration_ms"`
}

func New(cfg config.ProcessConfig) (*Runner, error) {
	roots, err := policy.NewRootPolicy(cfg.WorkingRoots)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Runner, error) {
		_ = roots.Close()
		return nil, err
	}
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent == 0 {
		maxConcurrent = 4
	}
	r := &Runner{
		enabled: cfg.Enabled,
		grants: make(map[string]grant),
		roots: roots,
		timeout: time.Duration(cfg.MaxRuntimeSeconds * float64(time.Second)),
		maxOut: cfg.MaxOutputBytes,
		maxArgs: cfg.MaxArgs,
		slots: make(chan struct{}, maxConcurrent),
	}
	if !cfg.Enabled {
		return r, nil
	}
	for _, item := range cfg.AllowedExecutables {
		resolved, err := filepath.EvalSymlinks(filepath.Clean(item.Path))
		if err != nil {
			return fail(fmt.Errorf("resolve executable %q: %w", item.Name, err))
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return fail(err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return fail(fmt.Errorf("stat executable %q: %w", item.Name, err))
		}
		if !info.Mode().IsRegular() {
			return fail(fmt.Errorf("executable %q is not a regular file", item.Name))
		}
		digest, err := hashFile(resolved)
		if err != nil {
			return fail(err)
		}
		if digest != item.SHA256 {
			return fail(fmt.Errorf("executable %q sha256 mismatch", item.Name))
		}
		r.grants[item.Name] = grant{path: filepath.Clean(resolved), sha256: digest}
	}
	return r, nil
}

func (r *Runner) Close() error {
	if r == nil || r.roots == nil {
		return nil
	}
	return r.roots.Close()
}

func (r *Runner) Enabled() bool { return r != nil && r.enabled }

func (r *Runner) acquire(ctx context.Context) error {
	select {
	case r.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runner) release() {
	<-r.slots
}

func (r *Runner) Run(ctx context.Context, name string, args []string, workingDir string) (*Result, error) {
	if !r.Enabled() {
		return nil, errors.New("process capability is disabled")
	}
	g, ok := r.grants[name]
	if !ok {
		return nil, fmt.Errorf("unknown executable grant %q", name)
	}
	if len(args) > r.maxArgs {
		return nil, fmt.Errorf("argument count %d exceeds limit %d", len(args), r.maxArgs)
	}
	totalArgBytes := 0
	for _, arg := range args {
		totalArgBytes += len(arg)
	}
	if totalArgBytes > 64*1024 {
		return nil, errors.New("process arguments exceed 64 KiB")
	}
	dir, err := r.roots.ResolveExisting(workingDir)
	if err != nil {
		return nil, fmt.Errorf("working directory: %w", err)
	}
	info, _, err := r.roots.Stat(workingDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("working_dir is not a directory")
	}
	resolved, err := filepath.EvalSymlinks(g.path)
	if err != nil {
		return nil, err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return nil, err
	}
	resolved = filepath.Clean(resolved)
	if resolved != g.path {
		return nil, errors.New("executable path identity changed since admission")
	}
	before, err := hashFile(resolved)
	if err != nil {
		return nil, err
	}
	if before != g.sha256 {
		return nil, errors.New("executable content identity changed since admission")
	}
	if err := r.acquire(ctx); err != nil {
		return nil, err
	}
	defer r.release()

	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	stdout := &cappedBuffer{max: r.maxOut}
	stderr := &cappedBuffer{max: r.maxOut}
	cmd := exec.CommandContext(runCtx, resolved, args...)
	cmd.Dir = dir
	cmd.Env = safeEnvironment()
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	started := time.Now()
	runErr := cmd.Run()
	duration := time.Since(started)
	result := &Result{
		ExitCode: 0,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		OutputTruncated: stdout.truncated || stderr.truncated,
		TimedOut: errors.Is(runCtx.Err(), context.DeadlineExceeded),
		DurationMS: duration.Milliseconds(),
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else if result.TimedOut {
			result.ExitCode = -1
		} else {
			return nil, runErr
		}
	}

	after, err := hashFile(g.path)
	if err != nil {
		return nil, fmt.Errorf("post-run executable identity readback: %w", err)
	}
	if after != g.sha256 {
		return nil, errors.New("executable content identity changed during execution")
	}
	return result, nil
}

func safeEnvironment() []string {
	allowed := map[string]struct{}{
		"SYSTEMROOT": {}, "WINDIR": {}, "COMSPEC": {}, "TEMP": {}, "TMP": {},
		"USERPROFILE": {}, "PATHEXT": {}, "HOME": {}, "TMPDIR": {},
		"LANG": {}, "LC_ALL": {}, "TERM": {},
	}
	var out []string
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, keep := allowed[strings.ToUpper(key)]; keep {
			out = append(out, entry)
		}
	}
	return out
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type cappedBuffer struct {
	buf       bytes.Buffer
	max       int64
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.max - int64(b.buf.Len())
	if remaining <= 0 {
		b.truncated = b.truncated || original > 0
		return original, nil
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buf.Write(p)
	return original, nil
}

func (b *cappedBuffer) String() string { return b.buf.String() }
