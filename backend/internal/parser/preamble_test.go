package parser

import (
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
)

func TestSplitPreambleBody(t *testing.T) {
	src := "% \\begin{document}\n\\documentclass{article}\n\\begin{document}\nHello"
	pre, body, err := splitPreambleBody(src)
	if err != nil {
		t.Fatal(err)
	}
	if src[pre[0]:pre[1]] != "% \\begin{document}\n\\documentclass{article}\n" || src[body[0]:body[1]] != "\nHello" {
		t.Fatalf("wrong spans: %v %v", pre, body)
	}
	if _, _, err := splitPreambleBody(`\documentclass{article}`); err == nil {
		t.Fatal("missing document accepted")
	}
}

func TestReadDefinitions(t *testing.T) {
	cases := []struct {
		source, name string
		count        int
		optional     string
	}{
		{`\newcommand{\itemCustom}[1]{\item{#1}}`, `\itemCustom`, 1, ""},
		{`\renewcommand\short[2][fallback]{#1 #2}`, `\short`, 2, "fallback"},
		{`\def\classic#1#2{\textbf{#1} #2}`, `\classic`, 2, ""},
		{`\NewDocumentCommand{\modern}{m O{fallback} m}{#1 #2 #3}`, `\modern`, 3, "fallback"},
		{`\newcommand*{\star}[2]{\textbf{#1} #2}`, `\star`, 2, ""},
	}
	for _, tc := range cases {
		defs := readDefinitions(latex.Tokenize(tc.source), tc.source)
		if len(defs) != 1 {
			t.Errorf("%s: got %d definitions", tc.name, len(defs))
			continue
		}
		got := defs[0]
		if got.Name != tc.name || got.ArgCount != tc.count || (got.OptionalDefault != nil && *got.OptionalDefault != tc.optional) || (tc.optional != "" && got.OptionalDefault == nil) {
			t.Errorf("%s: got %+v", tc.name, got)
		}
	}
}
