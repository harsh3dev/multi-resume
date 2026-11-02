//go:build unix

package compile

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// ErrCompilerDisabled is returned for compilers DirectRunner refuses to run.
var ErrCompilerDisabled = errors.New("compiler disabled in direct mode")

// DirectRunner runs latexmk as a plain child process, without bubblewrap.
// It is for hosts that cannot create user namespaces (Cloud Run). The
// container is the isolation boundary, so this runner only adds what a
// process can enforce on itself, and the deployment must supply the rest:
// a non-root user, no secrets and no network egress in the service, a
// private per-job directory, one job per instance.
//
// What it enforces:
//   - shell escape off three ways (flag, env, texmf.cnf in the image)
//   - openin_any/openout_any=paranoid: TeX refuses paths outside the job dir
//   - lualatex refused (arbitrary Lua is too large an attack surface here)
//   - rlimits: address space, CPU seconds, file size, process count, open files
//   - a wall-clock timeout, killing the entire process group (latexmk spawns
//     pdflatex; killing only the parent would leak the child)
//   - HOME, TEXMF caches and TMPDIR pinned inside the job dir, so nothing
//     persists between jobs
//   - a minimal environment: nothing from the service's own env is inherited
type DirectRunner struct {
	PrlimitPath string // default "prlimit"
	LatexmkPath string // default "latexmk"
	// MaxProcs caps tasks for the user (RLIMIT_NPROC counts every thread of
	// every process of the uid, including this server). Default 256.
	MaxProcs int
	// MaxFileBytes caps any single file a job writes. Default 64 MiB.
	MaxFileBytes int64
}

const (
	defaultMaxProcs     = 256
	defaultMaxFileBytes = 64 << 20
	directKillGrace     = 2 * time.Second
)

func (r DirectRunner) Run(ctx context.Context, job Job) error {
	if !IsValidCompiler(job.Compiler) {
		return fmt.Errorf("unsupported compiler %q", job.Compiler)
	}
	if job.Compiler == CompilerLuaLatex {
		return ErrCompilerDisabled
	}
	if job.Timeout <= 0 {
		job.Timeout = DefaultTimeout
	}
	if job.MaxMemoryBytes <= 0 {
		job.MaxMemoryBytes = DefaultMaxMemoryBytes
	}
	runCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	name, args := r.command(job)
	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Dir = job.Dir
	cmd.Env = r.env(job)
	// Own process group, so the whole tree can be killed at once.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = directKillGrace
	var out cappedBuffer
	out.max = maxRunOutput
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	// Whatever happened, nothing from this job may outlive it: sweep the group
	// (ESRCH when it is already empty).
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
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

// command builds: prlimit <limits> -- latexmk <flags> main.tex
func (r DirectRunner) command(job Job) (string, []string) {
	prlimit := r.PrlimitPath
	if prlimit == "" {
		prlimit = "prlimit"
	}
	latexmk := r.LatexmkPath
	if latexmk == "" {
		latexmk = "latexmk"
	}
	procs := r.MaxProcs
	if procs <= 0 {
		procs = defaultMaxProcs
	}
	fsize := r.MaxFileBytes
	if fsize <= 0 {
		fsize = defaultMaxFileBytes
	}
	cpu := int64(job.Timeout/time.Second) + 5
	return prlimit, []string{
		"--as=" + strconv.FormatInt(job.MaxMemoryBytes, 10),
		"--cpu=" + strconv.FormatInt(cpu, 10),
		"--fsize=" + strconv.FormatInt(fsize, 10),
		"--nproc=" + strconv.Itoa(procs),
		"--nofile=256",
		"--core=0",
		"--",
		latexmk, "-" + job.Compiler,
		"-interaction=nonstopmode",
		"-no-shell-escape",
		"-halt-on-error",
		"main.tex",
	}
}

// env is the complete environment of the job: nothing is inherited.
func (r DirectRunner) env(job Job) []string {
	return []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + job.Dir,
		"TMPDIR=" + job.Dir,
		"XDG_CACHE_HOME=" + filepath.Join(job.Dir, ".cache"),
		"TEXMFVAR=" + filepath.Join(job.Dir, ".texmf-var"),
		"TEXMFCONFIG=" + filepath.Join(job.Dir, ".texmf-config"),
		"LC_ALL=C.UTF-8",
		// kpathsea reads texmf.cnf variables from the environment too, so the
		// file-access policy holds even if the image's texmf.cnf is changed.
		"openin_any=p",
		"openout_any=p",
		"shell_escape=f",
	}
}
