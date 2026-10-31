package parser

import (
	"os"
	"testing"
)

func TestJakeShape(t *testing.T) {
	src, err := os.ReadFile("../../testdata/resumes/jake.tex")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(string(src), ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("jake sections=%d entries=%d bullets=%d", len(r.Sections), r.Report.Entries, r.Report.Bullets)
	if len(r.Sections) != 4 || r.Sections[1].Title.Value != "Experience" || r.Report.Entries < 6 || r.Report.Bullets < 15 {
		t.Fatalf("jake parser coverage is insufficient: %+v", r.Report)
	}
	if r.Sections[1].Items[0].Entry == nil || r.Sections[1].Items[0].Entry.Bullets == nil {
		t.Fatal("experience entry missing bullets")
	}
	if r.Header == nil || r.Header.Name == nil || r.Header.Name.Value != "Jake Ryan" {
		t.Fatalf("name extraction failed: %+v", r.Header)
	}
	// Since the converter handles $\times$/$|$-style math, jake.tex parses
	// completely: full coverage and no read-only regions.
	if r.Report.Coverage != 1 || r.Report.ReadOnlyRegions != 0 {
		t.Fatalf("jake.tex should parse fully: %+v", r.Report)
	}
	if r.Report.Coverage > 1 {
		t.Fatal("overlapping spans inflated coverage")
	}
	skillCount := 0
	for _, item := range r.Sections[3].Items {
		if item.Skill != nil {
			skillCount++
		}
	}
	if skillCount != 4 {
		t.Fatalf("skill extraction: got %d", skillCount)
	}
}

func TestExampleShape(t *testing.T) {
	src := `\documentclass{article}
\newcommand{\resumeSubheading}[4]{\item\begin{tabular}{lr}\textbf{#1} & #2 \\ \textit{#3} & #4\end{tabular}}
\newcommand{\resumeItem}[1]{\item{#1}}
\newcommand{\resumeItemListStart}{\begin{itemize}}
\newcommand{\resumeItemListEnd}{\end{itemize}}
\begin{document}
\section{Experience}
\resumeSubheading{Groww}{Bengaluru, India}{Software Development Engineer 1}{Sep 2025 -- Present}
\resumeItemListStart
\resumeItem{Reduced API latency by 34\% using \textbf{Redis} caching}
\resumeItemListEnd
\end{document}`
	r, err := Parse(src, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sections) != 1 || r.Sections[0].Title.Value != "Experience" {
		t.Fatalf("bad sections: %+v", r.Sections)
	}
	if len(r.Sections[0].Items) != 1 || r.Sections[0].Items[0].Entry == nil || len(r.Sections[0].Items[0].Entry.Fields) != 4 {
		t.Fatalf("bad entry: %+v", r.Sections[0].Items)
	}
	if r.Sections[0].Items[0].Entry.Bullets == nil {
		t.Fatal("missing bullet list")
	}
}

func TestInvalidSource(t *testing.T) {
	for _, source := range []string{"", string([]byte{0xff}), `\documentclass{article}`} {
		if _, err := Parse(source, ParseOptions{}); err == nil {
			t.Fatalf("accepted invalid source %q", source)
		}
	}
}
