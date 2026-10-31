package compile

import (
	"regexp"
	"strings"
)

// Supported compilers (allowlist — a compiler name becomes a latexmk CLI flag,
// so anything outside this set must be rejected to prevent argument injection).
const (
	CompilerPDFLatex = "pdflatex"
	CompilerXeLatex  = "xelatex"
	CompilerLuaLatex = "lualatex"
)

var texProgramRe = regexp.MustCompile(`(?i)^\s*%\s*!\s*TEX\s+program\s*=\s*([A-Za-z]+)`)
var usePackageRe = regexp.MustCompile(`\\(?:usepackage|RequirePackage)\s*(?:\[[^\]]*\])?\s*\{([^}]*)\}`)

var xelatexHints = map[string]bool{
	"fontspec":    true,
	"polyglossia": true,
}

var lualatexHints = map[string]bool{
	"luacode":    true,
	"luatextra":  true,
	"luatexbase": true,
	"luaotfload": true,
}

// IsValidCompiler reports whether name is in the supported compiler allowlist.
func IsValidCompiler(name string) bool {
	switch name {
	case CompilerPDFLatex, CompilerXeLatex, CompilerLuaLatex:
		return true
	}
	return false
}

// DetectCompiler picks a compiler from the preamble. A `% !TEX program`
// comment wins; otherwise fontspec/polyglossia imply xelatex, lualatex-only
// packages imply lualatex, and the default is pdflatex.
func DetectCompiler(preamble string) string {
	for _, line := range strings.Split(preamble, "\n") {
		m := texProgramRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		switch strings.ToLower(m[1]) {
		case CompilerXeLatex:
			return CompilerXeLatex
		case CompilerLuaLatex:
			return CompilerLuaLatex
		case CompilerPDFLatex:
			return CompilerPDFLatex
		}
	}
	for _, line := range strings.Split(preamble, "\n") {
		code := stripComment(line)
		for _, m := range usePackageRe.FindAllStringSubmatch(code, -1) {
			for _, pkg := range strings.Split(m[1], ",") {
				pkg = strings.TrimSpace(pkg)
				if xelatexHints[pkg] {
					return CompilerXeLatex
				}
				if lualatexHints[pkg] {
					return CompilerLuaLatex
				}
			}
		}
	}
	return CompilerPDFLatex
}

// stripComment cuts a line at the first unescaped %.
func stripComment(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] != '%' {
			continue
		}
		backslashes := 0
		for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return line[:i]
		}
	}
	return line
}

// preambleOf returns everything before \begin{document} (the whole input if absent).
func preambleOf(tex string) string {
	if i := strings.Index(tex, `\begin{document}`); i >= 0 {
		return tex[:i]
	}
	return tex
}
