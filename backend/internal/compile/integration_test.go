//go:build docker

// Integration tests for the real sandbox. They require a Linux environment
// with latexmk (TeX Live), bubblewrap, prlimit, and poppler-utils — i.e. the
// backend Docker image — and are skipped by default:
//
//	go test -tags docker ./internal/compile/ -run TestIntegration -v
package compile_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
)

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not available; run inside the backend image", name)
	}
}

func newRealCompiler(t *testing.T) (*compile.Compiler, *compile.MemoryStore) {
	t.Helper()
	requireTool(t, "latexmk")
	requireTool(t, "bwrap")
	requireTool(t, "prlimit")
	requireTool(t, "pdfinfo")
	requireTool(t, "pdftotext")
	store := compile.NewMemoryStore()
	c, err := compile.New(compile.Options{
		Cache:    compile.NewMemoryCache(),
		Store:    store,
		Runner:   compile.BwrapRunner{ExtraBinds: existingDirs("/lib64")},
		PDFTools: compile.PopplerTools{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c, store
}

func existingDirs(paths ...string) []string {
	var out []string
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/compile/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestIntegrationJakeCompiles(t *testing.T) {
	c, _ := newRealCompiler(t)
	res, err := c.Compile(t.Context(), readFixture(t, "jake-minimal.tex"), nil, "pdflatex")
	if err != nil {
		t.Fatalf("jake-minimal should compile: %v", err)
	}
	if res.PageCount != 1 {
		t.Errorf("expected 1 page, got %d", res.PageCount)
	}
	// Cache: second compile must not rerun latexmk (implicitly verified by
	// Cached flag; the runner is real here).
	res2, err := c.Compile(t.Context(), readFixture(t, "jake-minimal.tex"), nil, "pdflatex")
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Cached {
		t.Error("second compile should be a cache hit")
	}
}

// TestIntegrationShellEscapeDenied: the \write18 fixture must be rejected
// before latexmk runs, and even if it reached latexmk, -no-shell-escape
// guarantees no side effect. Assert the marker file does not exist anywhere
// we can observe.
func TestIntegrationShellEscapeDenied(t *testing.T) {
	c, _ := newRealCompiler(t)
	_, err := c.Compile(t.Context(), readFixture(t, "shell-escape.tex"), nil, "pdflatex")
	if err == nil {
		t.Fatal("shell-escape fixture must fail")
	}
	var ce *compile.CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("expected a client-safe CompileError, got %T: %v", err, err)
	}
	for _, sideEffect := range []string{"pwned-shell-escape", "/tmp/pwned-shell-escape", "/work/pwned-shell-escape"} {
		if _, statErr := os.Stat(sideEffect); statErr == nil {
			t.Fatalf("shell escape executed: %s exists", sideEffect)
		}
	}
}

func TestIntegrationHangKilled(t *testing.T) {
	c, _ := newRealCompiler(t)
	start := time.Now()
	_, err := c.Compile(t.Context(), readFixture(t, "hang.tex"), nil, "pdflatex")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("hang fixture must fail")
	}
	var ce *compile.CompileError
	if !errors.As(err, &ce) || !strings.Contains(ce.Message, "timed out") {
		t.Fatalf("expected timeout, got %v", err)
	}
	if elapsed > 20*time.Second {
		t.Errorf("timeout not enforced at ~15s: took %s", elapsed)
	}
}

func TestIntegrationXeLatex(t *testing.T) {
	c, store := newRealCompiler(t)
	tex := readFixture(t, "xelatex-minimal.tex")
	if got := compile.DetectCompiler(tex); got != "xelatex" {
		t.Fatalf("detected %q, want xelatex", got)
	}
	res, err := c.Compile(t.Context(), tex, nil, "")
	if err != nil {
		t.Fatalf("xelatex fixture should compile: %v", err)
	}
	if res.PageCount != 1 {
		t.Errorf("expected 1 page, got %d", res.PageCount)
	}
	// Pull the PDF back out of the store and check its text (probe-compile
	// style: the ZQ token must survive to the rendered PDF).
	pdf, err := store.Get(t.Context(), res.PDFKey)
	if err != nil {
		t.Fatal(err)
	}
	pdfPath := filepath.Join(t.TempDir(), "out.pdf")
	if err := os.WriteFile(pdfPath, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	text, err := c.ExtractText(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "ZQ001") {
		t.Errorf("extracted text missing probe token: %q", text)
	}
}
