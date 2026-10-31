package textconv

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// ToValue is deliberately conservative: unfamiliar commands stay raw, never
// silently lose source formatting. The span must belong to this exact source.
func ToValue(src string, span model.Span) model.Text {
	if span[0] < 0 || span[1] < span[0] || span[1] > len(src) {
		return model.Text{Mode: "raw", Span: span}
	}
	raw := src[span[0]:span[1]]
	text := model.Text{Value: raw, Mode: "raw", Span: span}
	if !utf8.ValidString(raw) {
		return text
	}
	value, ok := decode(raw)
	if ok {
		text.Value, text.Mode = value, "text"
		if url, urlSpan, ok := singleHref(src, span); ok {
			text.URL = &model.URL{Value: url, Span: urlSpan}
		}
	}
	return text
}

// singleHref reports whether the span is exactly one \href{url}{text} call and
// returns the URL with its absolute span.
func singleHref(src string, span model.Span) (string, model.Span, bool) {
	raw := src[span[0]:span[1]]
	if !strings.HasPrefix(raw, `\href{`) {
		return "", model.Span{}, false
	}
	urlStart := len(`\href{`)
	urlEnd, ok := closingBrace(raw, urlStart)
	if !ok || urlEnd+1 >= len(raw) || raw[urlEnd+1] != '{' {
		return "", model.Span{}, false
	}
	textEnd, ok := closingBrace(raw, urlEnd+2)
	if !ok || textEnd != len(raw)-1 {
		return "", model.Span{}, false
	}
	url := raw[urlStart:urlEnd]
	if strings.ContainsAny(url, ") \t\n{}") {
		return "", model.Span{}, false
	}
	return url, model.Span{span[0] + urlStart, span[0] + urlEnd}, true
}

func decode(src string) (string, bool) {
	var out strings.Builder
	for i := 0; i < len(src); {
		switch src[i] {
		case '\\':
			if i+1 >= len(src) {
				return "", false
			}
			if strings.ContainsRune(`%&#_$*[]`, rune(src[i+1])) {
				out.WriteByte(src[i+1])
				i += 2
				continue
			}
			if strings.HasPrefix(src[i:], `\href{`) {
				urlStart := i + len(`\href{`)
				urlEnd, ok := closingBrace(src, urlStart)
				if !ok || urlEnd+1 >= len(src) || src[urlEnd+1] != '{' {
					return "", false
				}
				textEnd, ok := closingBrace(src, urlEnd+2)
				if !ok {
					return "", false
				}
				url := src[urlStart:urlEnd]
				label, ok := decode(src[urlEnd+2 : textEnd])
				if !ok || strings.ContainsAny(url, ") \t\n{}") {
					return "", false
				}
				out.WriteByte('[')
				out.WriteString(label)
				out.WriteString("](")
				out.WriteString(url)
				out.WriteByte(')')
				i = textEnd + 1
				continue
			}
			command, mark := "", ""
			for _, item := range []struct{ prefix, marker string }{
				{`\textbf{`, "**"}, {`\textit{`, "*"}, {`\emph{`, "*"},
			} {
				if strings.HasPrefix(src[i:], item.prefix) {
					command, mark = item.prefix, item.marker
					break
				}
			}
			if command == "" && strings.HasPrefix(src[i:], `\underline{`) {
				start := i + len(`\underline{`)
				end, ok := closingBrace(src, start)
				if !ok {
					return "", false
				}
				inner, ok := decode(src[start:end])
				if !ok {
					return "", false
				}
				out.WriteString(inner)
				i = end + 1
				continue
			}
			if command != "" {
				start := i + len(command)
				end, ok := closingBrace(src, start)
				if !ok {
					return "", false
				}
				inner, ok := decode(src[start:end])
				if !ok {
					return "", false
				}
				out.WriteString(mark)
				out.WriteString(inner)
				out.WriteString(mark)
				i = end + 1
				continue
			}
			return "", false
		case '{', '}', '%', '&', '#', '_':
			return "", false
		case '$':
			matched := false
			for _, pair := range []struct{ tex, value string }{
				{`$\sim$`, "~"}, {`$>$`, ">"}, {`$<$`, "<"},
				{`$\times$`, "×"}, {`$\cdot$`, "·"}, {`$\rightarrow$`, "→"},
				{`$\geq$`, "≥"}, {`$\leq$`, "≤"}, {`$\pm$`, "±"}, {`$|$`, "|"},
			} {
				if strings.HasPrefix(src[i:], pair.tex) {
					out.WriteString(pair.value)
					i += len(pair.tex)
					matched = true
					break
				}
			}
			if !matched {
				return "", false
			}
			continue
		case '~':
			out.WriteByte(' ')
			i++
		case '-':
			if strings.HasPrefix(src[i:], "---") {
				out.WriteString("—")
				i += 3
				continue
			}
			if strings.HasPrefix(src[i:], "--") {
				out.WriteString("–")
				i += 2
				continue
			}
			out.WriteByte('-')
			i++
		default:
			out.WriteByte(src[i])
			i++
		}
	}
	return out.String(), true
}

