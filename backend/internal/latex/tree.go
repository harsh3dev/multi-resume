package latex

import "strings"

// NodeKind classifies a node of the nested tree built by Parse.
type NodeKind int

const (
	KText    NodeKind = iota // visible text (may include punctuation)
	KSpace                   // whitespace-only text or a newline
	KComment                 // % ... up to (not including) the newline
	KMath                    // $...$ or $$...$$
	KCommand                 // \name plus the arguments it consumed
	KGroup                   // { ... }
	KEnv                     // \begin{name}...\end{name}
)

// Arg is one braced or bracketed argument. Start/End include the delimiters,
// Inner excludes them.
type Arg struct {
	Start, End int
	Inner      [2]int
	Nodes      []Node
}

// Node is one element of the tree. Every node keeps its byte range in the
// original source; nothing is normalized. A command's range runs from the
// backslash to the end of its last argument, so any whitespace or comments
// between arguments are inside the range.
type Node struct {
	Kind       NodeKind
	Start, End int
	Name       string // command (with backslash) or environment name
	Opts, Args []Arg
	Body       []Node // KGroup and KEnv contents
	Inner      [2]int // KGroup and KEnv interior
	Unclosed   bool
}

// Arity says how many optional ([..], first) and mandatory ({..}) arguments
// a command or environment takes.
type Arity struct{ Opt, Req int }

// Options carry arities learned from the document's own definitions.
type Options struct {
	Commands map[string]Arity
	Envs     map[string]Arity
}

var builtinCommands = map[string]Arity{
	`\textbf`: {0, 1}, `\textit`: {0, 1}, `\emph`: {0, 1}, `\textsc`: {0, 1},
	`\textsf`: {0, 1}, `\texttt`: {0, 1}, `\textrm`: {0, 1}, `\textnormal`: {0, 1},
	`\textsl`: {0, 1}, `\textup`: {0, 1}, `\underline`: {0, 1}, `\uppercase`: {0, 1},
	`\MakeUppercase`: {0, 1}, `\mbox`: {0, 1}, `\hbox`: {0, 1}, `\phantom`: {0, 1},
	`\hphantom`: {0, 1}, `\vphantom`: {0, 1}, `\footnote`: {0, 1}, `\url`: {0, 1},
	`\href`: {0, 2}, `\name`: {0, 1}, `\hyperref`: {1, 1}, `\textcolor`: {1, 2}, `\color`: {1, 1},
	`\colorbox`: {1, 2}, `\fontsize`: {0, 2}, `\vspace`: {0, 1}, `\vspace*`: {0, 1},
	`\hspace`: {0, 1}, `\hspace*`: {0, 1}, `\setlength`: {0, 2}, `\input`: {0, 1},
	`\include`: {0, 1}, `\includegraphics`: {1, 1}, `\section`: {1, 1}, `\section*`: {1, 1},
	`\subsection`: {1, 1}, `\subsection*`: {1, 1}, `\subsubsection`: {1, 1},
	`\subsubsection*`: {1, 1}, `\item`: {1, 0}, `\\`: {1, 0}, `\raisebox`: {1, 2}, `\fcolorbox`: {0, 3}, `\fbox`: {0, 1},
	`\makebox`: {2, 1}, `\parbox`: {2, 2}, `\scalebox`: {0, 2}, `\resizebox`: {0, 3},
	`\addtolength`: {0, 2}, `\pagecolor`: {0, 1},
}

var builtinEnvs = map[string]Arity{
	"tabular": {1, 1}, "tabular*": {0, 2}, "tabularx": {0, 2}, "tabu": {0, 1},
	"minipage": {1, 1}, "multicols": {0, 1}, "multicols*": {0, 1},
	"itemize": {1, 0}, "enumerate": {1, 0}, "description": {1, 0},
	"adjustwidth": {0, 2}, "paracol": {0, 1},
}

// SwitchCommands change the style of what follows without taking arguments.
// They are skipped, never shown as content, and never rewritten.
var SwitchCommands = map[string]bool{
	`\Huge`: true, `\huge`: true, `\LARGE`: true, `\Large`: true, `\large`: true, `\HUGE`: true,
	`\normalsize`: true, `\small`: true, `\footnotesize`: true, `\scriptsize`: true,
	`\tiny`: true, `\scshape`: true, `\bfseries`: true, `\itshape`: true, `\slshape`: true,
	`\upshape`: true, `\mdseries`: true, `\rmfamily`: true, `\sffamily`: true,
	`\ttfamily`: true, `\normalfont`: true, `\centering`: true, `\raggedright`: true,
	`\raggedleft`: true, `\bf`: true, `\it`: true, `\sc`: true, `\em`: true,
}

