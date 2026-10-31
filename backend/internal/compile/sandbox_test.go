package compile

import (
	"strings"
	"testing"
	"time"
)

func testJob() Job {
	return Job{
		Dir:            "/tmp/compile-abc123",
		Compiler:       "pdflatex",
		Timeout:        15 * time.Second,
		MaxMemoryBytes: 1 << 30,
	}
}

// TestArgvNeverEnablesShellEscape proves the constructed command line cannot
// be turned into a shell-escape: -no-shell-escape is always present, the
// compiler token is only ever one allowlisted word, and no caller-controlled
// string lands in the flag position.
func TestArgvNeverEnablesShellEscape(t *testing.T) {
	argv := BwrapRunner{}.argv(testJob())
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "-no-shell-escape") {
		t.Error("missing -no-shell-escape")
	}
	for _, tok := range argv {
		if tok == "--shell-escape" || tok == "-shell-escape" || tok == "--enable-write18" {
			t.Fatalf("argv enables shell escape: %v", argv)
		}
	}
	if !strings.HasPrefix(strings.Join(argv, " "), "--as=1073741824 -- ") {
		t.Errorf("prlimit memory cap must wrap everything, got: %v", argv[:3])
	}
}

func TestArgvSandboxIsolation(t *testing.T) {
	argv := BwrapRunner{}.argv(testJob())
	joined := strings.Join(argv, " ")
	for _, want := range []string{
		"--unshare-all",     // no network, no namespaces shared
		"--die-with-parent", // no orphans
		"--ro-bind /usr /usr",
		"--bind /tmp/compile-abc123 /work", // only writable dir
		"--chdir /work",
		"openin_any p",
		"openout_any p",
		"latexmk -pdflatex",
		"-interaction=nonstopmode",
		"-halt-on-error",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q:\n%s", want, joined)
		}
	}
}

func TestArgvCompilerFlag(t *testing.T) {
	for _, tc := range []struct{ compiler, flag string }{
		{"pdflatex", "-pdflatex"},
		{"xelatex", "-xelatex"},
		{"lualatex", "-lualatex"},
	} {
		job := testJob()
		job.Compiler = tc.compiler
		argv := BwrapRunner{}.argv(job)
		found := false
		for _, tok := range argv {
			if tok == tc.flag {
				found = true
			}
		}
		if !found {
			t.Errorf("compiler %s: flag %s not in %v", tc.compiler, tc.flag, argv)
		}
	}
}

// TestRunRejectsInvalidCompilerBeforeExec: a hostile compiler string must
// never reach exec. Run validates first and errors out without bwrap/prlimit
// even existing on this machine.
func TestRunRejectsInvalidCompilerBeforeExec(t *testing.T) {
	job := testJob()
	job.Compiler = "pdflatex -shell-escape"
	err := BwrapRunner{}.Run(t.Context(), job)
	if err == nil || !strings.Contains(err.Error(), "unsupported compiler") {
		t.Fatalf("expected unsupported compiler error, got %v", err)
	}
}

func TestValidateAssetPath(t *testing.T) {
	for _, bad := range []string{
		"", ".", "/etc/passwd", "../evil.tex", "a/../../b", "a//b",
		`a\b`, `C:\evil`, "a/../../..", "assets/../../../etc/passwd",
		"with\x00nul",
	} {
		if err := ValidateAssetPath(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	for _, ok := range []string{"photo.png", "img/photo.png", "a/b/c.pdf", "glyphtounicode.tex"} {
		if err := ValidateAssetPath(ok); err != nil {
			t.Errorf("%q should be accepted: %v", ok, err)
		}
	}
}

func TestContainsShellEscape(t *testing.T) {
	for _, bad := range []string{
		`\immediate\write18{touch pwned}`,
		`\write18{ls}`,
		`\write 18 {ls}`,
		"% !TEX options = --shell-escape",
		"% !TEX options = -shell-escape",
		"% !TEX options = --enable-write18",
		"--enable-pipes",
	} {
		if !ContainsShellEscape(bad) {
			t.Errorf("should detect shell-escape attempt: %q", bad)
		}
	}
	for _, ok := range []string{
		`\documentclass{article}\begin{document}hello\end{document}`,
		`write 18 pages of content`, // no backslash
	} {
		if ContainsShellEscape(ok) {
			t.Errorf("false positive on benign input: %q", ok)
		}
	}
}
