package parser

import "testing"

func TestCompilerHint(t *testing.T) {
	for _, tc := range []struct{ preamble, want string }{
		{`% !TEX program = lualatex` + "\n" + `\usepackage{fontspec}`, "lualatex"},
		{`\usepackage{fontspec}`, "xelatex"},
		{"", "pdflatex"},
	} {
		src := `\documentclass{article}` + "\n" + tc.preamble + "\n" + `\begin{document}\section{A}\end{document}`
		r, err := Parse(src, ParseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if r.Meta.Compiler != tc.want {
			t.Errorf("compiler %s != %s", r.Meta.Compiler, tc.want)
		}
	}
}
