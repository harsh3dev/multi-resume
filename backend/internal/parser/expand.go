package parser

import (
	"strconv"
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
)

const (
	maxExpandDepth = 4
	maxExpandSize  = 64 * 1024
)

type replacement struct {
	start, end int
	text       string
}

// expandBody substitutes calls of other custom commands inside body with
// their definitions, binding #1, #2, ... to the real arguments, to a depth
// limit. Fields are always located at the original call site, so no
// position map back is needed: the expansion is only used to classify what a
// command does.
func expandBody(body string, defs map[string]*definition, opt latex.Options, depth int) string {
	if depth <= 0 || len(body) > maxExpandSize {
		return body
	}
	nodes := latex.Parse(body, 0, len(body), opt)
	var reps []replacement
	var walk func(nodes []latex.Node)
	walk = func(nodes []latex.Node) {
		for _, n := range nodes {
			if n.Kind == latex.KCommand {
				if d, ok := defs[n.Name]; ok && len(n.Args) >= requiredArgs(d) {
					reps = append(reps, replacement{n.Start, n.End, expandBody(substitute(d, n, body), defs, opt, depth-1)})
					continue
				}
				for _, a := range n.Args {
					walk(a.Nodes)
				}
			}
			walk(n.Body)
		}
	}
	walk(nodes)
	if len(reps) == 0 {
		return body
	}
	var out strings.Builder
	pos := 0
	for _, r := range reps {
		out.WriteString(body[pos:r.start])
		out.WriteString(r.text)
		pos = r.end
		if out.Len() > maxExpandSize {
			return body
		}
	}
	out.WriteString(body[pos:])
	return out.String()
}

func requiredArgs(d *definition) int {
	if d.OptionalDefault != nil && d.ArgCount > 0 {
		return d.ArgCount - 1
	}
	return d.ArgCount
}

// substitute binds the call's arguments into the definition body.
func substitute(d *definition, call latex.Node, src string) string {
	vals := make([]string, 0, d.ArgCount)
	if d.OptionalDefault != nil && d.ArgCount > 0 {
		if len(call.Opts) > 0 {
			vals = append(vals, src[call.Opts[0].Inner[0]:call.Opts[0].Inner[1]])
		} else {
			vals = append(vals, *d.OptionalDefault)
		}
	}
	for _, a := range call.Args {
		vals = append(vals, src[a.Inner[0]:a.Inner[1]])
	}
	body := d.Body
	for i := len(vals); i >= 1; i-- {
		body = strings.ReplaceAll(body, "#"+strconv.Itoa(i), vals[i-1])
	}
	return body
}

// logicMarkers are conditionals the parser cannot evaluate. A command that
// uses one is marked unknown instead of guessed at.
var logicMarkers = []string{`\ifthenelse`, `\ifdefined`, `\ifx`, `\ifnum`, `\ifdim`, `\iftoggle`, `\ifbool`, `\IfValueT`, `\IfNoValue`, `\if@`, `\ifcase`}

func hasLogic(body string) bool {
	for _, m := range logicMarkers {
		if strings.Contains(body, m) {
			return true
		}
	}
	return false
}
