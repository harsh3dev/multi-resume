package parser

import (
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// extractHeader finds the name and the contact items of the header area.
//
// It works on the nested tree (not on lines or raw text), so a name written
// as `{\Huge \textbf{\scshape Jane Doe}}`, `\Huge{Jane Doe}` or
// `\Huge \scshape Jane Doe \\` is found by its shape. The editable span of
// every field is narrowed to the real text: size switches, style wrappers and
// spacing around it stay in the file untouched.
func extractHeader(src string, header *model.Header, macros macroTexts) {
	extractHeaderWith(src, header, macros, latex.Options{})
}

func extractHeaderWith(src string, header *model.Header, macros macroTexts, opt latex.Options) {
	if header == nil {
		return
	}
	nodes := latex.Parse(src, header.Span[0], header.Span[1], opt)
	cut := model.Span{-1, -1}
	// \Huge-class switches first; \Large is a weaker hint (often a subtitle).
	for _, sizes := range []map[string]bool{
		{`\Huge`: true, `\huge`: true, `\LARGE`: true, `\HUGE`: true},
		{`\Large`: true},
	} {
		if span, region, ok := findName(src, nodes, sizes); ok {
			if start, end, ok := narrow(src, truncateName(region)); ok && hasVisibleText(src[start:end]) {
				f := macros.field(src, model.Span{start, end}, "")
				header.Name = &f
				cut = span
				break
			}
		}
	}
	if items := contactCommandItems(src, nodes, macros); len(items) > 0 {
		header.Items = items
		return
	}
	header.Items = contactItems(src, nodes, cut, macros)
}

// contactCommandItems handles templates that declare each contact with its
// own command (\email{..}, \phone{..}, ...), known from the template
// dictionary. The text of each call's first argument is the editable field.
func contactCommandItems(src string, nodes []latex.Node, macros macroTexts) []model.Field {
	var items []model.Field
	var walk func(nodes []latex.Node)
	walk = func(nodes []latex.Node) {
		for _, n := range nodes {
			if n.Kind == latex.KCommand {
				if t, ok := dictionary.Commands[n.Name]; ok && (t.Role == "contact" || n.Name == `\location`) && len(n.Args) >= 1 {
					if start, end, ok := narrow(src, n.Args[0].Nodes); ok {
						items = append(items, macros.field(src, model.Span{start, end}, ""))
					}
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
	return items
}

// containerCommands wrap their last argument in a box or colour; the content
// is that last argument, so the header search can look inside.
var containerCommands = map[string]bool{
	`\textcolor`: true, `\colorbox`: true, `\fcolorbox`: true, `\parbox`: true,
	`\mbox`: true, `\makebox`: true, `\hbox`: true, `\raisebox`: true, `\fbox`: true,
}

// wrapperBody returns the nodes a wrapper or container command holds.
func wrapperBody(n latex.Node) ([]latex.Node, bool) {
	if n.Kind != latex.KCommand || len(n.Args) == 0 {
		return nil, false
	}
	if latex.WrapperCommands[n.Name] && len(n.Args) == 1 || containerCommands[n.Name] {
		return n.Args[len(n.Args)-1].Nodes, true
	}
	return nil, false
}

// isLayoutEnv reports whether an environment only arranges its children, so
// the header search may look inside it.
func isLayoutEnv(n latex.Node) bool {
	switch n.Name {
	case "center", "flushleft", "flushright", "minipage", "tabular", "tabular*",
		"tabularx", "adjustwidth", "multicols", "multicols*":
		return true
	}
	return false
}

// startsWithSize reports whether the body begins with a size switch (after
// any other switches) and returns the index just past it.
func startsWithSize(nodes []latex.Node, sizes map[string]bool) (int, bool) {
	for i, n := range nodes {
		switch {
		case latex.IsGap(n):
		case n.Kind == latex.KCommand && sizes[n.Name]:
			return i + 1, true
		case n.Kind == latex.KCommand && latex.SwitchCommands[n.Name]:
		default:
			return 0, false
		}
	}
	return 0, false
}

// lineBreak ends the region a size switch applies to.
func lineBreak(n latex.Node) bool {
	return n.Kind == latex.KCommand && (n.Name == `\\` || n.Name == `\newline` || n.Name == `\par`)
}

// findName returns the span of the whole construct that produces the name
// (so contact splitting can leave it out) and the nodes holding its text.
func findName(src string, nodes []latex.Node, sizes map[string]bool) (model.Span, []latex.Node, bool) {
	for i, n := range nodes {
		switch {
		case n.Kind == latex.KCommand && sizes[n.Name]:
			end, grouped := len(nodes), false
			// \Huge{Name}: a group right after the switch is the whole region.
			if k := nextSignificant(nodes, i+1); k < len(nodes) && nodes[k].Kind == latex.KGroup {
				end, grouped = k+1, true
			}
			for j := i + 1; j < len(nodes) && !grouped; j++ {
				if lineBreak(nodes[j]) || (nodes[j].Kind == latex.KCommand && latex.SizeSwitches[nodes[j].Name]) {
					end = j
					break
				}
			}
			region := nodes[i+1 : end]
			if len(latex.Significant(region)) == 0 {
				continue
			}
			return model.Span{n.Start, regionEnd(nodes, i, end)}, region, true
		case n.Kind == latex.KGroup:
			if at, ok := startsWithSize(n.Body, sizes); ok {
				if span, region, ok := regionAfter(n, n.Body, at); ok {
					return span, region, true
				}
			}
			if span, region, ok := findName(src, n.Body, sizes); ok {
				return span, region, true
			}
		case n.Kind == latex.KEnv && isLayoutEnv(n):
			if span, region, ok := findName(src, n.Body, sizes); ok {
				return span, region, true
			}
		case n.Kind == latex.KCommand && wrapperOK(n):
			body, _ := wrapperBody(n)
			if at, ok := startsWithSize(body, sizes); ok {
				if span, region, ok := regionAfter(n, body, at); ok {
					return span, region, true
				}
			}
			if span, region, ok := findName(src, body, sizes); ok {
				return span, region, true
			}
		case n.Kind == latex.KCommand && n.Name == `\name` && len(n.Args) >= 1:
			if len(latex.Significant(n.Args[0].Nodes)) > 0 {
				return model.Span{n.Start, n.End}, n.Args[0].Nodes, true
			}
		}
	}
	return model.Span{}, nil, false
}

// nameSafe reports whether a node can be part of a name: plain text, styling
// and spacing only. Rules, icons, math and unknown commands end the name.
func nameSafe(n latex.Node) bool {
	switch n.Kind {
	case latex.KText, latex.KSpace, latex.KComment:
		return true
	case latex.KGroup:
		for _, c := range n.Body {
			if !nameSafe(c) {
				return false
			}
		}
		return true
	case latex.KCommand:
		if isDecoration(n) || n.Name == `\&` || n.Name == `\.` {
			return true
		}
		if body, ok := wrapperBody(n); ok {
			for _, c := range body {
				if !nameSafe(c) {
					return false
				}
			}
			return true
		}
	}
	return false
}

// truncateName cuts the region at the first node that cannot be name text.
func truncateName(region []latex.Node) []latex.Node {
	for i, n := range region {
		if !nameSafe(n) {
			return region[:i]
		}
	}
	return region
}

func wrapperOK(n latex.Node) bool { _, ok := wrapperBody(n); return ok }

// regionAfter returns the text nodes after a leading size switch inside a
// group-like construct, up to the first line break.
func regionAfter(container latex.Node, body []latex.Node, at int) (model.Span, []latex.Node, bool) {
	end := len(body)
	for j := at; j < len(body); j++ {
		if lineBreak(body[j]) {
			end = j
			break
		}
	}
	region := body[at:end]
	if len(latex.Significant(region)) == 0 {
		return model.Span{}, nil, false
	}
	return model.Span{container.Start, container.End}, region, true
}

func nextSignificant(nodes []latex.Node, from int) int {
	for from < len(nodes) && latex.IsGap(nodes[from]) {
		from++
	}
	return from
}

func regionEnd(nodes []latex.Node, from, end int) int {
	for j := end - 1; j > from; j-- {
		if !latex.IsGap(nodes[j]) {
			return nodes[j].End
		}
	}
	return nodes[from].End
}

// hasVisibleText reports whether the piece contains letters or digits outside
// of commands — pure markup pieces (a lone brace, \begin{center}) are junk.
func hasVisibleText(piece string) bool {
	for _, t := range latex.Tokenize(piece) {
		if t.Kind == latex.TokText && strings.IndexFunc(t.Raw, func(r rune) bool {
			return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r > 127
		}) >= 0 {
			return true
		}
	}
	return false
}

// spacing commands carry layout only, never content.
func isSpacing(n latex.Node) bool {
	if n.Kind != latex.KCommand {
		return false
	}
	switch n.Name {
	case `\vspace`, `\vspace*`, `\hspace`, `\hspace*`, `\smallskip`, `\medskip`, `\bigskip`,
		`\noindent`, `\quad`, `\qquad`, `\,`, `\;`, `\!`, `\ `:
		return true
	}
	return false
}

// isDecoration reports whether a node at the edge of a field is styling or
// spacing that must stay in the file but never be shown as text.
func isDecoration(n latex.Node) bool {
	if latex.IsGap(n) || isSpacing(n) {
		return true
	}
	if n.Kind != latex.KCommand {
		return false
	}
	return latex.SwitchCommands[n.Name] || (len(n.Args) == 0 && strings.HasPrefix(n.Name, `\fa`))
}

// narrow reduces a run of sibling nodes to the editable text: it peels
// decoration off both edges and unwraps a single group, layout environment or
// style wrapper that holds all the content. The returned range is the text
// only; everything outside it is left alone by edits.
func narrow(src string, nodes []latex.Node) (int, int, bool) {
	for {
		sig := latex.Significant(nodes)
		if len(sig) == 0 {
			return 0, 0, false
		}
		if isDecoration(sig[0]) {
			nodes = sig[1:]
			continue
		}
		if last := sig[len(sig)-1]; isDecoration(last) || lineBreak(last) {
			nodes = sig[:len(sig)-1]
			continue
		}
		if len(sig) == 1 {
			n := sig[0]
			switch {
			case n.Kind == latex.KGroup:
				nodes = n.Body
				continue
			case n.Kind == latex.KEnv && isLayoutEnv(n):
				nodes = n.Body
				continue
			case n.Kind == latex.KCommand && latex.WrapperCommands[n.Name] && len(n.Args) == 1:
				nodes = n.Args[0].Nodes
				continue
			case n.Kind == latex.KCommand && n.Name == `\textcolor` && len(n.Args) == 2:
				nodes = n.Args[1].Nodes
				continue
			}
		}
		start, end := sig[0].Start, sig[len(sig)-1].End
		for start < end && (src[start] == ' ' || src[start] == '\t' || src[start] == '\n' || src[start] == '\r' || src[start] == '~') {
			start++
		}
		for end > start && (src[end-1] == ' ' || src[end-1] == '\t' || src[end-1] == '\n' || src[end-1] == '\r' || src[end-1] == '~') {
			end--
		}
		return start, end, start < end
	}
}

// contactSeparators split a header line into separate contact items.
var contactSeparators = map[string]bool{
	`\quad`: true, `\qquad`: true, `\cdot`: true, `\textbullet`: true, `\textbar`: true,
	`\bullet`: true, `\vert`: true, `\hfill`: true, `\\`: true, `\newline`: true, `\par`: true,
}

var separatorMath = map[string]bool{`$|$`: true, `$\cdot$`: true, `$\vert$`: true, `$\bullet$`: true, `$\mid$`: true}

const sepRunes = "|•·∙&"

// withSeparators splits text nodes at literal separator characters so that
// every separator is its own node.
func withSeparators(src string, nodes []latex.Node) []latex.Node {
	out := make([]latex.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.Kind != latex.KText || !strings.ContainsAny(src[n.Start:n.End], sepRunes) {
			out = append(out, n)
			continue
		}
		pos := n.Start
		for i, r := range src[n.Start:n.End] {
			if strings.ContainsRune(sepRunes, r) {
				at := n.Start + i
				if at > pos {
					out = append(out, latex.Node{Kind: latex.KText, Start: pos, End: at})
				}
				out = append(out, latex.Node{Kind: latex.KCommand, Name: `\sep`, Start: at, End: at + len(string(r))})
				pos = at + len(string(r))
			}
		}
		if pos < n.End {
			out = append(out, latex.Node{Kind: latex.KText, Start: pos, End: n.End})
		}
	}
	return out
}

func isSeparator(src string, n latex.Node) bool {
	switch n.Kind {
	case latex.KMath:
		return separatorMath[src[n.Start:n.End]]
	case latex.KCommand:
		return n.Name == `\sep` || contactSeparators[n.Name]
	}
	return false
}

func overlaps(n latex.Node, cut model.Span) bool { return n.Start < cut[1] && cut[0] < n.End }
func inside(n latex.Node, cut model.Span) bool   { return n.Start >= cut[0] && n.End <= cut[1] }

// containsSeparator reports whether any separator sits inside the node's
// own body, so splitting should descend into it.
func containsSeparator(src string, n latex.Node) bool {
	var bodies [][]latex.Node
	switch n.Kind {
	case latex.KGroup, latex.KEnv:
		bodies = append(bodies, n.Body)
	case latex.KCommand:
		if body, ok := wrapperBody(n); ok {
			bodies = append(bodies, body)
		}
	}
	for _, body := range bodies {
		for _, c := range withSeparators(src, body) {
			if isSeparator(src, c) || containsSeparator(src, c) {
				return true
			}
		}
	}
	return false
}

func contactItems(src string, nodes []latex.Node, cut model.Span, macros macroTexts) []model.Field {
	items := []model.Field{}
	flush := func(run []latex.Node) {
		start, end, ok := narrow(src, run)
		if !ok || !hasVisibleText(src[start:end]) {
			return
		}
		f := macros.field(src, model.Span{start, end}, "")
		if f.Mode == "text" && strings.TrimSpace(f.Value) == "" {
			return
		}
		items = append(items, f)
	}
	var walk func(level []latex.Node)
	walk = func(level []latex.Node) {
		var run []latex.Node
		for _, n := range withSeparators(src, level) {
			switch {
			case cut[0] >= 0 && inside(n, cut):
				flush(run)
				run = nil
			case isSeparator(src, n):
				flush(run)
				run = nil
			case (cut[0] >= 0 && overlaps(n, cut) || containsSeparator(src, n) || n.Kind == latex.KEnv && isLayoutEnv(n)) && descend(n) != nil:
				flush(run)
				run = nil
				walk(descend(n))
			default:
				run = append(run, n)
			}
		}
		flush(run)
	}
	walk(nodes)
	return items
}

// descend returns the child sequence to split inside a container node.
func descend(n latex.Node) []latex.Node {
	switch n.Kind {
	case latex.KGroup, latex.KEnv:
		return append([]latex.Node{}, n.Body...)
	case latex.KCommand:
		if body, ok := wrapperBody(n); ok {
			return append([]latex.Node{}, body...)
		}
	}
	return nil
}