func closingBrace(src string, start int) (int, bool) {
	depth := 1
	for i := start; i < len(src); i++ {
		if src[i] == '\\' && i+1 < len(src) {
			i++
			continue
		}
		if src[i] == '{' {
			depth++
		}
		if src[i] == '}' {
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

// ToLatex preserves the original source for an unchanged value, including its
// exact choice of LaTeX escapes. Changed values are re-encoded from the
// mini-markdown form: **bold**, *italic*, [text](url), with the italic command
// matching the one used in the original source.
func ToLatex(original model.Text, value string, src string) (string, error) {
	if original.Span[0] < 0 || original.Span[1] < original.Span[0] || original.Span[1] > len(src) {
		return "", errors.New("invalid source span")
	}
	// An unchanged value came from decode of this exact span; write the
	// original slice back verbatim (check 1 depends on this).
	if value == original.Value {
		return src[original.Span[0]:original.Span[1]], nil
	}
	if err := Validate(original.Mode, value); err != nil {
		return "", err
	}
	if original.Mode == "raw" {
		return value, nil
	}
	italic := `\textit`
	if strings.Contains(src[original.Span[0]:original.Span[1]], `\emph`) {
		italic = `\emph`
	}
	return encode(value, italic)
}

// encode turns a mini-markdown value into LaTeX source. User-escaped
// characters (\*, \[, \]) become literals; LaTeX specials are escaped.
func encode(value, italic string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(value); {
		switch value[i] {
		case '\\':
			// Validate already restricted backslash to \*, \[, \].
			if i+1 >= len(value) {
				return "", errors.New("dangling escape")
			}
			out.WriteByte(value[i+1])
			i += 2
		case '*':
			if strings.HasPrefix(value[i:], "**") {
				inner, next, ok := marked(value, i+2, "**")
				if !ok {
					out.WriteByte('*')
					i++
					continue
				}
				encoded, err := encode(inner, italic)
				if err != nil {
					return "", err
				}
				out.WriteString(`\textbf{`)
				out.WriteString(encoded)
				out.WriteByte('}')
				i = next
				continue
			}
			inner, next, ok := marked(value, i+1, "*")
			if !ok {
				out.WriteByte('*')
				i++
				continue
			}
			encoded, err := encode(inner, italic)
			if err != nil {
				return "", err
			}
			out.WriteString(italic)
			out.WriteByte('{')
			out.WriteString(encoded)
			out.WriteByte('}')
			i = next
		case '[':
			close := strings.IndexByte(value[i:], ']')
			if close < 0 || i+close+1 >= len(value) || value[i+close+1] != '(' {
				out.WriteByte('[')
				i++
				continue
			}
			label := value[i+1 : i+close]
			rest := value[i+close+2:]
			paren := strings.IndexByte(rest, ')')
			if paren < 0 {
				out.WriteByte('[')
				i++
				continue
			}
			url := rest[:paren]
			if !allowedURL(url) {
				return "", errors.New("link URL must use https, http, or mailto")
			}
			encoded, err := encode(label, italic)
			if err != nil {
				return "", err
			}
			out.WriteString(`\href{`)
			out.WriteString(url)
			out.WriteString(`}{`)
			out.WriteString(encoded)
			out.WriteByte('}')
			i += close + 2 + paren + 1
		default:
			r, size := utf8.DecodeRuneInString(value[i:])
			switch r {
			case '%', '&', '#', '_', '$':
				out.WriteByte('\\')
				out.WriteRune(r)
			case '–':
				out.WriteString("--")
			case '—':
				out.WriteString("---")
			case '~':
				out.WriteString(`$\sim$`)
			case '×':
				out.WriteString(`$\times$`)
			case '·':
				out.WriteString(`$\cdot$`)
			case '→':
				out.WriteString(`$\rightarrow$`)
			case '≥':
				out.WriteString(`$\geq$`)
			case '≤':
				out.WriteString(`$\leq$`)
			case '±':
				out.WriteString(`$\pm$`)
			case '|':
				out.WriteString(`$|$`)
			default:
				out.WriteRune(r)
			}
			i += size
		}
	}
	return out.String(), nil
}

func marked(value string, start int, mark string) (string, int, bool) {
	end := strings.Index(value[start:], mark)
	if end <= 0 {
		return "", 0, false
	}
	return value[start : start+end], start + end + len(mark), true
}

func allowedURL(url string) bool {
	for _, scheme := range []string{"https://", "http://", "mailto:"} {
		if strings.HasPrefix(url, scheme) {
			return !strings.ContainsAny(url, "{}\\")
		}
	}
	return false
}

func Validate(mode, value string) error {
	if len(value) > 10000 || !utf8.ValidString(value) {
		return errors.New("invalid text length or encoding")
	}
	if mode != "text" && mode != "raw" {
		return errors.New("invalid text mode")
	}
	if mode == "text" {
		if strings.ContainsAny(value, "{}") {
			return errors.New("LaTeX braces are not allowed in text mode")
		}
		// Backslash is only allowed to escape the mini-markdown characters
		// a user may type literally: \*, \[, \].
		for i := 0; i < len(value); i++ {
			if value[i] != '\\' {
				continue
			}
			if i+1 >= len(value) || !strings.ContainsRune("*[]", rune(value[i+1])) {
				return errors.New("unsupported escape in text mode")
			}
			i++
		}
	}
	return nil
}
