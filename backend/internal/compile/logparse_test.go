package compile

import (
	"strings"
	"testing"
)

const sampleLog = `This is pdfTeX, Version 3.14159265-2.6-1.40.22 (TeX Live 2024)
entering extended mode
(/tmp/compile-123456/main.tex
LaTeX2e <2023-11-01> patch level 1
(/usr/share/texlive/texmf-dist/tex/latex/base/article.cls)
! Undefined control sequence.
l.42 \badcommand
                 {oops}
The control sequence at the end of the top line
of your error message was never \def'ed.
`

func TestParseLogError(t *testing.T) {
	got := ParseLogError(sampleLog, "/tmp/compile-123456")
	if got == "" {
		t.Fatal("expected an error excerpt")
	}
	if !strings.HasPrefix(got, "! Undefined control sequence.") {
		t.Errorf("should start at the ! line, got:\n%s", got)
	}
	if !strings.Contains(got, "l.42 \\badcommand") {
		t.Errorf("should include context lines, got:\n%s", got)
	}
	if strings.Contains(got, "/tmp/compile-123456") {
		t.Error("job dir must be stripped")
	}
	if strings.Contains(got, "/usr/share") {
		t.Error("absolute system paths must be stripped")
	}
}

func TestParseLogErrorNoError(t *testing.T) {
	if got := ParseLogError("This is pdfTeX\nOutput written on main.pdf (1 page).", "/tmp/x"); got != "" {
		t.Errorf("no ! line should yield empty, got %q", got)
	}
}

func TestParseLogErrorStripsOnlyFirstError(t *testing.T) {
	log := "! First error.\nl.1 x\n! Second error.\nl.2 y\n"
	got := ParseLogError(log)
	if !strings.Contains(got, "! First error.") {
		t.Error("first error missing")
	}
	if strings.Contains(got, "! Second error.") {
		t.Error("should stop before the second error context window ends")
	}
}

func TestParseLogErrorCapsLength(t *testing.T) {
	var b strings.Builder
	b.WriteString("! Boom.\n")
	for i := 0; i < 100; i++ {
		b.WriteString(strings.Repeat("x", 200) + "\n")
	}
	if got := ParseLogError(b.String()); len(got) > maxErrorLen {
		t.Errorf("excerpt exceeds cap: %d > %d", len(got), maxErrorLen)
	}
}

func TestStripPaths(t *testing.T) {
	in := "(/tmp/compile-99/main.tex) and /usr/share/texmf/tex/latex/base/article.cls but ./main.log stays"
	got := stripPaths(in, []string{"/tmp/compile-99"})
	if strings.Contains(got, "/tmp/compile-99") || strings.Contains(got, "/usr/share") {
		t.Errorf("paths not stripped: %s", got)
	}
	if !strings.Contains(got, "main.tex") || !strings.Contains(got, "article.cls") {
		t.Errorf("base names should survive: %s", got)
	}
	if !strings.Contains(got, "./main.log") {
		t.Errorf("relative paths should be untouched: %s", got)
	}
}
