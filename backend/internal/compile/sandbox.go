package compile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrTimeout is returned when a compile exceeds its time limit.
var ErrTimeout = errors.New("compile timed out")

// Job is one sandboxed latexmk invocation.
type Job struct {
	// Dir contains main.tex and any assets. It is the only writable
	// directory inside the sandbox.
	Dir            string
	Compiler       string
	Timeout        time.Duration
	MaxMemoryBytes int64
}

// Runner executes one compile job. The real implementation is BwrapRunner;
// tests use fakes.
type Runner interface {
	Run(ctx context.Context, job Job) error
}

const (
	// DefaultTimeout bounds one latexmk run (tech-stack §6: 15 s).
	DefaultTimeout = 15 * time.Second
	// DefaultMaxMemoryBytes caps the whole sandboxed process tree (prlimit --as).
	DefaultMaxMemoryBytes = 1 << 30 // 1 GiB
	// maxRunOutput caps captured latexmk stdout/stderr.
	maxRunOutput = 256 << 10
)

// BwrapRunner runs latexmk under bubblewrap with no network, read-only system
// binds, and a writable per-job directory. Memory is capped by prlimit
// wrapping the whole bwrap process tree.
type BwrapRunner struct {
	BwrapPath   string // default "bwrap"
	PrlimitPath string // default "prlimit"
	// LatexmkPath is the latexmk binary inside the sandbox (default "latexmk").
	LatexmkPath string
	// ExtraBinds are additional read-only host paths needed on some distros
	// (e.g. /lib64). Only absolute, existing directories should be passed.
	ExtraBinds []string
}

var write18Re = regexp.MustCompile(`\\write\s*18\b`)

// ContainsShellEscape reports whether the source attempts shell escape. The
// sandbox fails closed: any of these tokens rejects the job outright, even in
// comments — a resume never needs them.
func ContainsShellEscape(tex string) bool {
	if write18Re.MatchString(tex) {
		return true
	}
	for _, token := range []string{"shell-escape", "enable-write18", "enable-pipes"} {
		if strings.Contains(tex, token) {
			return true
		}
	}
	return false
}

// ValidateAssetPath rejects anything that could escape the job directory.
func ValidateAssetPath(p string) error {
	if p == "" || p == "." {
		return errors.New("empty asset path")
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) {
		return fmt.Errorf("invalid asset path %q", p)
	}
	// Reject Windows-style separators/drives outright so the same validation
	// holds if the server ever runs on a non-POSIX host.
	if strings.ContainsAny(p, "\\") || (len(p) >= 2 && p[1] == ':') {
		return fmt.Errorf("invalid asset path %q", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == ".." {
			return fmt.Errorf("invalid asset path %q", p)
		}
	}
	return nil
}

// Run executes latexmk for the job. The compiler name is validated against
// the allowlist before anything is spawned, so a hostile compiler string can
// never become a latexmk flag.
func (r BwrapRunner) Run(ctx context.Context, job Job) error {
	if !IsValidCompiler(job.Compiler) {
		return fmt.Errorf("unsupported compiler %q", job.Compiler)
	}
	if job.Timeout <= 0 {
		job.Timeout = DefaultTimeout
	}
	if job.MaxMemoryBytes <= 0 {
		job.MaxMemoryBytes = DefaultMaxMemoryBytes
	}
	runCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, r.prlimit(), r.argv(job)...)
	// Minimal host-side environment; the sandbox env is set via --setenv.
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	var out cappedBuffer
	out.max = maxRunOutput
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if runCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
		return ErrTimeout
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("latexmk failed: %w", err)
	}
	return nil
}

// argv builds: prlimit --as=<bytes> -- bwrap <isolation> -- latexmk <flags> main.tex
// extraBinds are additional read-only host→sandbox directory binds.
func (r BwrapRunner) argv(job Job) []string {
	bwrap := r.BwrapPath
	if bwrap == "" {
		bwrap = "bwrap"
	}
	latexmk := r.LatexmkPath
	if latexmk == "" {
		latexmk = "latexmk"
	}
	argv := []string{
		"--as=" + strconv.FormatInt(job.MaxMemoryBytes, 10), "--",
		bwrap,
		// No network, no IPC, no PID/user namespace sharing; die with parent.
		"--unshare-all", "--die-with-parent",
		// Read-only system; only the job dir is writable.
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/etc", "/etc",
		// Debian TeX Live keeps prebuilt format files (.fmt) and ls-R
		// databases under /var/lib/texmf; pdflatex also needs /dev/null and
		// a /proc mount or it dies before writing main.log.
		"--ro-bind", "/var/lib/texmf", "/var/lib/texmf",
		"--proc", "/proc",
		"--dev", "/dev",
	}
	for _, b := range r.ExtraBinds {
		argv = append(argv, "--ro-bind", b, b)
	}
	argv = append(argv,
		"--tmpfs", "/tmp",
		"--bind", job.Dir, "/work",
		"--chdir", "/work",
		// TeX file-access policy: openin_any=p (paranoid read), openout_any=p
		// (paranoid write) — no reads/writes outside the job dir even if TeX
		// code tries.
		"--setenv", "openin_any", "p",
		"--setenv", "openout_any", "p",
		"--setenv", "PATH", "/usr/bin:/bin",
		"--setenv", "HOME", "/work",
		"--",
		latexmk, "-"+job.Compiler,
		"-interaction=nonstopmode",
		"-no-shell-escape",
		"-halt-on-error",
		"main.tex",
	)
	return argv
}

func (r BwrapRunner) prlimit() string {
	if r.PrlimitPath == "" {
		return "prlimit"
	}
	return r.PrlimitPath
}

// cappedBuffer keeps at most max bytes of process output.
type cappedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len() < b.max {
		n := b.max - b.buf.Len()
		if n > len(p) {
			n = len(p)
		}
		b.buf.Write(p[:n])
		if n < len(p) {
			b.truncated = true
		}
	} else {
		b.truncated = true
	}
	return len(p), nil
}

// writeJobFiles writes main.tex and assets into a fresh temp dir and returns
// the dir. The caller removes it.
func writeJobFiles(tex string, assets []assetFile) (string, error) {
	dir, err := os.MkdirTemp("", "compile-*")
	if err != nil {
		return "", fmt.Errorf("create job dir: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(dir)
		}
	}()
	if err := os.WriteFile(filepath.Join(dir, "main.tex"), []byte(tex), 0o600); err != nil {
		return "", fmt.Errorf("write main.tex: %w", err)
	}
	for _, a := range assets {
		dest := filepath.Join(dir, filepath.FromSlash(a.path))
		// Defense in depth: path was validated, confirm it stays inside dir.
		if !strings.HasPrefix(dest, dir+string(os.PathSeparator)) {
			return "", fmt.Errorf("invalid asset path %q", a.path)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return "", fmt.Errorf("create asset dir: %w", err)
		}
		if err := os.WriteFile(dest, a.data, 0o600); err != nil {
			return "", fmt.Errorf("write asset %q: %w", a.path, err)
		}
	}
	ok = true
	return dir, nil
}

type assetFile struct {
	path string
	data []byte
}
