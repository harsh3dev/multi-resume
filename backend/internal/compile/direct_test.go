//go:build unix

package compile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDirectCommandHardening(t *testing.T) {
	name, args := DirectRunner{}.command(testJob())
	joined := name + " " + strings.Join(args, " ")
	for _, want := range []string{
		"prlimit --as=1073741824", "--cpu=20", "--fsize=", "--nproc=256", "--nofile=256", "--core=0",
		"-- latexmk -pdflatex", "-no-shell-escape", "-halt-on-error", "-interaction=nonstopmode", "main.tex",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("command missing %q:\n%s", want, joined)
		}
	}
	for _, a := range args {
		if a == "-shell-escape" || a == "--shell-escape" || a == "--enable-write18" {
			t.Fatalf("enables shell escape: %v", args)
		}
	}
}

func TestDirectEnvIsMinimalAndJobScoped(t *testing.T) {
	t.Setenv("R2_SECRET_ACCESS_KEY", "super-secret")
	job := testJob()
	env := DirectRunner{}.env(job)
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "super-secret") || strings.Contains(joined, "R2_") {
		t.Fatal("service environment leaked into the job")
	}
	for _, want := range []string{
		"HOME=" + job.Dir, "TMPDIR=" + job.Dir, "TEXMFVAR=" + job.Dir + "/.texmf-var",
		"openin_any=p", "openout_any=p", "shell_escape=f",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("env missing %q", want)
		}
	}
}

func TestDirectRefusesLuaLatexAndBadCompiler(t *testing.T) {
	job := testJob()
	job.Compiler = CompilerLuaLatex
	if err := (DirectRunner{}).Run(context.Background(), job); !errors.Is(err, ErrCompilerDisabled) {
		t.Fatalf("lualatex: got %v", err)
	}
	job.Compiler = "pdflatex; rm -rf /"
	if err := (DirectRunner{}).Run(context.Background(), job); err == nil {
		t.Fatal("hostile compiler name accepted")
	}
}

// fakeTools writes executable stand-ins: a prlimit that drops its own flags
// (macOS has no prlimit) and a latexmk running body.
func fakeTools(t *testing.T, latexmkBody string) DirectRunner {
	t.Helper()
	dir := t.TempDir()
	prlimit := filepath.Join(dir, "prlimit")
	latexmk := filepath.Join(dir, "latexmk")
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write(prlimit, `while [ "$1" != "--" ]; do shift; done; shift; exec "$@"`)
	write(latexmk, latexmkBody)
	return DirectRunner{PrlimitPath: prlimit, LatexmkPath: latexmk}
}

func TestDirectRunSuccessUsesJobDir(t *testing.T) {
	r := fakeTools(t, `echo "$HOME $PWD $openout_any $shell_escape" > main.pdf`)
	job := testJob()
	job.Dir = t.TempDir()
	if err := r.Run(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(job.Dir, "main.pdf"))
	dir, _ := filepath.EvalSymlinks(job.Dir)
	if !strings.Contains(string(got), dir) || !strings.Contains(string(got), "p f") {
		t.Fatalf("unexpected job environment: %q", got)
	}
}

func TestDirectTimeoutKillsWholeProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	// latexmk spawns a long-lived grandchild, like latexmk → pdflatex.
	r := fakeTools(t, `sleep 300 & echo $! > `+pidFile+`; wait`)
	job := testJob()
	job.Dir = t.TempDir()
	job.Timeout = 1500 * time.Millisecond

	start := time.Now()
	err := r.Run(context.Background(), job)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("timeout took %v", time.Since(start))
	}
	raw, _ := os.ReadFile(pidFile)
	pid := 0
	for _, c := range strings.TrimSpace(string(raw)) {
		pid = pid*10 + int(c-'0')
	}
	if pid == 0 {
		t.Fatal("grandchild pid not recorded")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return // gone
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("grandchild %d survived the timeout", pid)
}

func TestDirectCancelStopsJob(t *testing.T) {
	r := fakeTools(t, `sleep 300`)
	job := testJob()
	job.Dir = t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	if err := r.Run(ctx, job); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
