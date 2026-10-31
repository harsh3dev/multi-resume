package parser

import (
	"errors"
	"strconv"
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func splitPreambleBody(src string) (model.Span, model.Span, error) {
	tokens := latex.Tokenize(src)
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Kind != latex.TokEnv || tokens[i].Raw != `\begin` {
			continue
		}
		arg, next, ok := group(tokens, i+1, src, latex.TokOpenBrace, latex.TokCloseBrace)
		if ok && strings.TrimSpace(src[arg[0]:arg[1]]) == "document" {
			return model.Span{0, tokens[i].Start}, model.Span{tokens[next-1].End, len(src)}, nil
		}
	}
	return model.Span{}, model.Span{}, errors.New("missing document environment")
}

// group reads a balanced braced or bracketed token group, returning only its interior.
func group(tokens []latex.Token, index int, src string, open, close latex.TokenKind) (model.Span, int, bool) {
	for index < len(tokens) && (tokens[index].Kind == latex.TokNewline || tokens[index].Kind == latex.TokComment || (tokens[index].Kind == latex.TokText && strings.TrimSpace(tokens[index].Raw) == "")) {
		index++
	}
	if index >= len(tokens) || tokens[index].Kind != open {
		return model.Span{}, index, false
	}
	depth := 1
	start := tokens[index].End
	for j := index + 1; j < len(tokens); j++ {
		switch tokens[j].Kind {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return model.Span{start, tokens[j].Start}, j + 1, true
			}
		}
	}
	_ = src
	return model.Span{}, index, false
}

type definition struct {
	model.CommandInfo
	Body string
}

func readDefinitions(tokens []latex.Token, src string) []definition {
	defs := make([]definition, 0)
	for i, tok := range tokens {
		if tok.Kind != latex.TokCommand || !isDefinition(tok.Raw) {
			continue
		}
		j := i + 1
		if j < len(tokens) && tokens[j].Kind == latex.TokText && strings.HasPrefix(tokens[j].Raw, "*") {
			j++
		}
		var name string
		if tok.Raw == `\def` {
			if j >= len(tokens) || tokens[j].Kind != latex.TokCommand {
				continue
			}
			name = tokens[j].Raw
			j++
		} else if j < len(tokens) && tokens[j].Kind == latex.TokCommand {
			name = tokens[j].Raw
			j++
		} else {
			span, next, ok := group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				continue
			}
			name = strings.TrimSpace(src[span[0]:span[1]])
			j = next
		}
		if !strings.HasPrefix(name, `\`) || len(name) < 2 || len(name) > 128 {
			continue
		}
		count := 0
		var optional *string
		if tok.Raw == `\def` {
			for j < len(tokens) && tokens[j].Kind != latex.TokOpenBrace && tokens[j].Kind != latex.TokNewline {
				if tokens[j].Kind == latex.TokText {
					for k := 1; k <= 9; k++ {
						if strings.Contains(tokens[j].Raw, "#"+strconv.Itoa(k)) && k > count {
							count = k
						}
					}
				}
				j++
			}
		} else if tok.Raw == `\NewDocumentCommand` {
			sig, next, ok := group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				continue
			}
			for k := sig[0]; k < sig[1]; k++ {
				if src[k] == 'm' || src[k] == 'o' || src[k] == 'O' {
					count++
					if src[k] == 'O' {
						if end := strings.IndexByte(src[k+1:sig[1]], '}'); end >= 0 && k+1 < sig[1] && src[k+1] == '{' {
							value := src[k+2 : k+1+end]
							optional = &value
							k += end + 1
						}
					}
				}
			}
			j = next
		} else if span, next, ok := group(tokens, j, src, latex.TokOpenBracket, latex.TokCloseBracket); ok {
			n, err := strconv.Atoi(strings.TrimSpace(src[span[0]:span[1]]))
			if err != nil || n < 0 || n > 9 {
				continue
			}
			count, j = n, next
			if span, next, ok = group(tokens, j, src, latex.TokOpenBracket, latex.TokCloseBracket); ok {
				value := src[span[0]:span[1]]
				optional = &value
				j = next
			}
		}
		body, _, ok := group(tokens, j, src, latex.TokOpenBrace, latex.TokCloseBrace)
		if !ok {
			continue
		}
		defs = append(defs, definition{CommandInfo: model.CommandInfo{
			Name: name, ArgCount: count, OptionalDefault: optional, Role: "other", Args: []model.CommandArg{},
		}, Body: src[body[0]:body[1]]})
	}
	return defs
}

func isDefinition(raw string) bool {
	switch raw {
	case `\newcommand`, `\newcommand*`, `\renewcommand`, `\renewcommand*`, `\def`, `\NewDocumentCommand`:
		return true
	}
	return false
}
