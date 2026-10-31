package parser

import (
	"testing"
)

func TestStylesAndAssets(t *testing.T) {
	src := `\documentclass[a4paper,11pt]{article}
\geometry{margin=0.5in,top=0.2in}
\setmainfont{Charter}
\titlespacing{\section}{0pt}{4pt}{2pt}
\titleformat{\section}{\large\bfseries}{}{0pt}{}
\definecolor{navy}{HTML}{1F4E79}
\setlength{\itemsep}{3pt}
\begin{document}
\includegraphics[width=1in]{headshot.png}
\end{document}`
	pre, _, err := splitPreambleBody(src)
	if err != nil {
		t.Fatal(err)
	}
	styles := findStyles(src, pre)
	values := map[string]string{}
	for _, s := range styles {
		values[s.Key] = s.Value
		if s.Span == nil || src[s.Span[0]:s.Span[1]] != s.Value {
			t.Fatalf("bad style span: %+v", s)
		}
	}
	for k, v := range map[string]string{"fontSize": "11pt", "pageSize": "a4paper", "margin": "0.5in", "fontFamily": "Charter", "color:navy": "1F4E79", "bulletSpacing": "3pt", "sectionSpacing": "4pt", "headingFormat": `\large\bfseries`} {
		if values[k] != v {
			t.Errorf("%s: got %q, want %q", k, values[k], v)
		}
	}
	assets := findAssets(src)
	if len(assets) != 1 || assets[0].Path != "headshot.png" || src[assets[0].Span[0]:assets[0].Span[1]] != "headshot.png" {
		t.Fatalf("assets: %+v", assets)
	}
}