// SizeSwitches are the switches that mark a heading-sized line such as a name.
var SizeSwitches = map[string]bool{
	`\Huge`: true, `\huge`: true, `\LARGE`: true, `\Large`: true, `\HUGE`: true,
}

// WrapperCommands keep their single text argument: removing the wrapper
// leaves the same words, only styled differently.
var WrapperCommands = map[string]bool{
	`\textbf`: true, `\textit`: true, `\emph`: true, `\textsc`: true, `\textsf`: true,
	`\texttt`: true, `\textrm`: true, `\textnormal`: true, `\textsl`: true,
	`\textup`: true, `\underline`: true, `\uppercase`: true, `\MakeUppercase`: true,
}

type parser struct {
	src  string
	toks []Token
	i    int
	opt  Options
}

// Parse builds a nested tree for src[from:to]. It never fails and never
// drops bytes: unbalanced or unknown input becomes leaf nodes (or Unclosed
// groups) rather than errors. All offsets refer to src.
func Parse(src string, from, to int, opt Options) []Node {
	toks := Tokenize(src[from:to])
	for i := range toks {
		toks[i].Start += from
		toks[i].End += from
	}
	p := &parser{src: src, toks: toks, opt: opt}
	return p.seq(func() bool { return false })
}

func (p *parser) cmdArity(name string) Arity {
	if a, ok := p.opt.Commands[name]; ok {
		return a
	}
	return builtinCommands[name]
}

func (p *parser) envArity(name string) Arity {
	if a, ok := p.opt.Envs[name]; ok {
		return a
	}
	return builtinEnvs[name]
}

func isSpaceTok(t Token) bool {
	return t.Kind == TokNewline || (t.Kind == TokText && strings.TrimSpace(t.Raw) == "")
}

// seq reads nodes until stop reports true or the tokens run out.
func (p *parser) seq(stop func() bool) []Node {
	var nodes []Node
	for p.i < len(p.toks) && !stop() {
		t := p.toks[p.i]
		switch t.Kind {
		case TokNewline:
			nodes = append(nodes, Node{Kind: KSpace, Start: t.Start, End: t.End})
			p.i++
		case TokText:
			kind := KText
			if strings.TrimSpace(t.Raw) == "" {
				kind = KSpace
			}
			nodes = append(nodes, Node{Kind: kind, Start: t.Start, End: t.End})
			p.i++
		case TokComment:
			nodes = append(nodes, Node{Kind: KComment, Start: t.Start, End: t.End})
			p.i++
		case TokMath:
			nodes = append(nodes, Node{Kind: KMath, Start: t.Start, End: t.End})
			p.i++
		case TokOpenBrace:
			nodes = append(nodes, p.group())
		case TokCommand:
			nodes = append(nodes, p.command())
		case TokEnv:
			if t.Raw == `\begin` {
				if n, ok := p.env(); ok {
					nodes = append(nodes, n)
					continue
				}
			}
			nodes = append(nodes, Node{Kind: KCommand, Name: t.Raw, Start: t.Start, End: t.End})
			p.i++
		default: // stray } [ ]
			nodes = append(nodes, Node{Kind: KText, Start: t.Start, End: t.End})
			p.i++
		}
	}
	return nodes
}

func (p *parser) group() Node {
	open := p.toks[p.i]
	p.i++
	body := p.seq(func() bool { return p.toks[p.i].Kind == TokCloseBrace })
	n := Node{Kind: KGroup, Start: open.Start, Body: body, Inner: [2]int{open.End, open.End}}
	if p.i < len(p.toks) {
		cl := p.toks[p.i]
		p.i++
		n.End, n.Inner[1] = cl.End, cl.Start
	} else {
		n.Unclosed = true
		n.End = p.toks[len(p.toks)-1].End
		n.Inner[1] = n.End
	}
	return n
}

// gap skips whitespace, single newlines and comments starting at i and
// returns the index of the next significant token, or -1 when a blank line
// (paragraph break) intervenes or the tokens end.
func (p *parser) gap(i int) int {
	nl := 0
	for ; i < len(p.toks); i++ {
		t := p.toks[i]
		switch {
		case t.Kind == TokNewline:
			nl++
			if nl >= 2 {
				return -1
			}
		case t.Kind == TokComment:
			nl = 0
		case t.Kind == TokText && strings.TrimSpace(t.Raw) == "":
		default:
			return i
		}
	}
	return -1
}

