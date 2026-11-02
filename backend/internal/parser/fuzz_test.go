package parser

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fuzzTokens = []string{
	`{`, `}`, `\begin{itemize}`, `\end{itemize}`, `\item `, `%`, "\n", "\n\n", `\\`, `$`, `\section{X}`,
	`\textbf{`, `\href{a}`, `%%HIDE-START b_0123456789abcdef` + "\n", `%%HIDE-END b_0123456789abcdef` + "\n",
	`\resumeSubheading{a}{b}{c}{d}`, `\cventry{a}{b}{c}{d}{e}{f}`, `\Huge `, `[`, `]`, `\`, "é",
}

// mutate applies a few small random edits to a fixture.
func mutate(rng *rand.Rand, src string) string {
	for k := 1 + rng.Intn(4); k > 0 && len(src) > 10; k-- {
		i := rng.Intn(len(src))
		// Stay on rune boundaries so the result is still valid UTF-8.
		for i > 0 && src[i]&0xC0 == 0x80 {
			i--
		}
		switch rng.Intn(4) {
		case 0: // delete a range
			j := i + rng.Intn(60)
			if j > len(src) {
				j = len(src)
			}
			for j < len(src) && src[j]&0xC0 == 0x80 {
				j++
			}
			src = src[:i] + src[j:]
		case 1: // insert a token
			src = src[:i] + fuzzTokens[rng.Intn(len(fuzzTokens))] + src[i:]
		case 2: // duplicate a range
			j := i + rng.Intn(80)
			if j > len(src) {
				j = len(src)
			}
			for j < len(src) && src[j]&0xC0 == 0x80 {
				j++
			}
			src = src[:j] + src[i:j] + src[j:]
		case 3: // swap two lines
			lines := strings.Split(src, "\n")
			a, b := rng.Intn(len(lines)), rng.Intn(len(lines))
			lines[a], lines[b] = lines[b], lines[a]
			src = strings.Join(lines, "\n")
		}
	}
	return src
}

// checkTree is the Phase 6.2 invariant set: no panic (the test would crash),
// every span inside the source and well-formed, field values reproducible
// from their spans, sections ordered and non-overlapping.
func checkTree(t *testing.T, label, src string) {
	t.Helper()
	r, err := Parse(src, ParseOptions{})
	if err != nil {
		return // a clear "not supported" error is a valid outcome
	}
	for _, p := range checkSpans(src, r) {
		t.Errorf("%s: %s", label, p)
	}
	for _, s := range r.Sections {
		for _, it := range s.Items {
			var span [2]int
			switch {
			case it.Entry != nil:
				span = it.Entry.Span
			case it.Bullets != nil:
				span = it.Bullets.Span
				for _, b := range it.Bullets.Bullets {
					if b.Span[0] < 0 || b.Span[1] < b.Span[0] || b.Span[1] > len(src) {
						t.Errorf("%s: bullet span %v", label, b.Span)
					}
				}
			case it.Skill != nil:
				span = it.Skill.Span
			case it.Paragraph != nil:
				span = it.Paragraph.Span
			case it.Opaque != nil:
				span = it.Opaque.Span
			}
			if span[0] < 0 || span[1] < span[0] || span[1] > len(src) {
				t.Errorf("%s: item span %v out of range (len %d)", label, span, len(src))
			}
		}
	}
	if r.Report.Coverage < 0 || r.Report.Coverage > 1 {
		t.Errorf("%s: coverage %v", label, r.Report.Coverage)
	}
}

// TestFuzzMutatedFixtures mutates every fixture many times with a fixed seed
// so any failure is reproducible.
func TestFuzzMutatedFixtures(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/resumes/*.tex")
	if len(files) == 0 {
		t.Skip("no fixtures")
	}
	rounds := 40
	if testing.Short() {
		rounds = 5
	}
	rng := rand.New(rand.NewSource(20261101))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < rounds; i++ {
			src := mutate(rng, string(data))
			checkTree(t, filepath.Base(f)+"#"+string(rune('a'+i%26)), src)
		}
	}
}

// FuzzParse is the open-ended version: go test -fuzz FuzzParse ./internal/parser
func FuzzParse(f *testing.F) {
	for _, name := range []string{"jake.tex", "moderncv-template.tex", "altacv-sample.tex"} {
		if data, err := os.ReadFile("../../testdata/resumes/" + name); err == nil {
			f.Add(string(data))
		}
	}
	f.Add(hideDoc)
	f.Fuzz(func(t *testing.T, src string) {
		checkTree(t, "fuzz", src)
	})
}
