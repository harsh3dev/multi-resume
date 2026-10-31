package compile

import "testing"

func TestDetectCompiler(t *testing.T) {
	for _, tc := range []struct {
		name     string
		preamble string
		want     string
	}{
		{"empty", "", "pdflatex"},
		{"plain article", `\documentclass{article}\usepackage{enumitem}`, "pdflatex"},
		{"tex program wins", "% !TEX program = lualatex\n\\usepackage{fontspec}", "lualatex"},
		{"tex program xelatex", "% !TEX program=xelatex", "xelatex"},
		{"tex program case insensitive", "%!TEX program = XeLaTeX", "xelatex"},
		{"tex program unknown falls through", "% !TEX program = tectonic", "pdflatex"},
		{"fontspec", `\usepackage{fontspec}`, "xelatex"},
		{"fontspec with options", `\usepackage[no-math]{fontspec}`, "xelatex"},
		{"polyglossia", `\usepackage{polyglossia}`, "xelatex"},
		{"requirepackage fontspec", `\RequirePackage{fontspec}`, "xelatex"},
		{"package list with fontspec", `\usepackage{xcolor,fontspec,enumitem}`, "xelatex"},
		{"luacode", `\usepackage{luacode}`, "lualatex"},
		{"luaotfload", `\usepackage{luaotfload}`, "lualatex"},
		{"commented fontspec ignored", "% \\usepackage{fontspec}", "pdflatex"},
		{"trailing comment still detects", `\usepackage{fontspec} % needed`, "xelatex"},
		{"escaped percent does not comment", `\usepackage{fontspec} \% still code`, "xelatex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectCompiler(tc.preamble); got != tc.want {
				t.Errorf("DetectCompiler(%q) = %q, want %q", tc.preamble, got, tc.want)
			}
		})
	}
}

func TestIsValidCompiler(t *testing.T) {
	for _, ok := range []string{"pdflatex", "xelatex", "lualatex"} {
		if !IsValidCompiler(ok) {
			t.Errorf("%s should be valid", ok)
		}
	}
	for _, bad := range []string{"", "latex", "pdflatex -shell-escape", "--shell-escape", "PDFLATEX"} {
		if IsValidCompiler(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
