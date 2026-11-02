package parser

import (
	"strings"
	"testing"
)

const hideDoc = `\documentclass{article}
\newcommand{\resumeItem}[1]{\item{#1}}
\newcommand{\resumeItemListStart}{\begin{itemize}}
\newcommand{\resumeItemListEnd}{\end{itemize}}
\begin{document}
\section{Experience}
\resumeItemListStart
\resumeItem{Visible}
  %%HIDE-START b_0123456789abcdef
  % \resumeItem{
  %   Built backend APIs
  % }
  %%HIDE-END b_0123456789abcdef
\resumeItem{Also visible}
\resumeItemListEnd
\end{document}
`

func TestHideBlockIsOneHiddenNodeWithStableID(t *testing.T) {
	r, err := Parse(hideDoc, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	list := r.Sections[0].Items[0].Bullets
	if len(list.Bullets) != 3 {
		t.Fatalf("bullets: %+v", list.Bullets)
	}
	h := list.Bullets[1]
	if !h.Hidden || h.ID != "b_0123456789abcdef" || h.Text.Value != "Built backend APIs" && strings.TrimSpace(h.Text.Value) != "Built backend APIs" {
		t.Fatalf("hidden bullet: %+v", h)
	}
	if list.Bullets[0].Hidden || list.Bullets[2].Hidden {
		t.Fatal("neighbours must stay visible")
	}
	if got := hideDoc[h.Span[0] : h.Span[0]+1]; got != "%" {
		t.Fatalf("span must start at the comment mark, got %q", got)
	}
	// Hidden text is not visible content: coverage is unaffected.
	if r.Report.Coverage != 1 || r.Report.ReadOnlyRegions != 0 {
		t.Fatalf("report: %+v", r.Report)
	}
}

func TestHideBlockDuplicateAndUnpairedIgnored(t *testing.T) {
	src := strings.Replace(hideDoc, "%%HIDE-END b_0123456789abcdef", "%% stray", 1)
	r, err := Parse(src, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Without a matching END the block is just comments: the bullet is a
	// legacy-commented one (hidden), not a marker block.
	for _, b := range r.Sections[0].Items[0].Bullets.Bullets {
		if b.ID == "b_0123456789abcdef" {
			t.Fatal("unpaired marker must not assign an id")
		}
	}
}

func TestHideSection(t *testing.T) {
	src := `\documentclass{article}
\begin{document}
\section{Keep}
text
%%HIDE-START s_aaaaaaaaaaaaaaaa
% \section{Gone}
% \section*{Gone too}
%%HIDE-END s_aaaaaaaaaaaaaaaa
\end{document}
`
	r, err := Parse(src, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sections) != 3 || r.Sections[0].Hidden || !r.Sections[1].Hidden || !r.Sections[2].Hidden || r.Sections[1].ID != "s_aaaaaaaaaaaaaaaa" {
		t.Fatalf("%+v", r.Sections)
	}
}
