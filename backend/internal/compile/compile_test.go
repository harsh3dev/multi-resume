package compile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRunner writes a fake main.pdf (and optionally a main.log) into the job
// dir, like a real latexmk run would. block, when non-nil, holds every run
// open until closed or the context is cancelled. parkFirst holds only the
// first run open until its context is cancelled.
type fakeRunner struct {
	calls     atomic.Int64
	block     chan struct{}
	parkFirst bool
	log       string
	failErr   error
	noPDF     bool

	mu      sync.Mutex
	lastJob Job
}

func (f *fakeRunner) Run(ctx context.Context, job Job) error {
	call := f.calls.Add(1)
	f.mu.Lock()
	f.lastJob = job
	f.mu.Unlock()
	if f.parkFirst && call == 1 {
		<-ctx.Done()
		return ctx.Err()
	}
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.log != "" {
		// Mimic a real log that references the job dir.
		_ = os.WriteFile(filepath.Join(job.Dir, "main.log"), []byte(job.Dir+"/main.tex\n"+f.log), 0o600)
	}
	if f.failErr != nil {
		return f.failErr
	}
	if !f.noPDF {
		if err := os.WriteFile(filepath.Join(job.Dir, "main.pdf"), []byte("%PDF-fake"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

type fakePDF struct{}

func (fakePDF) PageCount(string) (int, error)      { return 1, nil }
func (fakePDF) ExtractText(string) (string, error) { return "pdf text", nil }

func newTestCompiler(t *testing.T, runner Runner, maxParallel int) *Compiler {
	t.Helper()
	c, err := New(Options{
		Cache:       NewMemoryCache(),
		Store:       NewMemoryStore(),
		Runner:      runner,
		PDFTools:    fakePDF{},
		MaxParallel: maxParallel,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const okTex = `\documentclass{article}\begin{document}hi\end{document}`

func TestCompileSuccess(t *testing.T) {
	runner := &fakeRunner{}
	c := newTestCompiler(t, runner, 4)
	res, err := c.Compile(t.Context(), okTex, nil, "pdflatex")
	if err != nil {
		t.Fatal(err)
	}
	if res.PageCount != 1 || res.Cached {
		t.Errorf("unexpected result %+v", res)
	}
	if !strings.HasPrefix(res.PDFKey, "pdf/") || !strings.HasSuffix(res.PDFKey, ".pdf") {
		t.Errorf("bad pdf key %q", res.PDFKey)
	}
}

func TestCompileCachesSecondRun(t *testing.T) {
	runner := &fakeRunner{}
	c := newTestCompiler(t, runner, 4)
	assets := map[string]string{"img/a.png": "r2/asset-a"}
	store := c.store.(*MemoryStore)
	_ = store.Put(t.Context(), "r2/asset-a", PlaceholderImage("a.png"))

	first, err := c.Compile(t.Context(), okTex, assets, "pdflatex")
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Compile(t.Context(), okTex, assets, "pdflatex")
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls.Load() != 1 {
		t.Errorf("gate: same input twice must run latexmk once, ran %d", runner.calls.Load())
	}
	if !second.Cached || second.PDFKey != first.PDFKey || second.PageCount != first.PageCount {
		t.Errorf("cache hit mismatch: %+v vs %+v", first, second)
	}
}

// TestCompileRejectsShellEscape: a source with \write18 must fail closed —
// rejected before the sandbox runner is ever invoked.
func TestCompileRejectsShellEscape(t *testing.T) {
	runner := &fakeRunner{}
	c := newTestCompiler(t, runner, 4)
	tex := `\documentclass{article}\begin{document}\immediate\write18{touch pwned}\end{document}`
	_, err := c.Compile(t.Context(), tex, nil, "pdflatex")
	var ce *CompileError
	if !errors.As(err, &ce) || !strings.Contains(ce.Message, "shell escape") {
		t.Fatalf("expected client-safe shell-escape rejection, got %v", err)
	}
	if runner.calls.Load() != 0 {
		t.Error("runner must not run for shell-escape input")
	}
}

// TestCompileRejectsInvalidCompiler: the compiler string becomes a CLI flag;
// anything off the allowlist must be refused before exec.
func TestCompileRejectsInvalidCompiler(t *testing.T) {
	runner := &fakeRunner{}
	c := newTestCompiler(t, runner, 4)
	_, err := c.Compile(t.Context(), okTex, nil, "pdflatex -shell-escape")
	if err == nil {
		t.Fatal("expected rejection")
	}
	if runner.calls.Load() != 0 {
		t.Error("runner must not run for invalid compiler")
	}
}

func TestCompileRejectsBadAssetPath(t *testing.T) {
	runner := &fakeRunner{}
	c := newTestCompiler(t, runner, 4)
	assets := map[string]string{"../../etc/passwd": "r2/x"}
	if _, err := c.Compile(t.Context(), okTex, assets, "pdflatex"); err == nil {
		t.Fatal("expected rejection of traversal path")
	}
	if runner.calls.Load() != 0 {
		t.Error("runner must not run for bad asset path")
	}
}

func TestCompileAutoDetectsCompiler(t *testing.T) {
	runner := &fakeRunner{}
	c := newTestCompiler(t, runner, 4)
	tex := `\documentclass{article}\usepackage{fontspec}\begin{document}hi\end{document}`
	if _, err := c.Compile(t.Context(), tex, nil, ""); err != nil {
		t.Fatal(err)
	}
	runner.mu.Lock()
	got := runner.lastJob.Compiler
	runner.mu.Unlock()
	if got != "xelatex" {
		t.Errorf("auto-detected compiler = %q, want xelatex", got)
	}
}

// TestCompileErrorParsing: on latexmk failure the client gets the parsed
// error line with the temp dir stripped — never raw paths or the full log.
func TestCompileErrorParsing(t *testing.T) {
	runner := &fakeRunner{
		log:     "(/usr/share/texlive/texmf-dist/tex/latex/base/article.cls)\n! Undefined control sequence.\nl.7 \\nope\n",
		failErr: errors.New("exit status 12"),
	}
	c := newTestCompiler(t, runner, 4)
	_, err := c.Compile(t.Context(), okTex, nil, "pdflatex")
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CompileError, got %T: %v", err, err)
	}
	if !strings.Contains(ce.Message, "! Undefined control sequence.") {
		t.Errorf("missing parsed error line: %q", ce.Message)
	}
	if strings.Contains(ce.Message, "/tmp/") || strings.Contains(ce.Message, "/usr/share") {
		t.Errorf("paths leaked to client message: %q", ce.Message)
	}
}

func TestCompileTimeoutIsClientSafe(t *testing.T) {
	runner := &fakeRunner{failErr: ErrTimeout}
	c := newTestCompiler(t, runner, 4)
	_, err := c.Compile(t.Context(), okTex, nil, "pdflatex")
	var ce *CompileError
	if !errors.As(err, &ce) || !strings.Contains(ce.Message, "timed out") {
		t.Fatalf("expected timeout CompileError, got %v", err)
	}
}

func TestCompileNoPDFProduced(t *testing.T) {
	runner := &fakeRunner{noPDF: true}
	c := newTestCompiler(t, runner, 4)
	_, err := c.Compile(t.Context(), okTex, nil, "pdflatex")
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CompileError, got %v", err)
	}
}

// TestSemaphoreBlocksExtraCompile: with MaxParallel=1 a second compile waits
// for the first to finish instead of running latexmk concurrently.
func TestSemaphoreBlocksExtraCompile(t *testing.T) {
	runner := &fakeRunner{block: make(chan struct{})}
	c := newTestCompiler(t, runner, 1)

	first := make(chan error, 1)
	// Use distinct sources so the second compile is not a cache hit.
	go func() {
		_, err := c.Compile(context.Background(), okTex, nil, "pdflatex")
		first <- err
	}()
	deadline := time.After(2 * time.Second)
	for runner.calls.Load() != 1 {
		select {
		case <-deadline:
			t.Fatal("first compile never started")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	second := make(chan error, 1)
	go func() {
		_, err := c.Compile(context.Background(), okTex+"% different", nil, "pdflatex")
		second <- err
	}()
	select {
	case <-second:
		t.Fatal("second compile ran while the semaphore was full")
	case <-time.After(50 * time.Millisecond):
	}
	close(runner.block)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if runner.calls.Load() != 2 {
		t.Errorf("expected 2 runner calls, got %d", runner.calls.Load())
	}
}

// TestCompileDraftCancelsPrevious: a new compile for the same draft cancels
// the in-flight one; the runner observes context cancellation.
func TestCompileDraftCancelsPrevious(t *testing.T) {
	runner := &fakeRunner{parkFirst: true}
	c := newTestCompiler(t, runner, 4)

	first := make(chan error, 1)
	go func() {
		_, err := c.CompileDraft(context.Background(), "draft-1", okTex, nil, "pdflatex")
		first <- err
	}()
	deadline := time.After(2 * time.Second)
	for runner.calls.Load() != 1 {
		select {
		case <-deadline:
			t.Fatal("first compile never started")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	second, err := c.CompileDraft(context.Background(), "draft-1", okTex+"% v2", nil, "pdflatex")
	if err != nil {
		t.Fatal(err)
	}
	if second == nil {
		t.Fatal("second compile should succeed")
	}
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Errorf("first compile should be cancelled, got %v", err)
	}
}

func TestNewRequiresDeps(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Error("missing deps should error")
	}
}
