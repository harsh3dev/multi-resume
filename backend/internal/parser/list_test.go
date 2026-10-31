package parser

import "testing"

func TestBareItemize(t *testing.T) {
	src := `\documentclass{article}
\begin{document}
\section{Projects}
\begin{itemize}
\item Built a reliable API
\item Reduced latency by 30\%
\end{itemize}
\end{document}`
	r, err := Parse(src, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sections) != 1 || len(r.Sections[0].Items) == 0 {
		t.Fatalf("missing list: %+v", r.Sections)
	}
	list := r.Sections[0].Items[0].Bullets
	if list == nil || len(list.Bullets) != 2 || list.Bullets[0].Text.Value != "Built a reliable API" || list.Bullets[1].Text.Value != "Reduced latency by 30%" {
		t.Fatalf("bare bullets: %+v", list)
	}
}
