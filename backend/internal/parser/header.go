package parser

import (
	"regexp"
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func extractHeader(src string, header *model.Header, macros macroTexts) {
	if header == nil {
		return
	}
	tokens := latex.Tokenize(src[header.Span[0]:header.Span[1]])
	for i := range tokens {
		tokens[i].Start += header.Span[0]
		tokens[i].End += header.Span[0]
	}
	// nameCall is the full command call producing the name (e.g. \Huge{...}),
	// so contact splitting can exclude the whole construct, not just the text.
	nameCall := model.Span{-1, -1}
	for i, tok := range tokens {
		if tok.Kind != latex.TokCommand {
			continue
		}
		if tok.Raw == `\name` || tok.Raw == `\Huge` || tok.Raw == `\LARGE` || tok.Raw == `\huge` {
			span, next, ok := group(tokens, i+1, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				continue
			}
			text := stripDecoration(src, span)
			if text[0] == text[1] {
				continue
			}
			f := macros.field(src, text, "")
			header.Name = &f
			nameCall = model.Span{tok.Start, tokens[next-1].End}
			break
		}
	}
	if header.Name == nil {
		for _, marker := range []string{`\Huge`, `\LARGE`, `\huge`} {
			for _, token := range tokens {
				if token.Kind != latex.TokCommand || token.Raw != marker {
					continue
				}
				start := token.Start + len(marker)
				for start < header.Span[1] {
					if src[start] == ' ' || src[start] == '\n' {
						start++
						continue
					}
					if strings.HasPrefix(src[start:], `\scshape`) {
						start += len(`\scshape`)
						continue
					}
					break
				}
				end := start
				for end < header.Span[1] && src[end] != '}' && src[end] != '\\' && src[end] != '\n' {
					end++
				}
				if end > start {
					f := macros.field(src, model.Span{start, end}, "")
					header.Name = &f
					nameCall = model.Span{start, end}
					break
				}
			}
			if header.Name != nil {
				break
			}
		}
	}
	header.Items = contactItems(src, header, nameCall, macros)
}

// headerSeparators split contact lines into individual items (step 5).
var headerSeparators = []string{`$|$`, `\|`, `\quad`, `\\`}

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

var trailingEnvEnd = regexp.MustCompile(`\\end\{[^}]*\}\s*$`)

var sizeCommands = map[string]bool{
	`\small`: true, `\footnotesize`: true, `\normalsize`: true,
	`\large`: true, `\Large`: true, `\LARGE`: true, `\huge`: true, `\Huge`: true,
	`\scshape`: true,
}

// trimBlank drops leading/trailing comments, newlines, and whitespace-only
// text. Token-based, so escaped \% is never mistaken for a comment.
func trimBlank(src string, start, end int) (int, int, bool) {
	tokens := latex.Tokenize(src[start:end])
	i, j := 0, len(tokens)
	blank := func(t latex.Token) bool {
		return t.Kind == latex.TokComment || t.Kind == latex.TokNewline || (t.Kind == latex.TokText && strings.TrimSpace(t.Raw) == "")
	}
	for i < j && blank(tokens[i]) {
		i++
	}
	for j > i && blank(tokens[j-1]) {
		j--
	}
	if i == j {
		return start, start, start != end
	}
	ns, ne := start+tokens[i].Start, start+tokens[j-1].End
	return ns, ne, ns != start || ne != end
}

func stripDecoration(src string, span model.Span) model.Span {
	start, end := span[0], span[1]
	for start < end {
		moved := false
		if ns, ne, ok := trimBlank(src, start, end); ok {
			start, end = ns, ne
			moved = true
		}
		if start < end && src[start] == '\\' {
			cmdEnd := start + 1
			for cmdEnd < end && ((src[cmdEnd] >= 'a' && src[cmdEnd] <= 'z') || (src[cmdEnd] >= 'A' && src[cmdEnd] <= 'Z')) {
				cmdEnd++
			}
			cmd := src[start:cmdEnd]
			switch {
			case cmd == `\textbf` || cmd == `\underline`:
				// Unwrap only a group that is balanced and ends the span.
				if cmdEnd < end && src[cmdEnd] == '{' && src[end-1] == '}' {
					if close, ok := closingCommentGroup(src, cmdEnd+1, end); ok && close == end-1 {
						start, end = cmdEnd+1, end-1
						moved = true
					}
				}
			case sizeCommands[cmd] || strings.HasPrefix(cmd, `\fa`):
				start = cmdEnd
				moved = true
			case cmd == `\vspace` || cmd == `\hspace`:
				cursor := cmdEnd
				if cursor < end && src[cursor] == '*' {
					cursor++
				}
				if cursor < end && src[cursor] == '{' {
					if close, ok := closingCommentGroup(src, cursor+1, end); ok {
						start = close + 1
						moved = true
					}
				}
			case cmd == `\begin`:
				if cmdEnd < end && src[cmdEnd] == '{' {
					if close, ok := closingCommentGroup(src, cmdEnd+1, end); ok {
						start = close + 1
						moved = true
					}
				}
			}
		}
		if m := trailingEnvEnd.FindStringIndex(src[start:end]); m != nil && m[0] > 0 {
			end = start + m[0]
			moved = true
		}
		if !moved {
			break
		}
	}
	// trimBlank keeps partially-blank text tokens; drop their edge whitespace.
	for start < end && (src[start] == ' ' || src[start] == '\t' || src[start] == '\n' || src[start] == '\r') {
		start++
	}
	for end > start && (src[end-1] == ' ' || src[end-1] == '\t' || src[end-1] == '\n' || src[end-1] == '\r') {
		end--
	}
	return model.Span{start, end}
}

func contactItems(src string, header *model.Header, nameCall model.Span, macros macroTexts) []model.Field {
	items := []model.Field{}
	start, end := header.Span[0], header.Span[1]
	var cut func(a, b int)
	cut = func(a, b int) {
		if nameCall[0] >= a && nameCall[1] <= b && nameCall[0] >= 0 {
			cut(a, nameCall[0])
			cut(nameCall[1], b)
			return
		}
		span := stripDecoration(src, model.Span{a, b})
		if span[0] >= span[1] || !hasVisibleText(src[span[0]:span[1]]) {
			return
		}
		f := macros.field(src, span, "")
		if f.Mode == "text" && strings.TrimSpace(f.Value) == "" {
			return
		}
		items = append(items, f)
	}
	cursor := start
	for i := start; i < end; {
		matched := false
		for _, sep := range headerSeparators {
			if strings.HasPrefix(src[i:end], sep) {
				cut(cursor, i)
				i += len(sep)
				cursor = i
				matched = true
				break
			}
		}
		if !matched {
			i++
		}
	}
	cut(cursor, end)
	return items
}
