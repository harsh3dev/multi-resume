package parser

import "testing"

func TestCommentedBullet(t *testing.T) {
	src := `\documentclass{article}
\newcommand{\resumeItem}[1]{\item{#1}}
\newcommand{\resumeItemListStart}{\begin{itemize}}
\newcommand{\resumeItemListEnd}{\end{itemize}}
\begin{document}
\section{Experience}
\resumeItemListStart
\resumeItem{Visible}
% \resumeItem{Hidden bullet}
\resumeItemListEnd
\end{document}`
	r, err := Parse(src, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	list := r.Sections[0].Items[0].Bullets
	if list == nil || len(list.Bullets) != 2 || !list.Bullets[1].Hidden || list.Bullets[1].Text.Value != "Hidden bullet" {
		t.Fatalf("hidden bullets: %+v", list)
	}
}