func (p *parser) readReq() (Arg, bool) {
	j := p.gap(p.i)
	if j < 0 || p.toks[j].Kind != TokOpenBrace {
		return Arg{}, false
	}
	p.i = j
	g := p.group()
	return Arg{Start: g.Start, End: g.End, Inner: g.Inner, Nodes: g.Body}, true
}

// readOpt reads a [..] argument. Brackets nested inside braces do not close it.
func (p *parser) readOpt(allowGap bool) (Arg, bool) {
	j := p.i
	if allowGap {
		if j = p.gap(p.i); j < 0 {
			return Arg{}, false
		}
	}
	if j >= len(p.toks) || p.toks[j].Kind != TokOpenBracket {
		return Arg{}, false
	}
	depth := 0
	for k := j + 1; k < len(p.toks); k++ {
		switch p.toks[k].Kind {
		case TokOpenBrace:
			depth++
		case TokCloseBrace:
			depth--
			if depth < 0 {
				return Arg{}, false // belongs to an enclosing group
			}
		case TokCloseBracket:
			if depth == 0 {
				a := Arg{Start: p.toks[j].Start, End: p.toks[k].End, Inner: [2]int{p.toks[j].End, p.toks[k].Start}}
				p.i = k + 1
				return a, true
			}
		}
		if p.toks[k].Kind == TokNewline && k+1 < len(p.toks) && p.toks[k+1].Kind == TokNewline {
			return Arg{}, false // blank line: not an argument
		}
	}
	return Arg{}, false
}

func (p *parser) command() Node {
	t := p.toks[p.i]
	p.i++
	n := Node{Kind: KCommand, Name: t.Raw, Start: t.Start, End: t.End}
	ar := p.cmdArity(t.Raw)
	immediate := t.Raw == `\\` || t.Raw == `\item`
	p.readArgs(&n, ar, !immediate)
	return n
}

func (p *parser) readArgs(n *Node, ar Arity, allowGap bool) {
	for k := 0; k < ar.Opt; k++ {
		a, ok := p.readOpt(allowGap)
		if !ok {
			break
		}
		n.Opts = append(n.Opts, a)
		n.End = a.End
	}
	for k := 0; k < ar.Req; k++ {
		a, ok := p.readReq()
		if !ok {
			break
		}
		n.Args = append(n.Args, a)
		n.End = a.End
	}
}

// env reads \begin{name}[opt]{args} body \end{name}. It reports false (and
// consumes nothing) when the begin has no readable name.
func (p *parser) env() (Node, bool) {
	save := p.i
	begin := p.toks[p.i]
	p.i++
	nameArg, ok := p.readReq()
	if !ok {
		p.i = save
		return Node{}, false
	}
	name := strings.TrimSpace(p.src[nameArg.Inner[0]:nameArg.Inner[1]])
	n := Node{Kind: KEnv, Name: name, Start: begin.Start, End: nameArg.End}
	p.readArgs(&n, p.envArity(name), true)
	bodyStart := n.End
	isEnd := func() bool {
		t := p.toks[p.i]
		if t.Kind != TokEnv || t.Raw != `\end` {
			return false
		}
		j := p.gap(p.i + 1)
		if j < 0 || p.toks[j].Kind != TokOpenBrace {
			return false
		}
		for k := j + 1; k < len(p.toks); k++ {
			if p.toks[k].Kind == TokCloseBrace {
				return strings.TrimSpace(p.src[p.toks[j].End:p.toks[k].Start]) == name
			}
		}
		return false
	}
	n.Body = p.seq(isEnd)
	n.Inner = [2]int{bodyStart, bodyStart}
	if p.i < len(p.toks) {
		endTok := p.toks[p.i]
		n.Inner[1] = endTok.Start
		p.i++
		if closeArg, ok := p.readReq(); ok {
			n.End = closeArg.End
		} else {
			n.End = endTok.End
		}
	} else {
		n.Unclosed = true
		n.Inner[1] = p.toks[len(p.toks)-1].End
		n.End = n.Inner[1]
	}
	return n, true
}

// IsGap reports whether the node carries no visible content.
func IsGap(n Node) bool { return n.Kind == KSpace || n.Kind == KComment }

// Significant returns the nodes with leading and trailing gaps removed.
func Significant(nodes []Node) []Node {
	i, j := 0, len(nodes)
	for i < j && IsGap(nodes[i]) {
		i++
	}
	for j > i && IsGap(nodes[j-1]) {
		j--
	}
	return nodes[i:j]
}
