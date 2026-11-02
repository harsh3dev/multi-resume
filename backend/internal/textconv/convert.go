package textconv

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
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
// returns the URL with its absolute span. The two arguments may be separated
// by whitespace, a newline or a comment.
func singleHref(src string, span model.Span) (string, model.Span, bool) {
	nodes := latex.Parse(src, span[0], span[1], latex.Options{})
	if len(nodes) != 1 || nodes[0].Name != `\href` || len(nodes[0].Args) != 2 || nodes[0].Start != span[0] || nodes[0].End != span[1] {
		return "", model.Span{}, false
	}
	arg := nodes[0].Args[0]
	url := src[arg.Inner[0]:arg.Inner[1]]
	if strings.ContainsAny(url, ") \t\n{}") {
		return "", model.Span{}, false
	}
	return url, model.Span{arg.Inner[0], arg.Inner[1]}, true
}

// Protected tokens: an inline piece the converter does not understand (an
// unknown command, a group, a custom macro) is carried through the value as
// ⟦verbatim LaTeX⟧. The rest of the field stays editable text; the token is
// written back unchanged and cannot be invented by an edit (see ToLatex).
const (
	tokOpen  = "⟦"
	tokClose = "⟧"
)

func decode(src string) (string, bool) {
	if strings.Contains(src, tokOpen) || strings.Contains(src, tokClose) {
		return "", false
	}
	var out strings.Builder
	if !decodeNodes(src, latex.Parse(src, 0, len(src), latex.Options{}), &out) {
		return "", false
	}
	value := out.String()
	// A field made only of protected pieces has nothing to edit: keep it raw.
	if strings.Contains(value, tokOpen) && !strings.ContainsFunc(stripTokens(value), func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return "", false
	}
	return value, true
}

// protectedTokens lists the ⟦…⟧ tokens of a value in order; ok is false when
// they are malformed (unbalanced or nested).
func protectedTokens(value string) (tokens []string, ok bool) {
	for i := 0; i < len(value); {
		switch {
		case strings.HasPrefix(value[i:], tokClose):
			return nil, false
		case strings.HasPrefix(value[i:], tokOpen):
			end := strings.Index(value[i+len(tokOpen):], tokClose)
			if end < 0 {
				return nil, false
			}
			inner := value[i+len(tokOpen) : i+len(tokOpen)+end]
			if strings.Contains(inner, tokOpen) {
				return nil, false
			}
			tokens = append(tokens, inner)
			i += len(tokOpen) + end + len(tokClose)
		default:
			i++
		}
	}
	return tokens, true
}

// stripTokens removes well-formed tokens, leaving the editable text.
func stripTokens(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		if strings.HasPrefix(value[i:], tokOpen) {
			if end := strings.Index(value[i:], tokClose); end >= 0 {
				i += end + len(tokClose)
				continue
			}
		}
		out.WriteByte(value[i])
		i++
	}
	return out.String()
}

var mathValues = []struct{ tex, value string }{
	{`$\sim$`, "~"}, {`$>$`, ">"}, {`$<$`, "<"},
	{`$\times$`, "×"}, {`$\cdot$`, "·"}, {`$\rightarrow$`, "→"},
	{`$\geq$`, "≥"}, {`$\leq$`, "≤"}, {`$\pm$`, "±"}, {`$|$`, "|"},
}

// decodeNodes renders the nodes as mini-markdown. It reports false at the
// first construct it cannot represent, so the caller keeps the field raw.
func decodeNodes(src string, nodes []latex.Node, out *strings.Builder) bool {
	prevTokenEnd := -1 // end offset of the token just written, to merge neighbours
	emitToken := func(n latex.Node) {
		raw := src[n.Start:n.End]
		if n.Start == prevTokenEnd {
			// Adjacent to the previous token: extend it (drop its closing mark).
			cur := out.String()
			out.Reset()
			out.WriteString(strings.TrimSuffix(cur, tokClose))
			out.WriteString(raw)
			out.WriteString(tokClose)
		} else {
			out.WriteString(tokOpen)
			out.WriteString(raw)
			out.WriteString(tokClose)
		}
		prevTokenEnd = n.End
	}
	for _, n := range nodes {
		if n.Unclosed {
			return false
		}
		raw := src[n.Start:n.End]
		switch n.Kind {
		case latex.KSpace:
			out.WriteString(raw)
		case latex.KText:
			if !decodeText(raw, out) {
				return false
			}
		case latex.KMath:
			ok := false
			for _, pair := range mathValues {
				if raw == pair.tex {
					out.WriteString(pair.value)
					ok = true
					break
				}
			}
			if !ok {
				emitToken(n)
				continue
			}
		case latex.KCommand:
			if len(n.Name) == 2 && strings.ContainsRune(`%&#_$*[]`, rune(n.Name[1])) {
				out.WriteByte(n.Name[1])
				break
			}
			if !decodeKnownCommand(src, n, out) {
				emitToken(n)
				continue
			}
		case latex.KGroup, latex.KEnv:
			emitToken(n)
			continue
		default: // comments cannot sit inside an editable value
			return false
		}
		prevTokenEnd = -1
	}
	return true
}

// decodeKnownCommand renders the commands that map to mini-markdown. It
// reports false (writing nothing) for anything else, so the caller can
// protect the call as a token.
func decodeKnownCommand(src string, n latex.Node, out *strings.Builder) bool {
	switch n.Name {
	case `\href`:
		if len(n.Args) != 2 {
			return false
		}
		url := src[n.Args[0].Inner[0]:n.Args[0].Inner[1]]
		if strings.ContainsAny(url, ") \t\n{}") {
			return false
		}
		var label strings.Builder
		if !decodeNodes(src, n.Args[1].Nodes, &label) {
			return false
		}
		out.WriteString("[" + label.String() + "](" + url + ")")
		return true
	case `\textbf`, `\textit`, `\emph`, `\underline`, `\uline`:
		if len(n.Args) != 1 {
			return false
		}
		mark := map[string]string{`\textbf`: "**", `\textit`: "*", `\emph`: "*", `\underline`: "", `\uline`: ""}[n.Name]
		var inner strings.Builder
		if !decodeNodes(src, n.Args[0].Nodes, &inner) {
			return false
		}
		out.WriteString(mark + inner.String() + mark)
		return true
	}
	return false
}

func decodeText(src string, out *strings.Builder) bool {
	for i := 0; i < len(src); {
		switch src[i] {
		case '{', '}', '%', '&', '#', '_', '\\', '$':
			return false
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
	return true
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
	// Protected tokens may only be kept from the original, never invented:
	// an edit must not be able to smuggle arbitrary LaTeX into the file.
	if err := checkTokens(original.Value, value); err != nil {
		return "", err
	}
	if original.Mode == "raw" {
		return value, nil
	}
	italic := `\textit`
	if strings.Contains(src[original.Span[0]:original.Span[1]], `\emph`) {
		italic = `\emph`
	}
	return encode(value, italic, existingURLs(original.Value))
}

// encode turns a mini-markdown value into LaTeX source. User-escaped
// characters (\*, \[, \]) become literals; LaTeX specials are escaped.
func encode(value, italic string, known map[string]bool) (string, error) {
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
				encoded, err := encode(inner, italic, known)
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
			encoded, err := encode(inner, italic, known)
			if err != nil {
				return "", err
			}
			out.WriteString(italic)
			out.WriteByte('{')
			out.WriteString(encoded)
			out.WriteByte('}')
			i = next
		case '\xe2': // possibly the start of a protected token (⟦ is E2 9F A6)
			if strings.HasPrefix(value[i:], tokOpen) {
				end := strings.Index(value[i:], tokClose)
				if end < 0 {
					return "", errors.New("malformed protected token")
				}
				out.WriteString(value[i+len(tokOpen) : i+end])
				i += end + len(tokClose)
				continue
			}
			r, size := utf8.DecodeRuneInString(value[i:])
			out.WriteString(encodeRune(r))
			i += size
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
			if !allowedURL(url) && !known[url] {
				return "", errors.New("link URL must use https, http, or mailto")
			}
			encoded, err := encode(label, italic, known)
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
			out.WriteString(encodeRune(r))
			i += size
		}
	}
	return out.String(), nil
}

func encodeRune(r rune) string {
	switch r {
	case '%', '&', '#', '_', '$':
		return `\` + string(r)
	case '–':
		return "--"
	case '—':
		return "---"
	case '~':
		return `$\sim$`
	case '×':
		return `$\times$`
	case '·':
		return `$\cdot$`
	case '→':
		return `$\rightarrow$`
	case '≥':
		return `$\geq$`
	case '≤':
		return `$\leq$`
	case '±':
		return `$\pm$`
	case '|':
		return `$|$`
	}
	return string(r)
}

func marked(value string, start int, mark string) (string, int, bool) {
	end := strings.Index(value[start:], mark)
	if end <= 0 {
		return "", 0, false
	}
	return value[start : start+end], start + end + len(mark), true
}

// existingURLs lists the link targets already present in a value. A link the
// user did not touch keeps its target even when the scheme is unusual (tel:,
// an empty href); only new targets must pass allowedURL.
func existingURLs(value string) map[string]bool {
	known := map[string]bool{}
	for i := 0; i < len(value); i++ {
		if value[i] != ']' || i+1 >= len(value) || value[i+1] != '(' {
			continue
		}
		if end := strings.IndexByte(value[i+2:], ')'); end >= 0 {
			known[value[i+2:i+2+end]] = true
		}
	}
	return known
}

func allowedURL(url string) bool {
	for _, scheme := range []string{"https://", "http://", "mailto:", "tel:"} {
		if strings.HasPrefix(url, scheme) {
			return !strings.ContainsAny(url, "{}\\")
		}
	}
	return false
}

func checkTokens(original, value string) error {
	have, ok := protectedTokens(original)
	if !ok && strings.Contains(original, tokOpen) {
		have = nil
	}
	want, ok := protectedTokens(value)
	if !ok {
		return errors.New("malformed protected token")
	}
	pool := map[string]int{}
	for _, t := range have {
		pool[t]++
	}
	for _, t := range want {
		if pool[t] == 0 {
			return errors.New("protected LaTeX pieces cannot be added or changed")
		}
		pool[t]--
	}
	return nil
}

func Validate(mode, value string) error {
	if len(value) > 10000 || !utf8.ValidString(value) {
		return errors.New("invalid text length or encoding")
	}
	if mode != "text" && mode != "raw" {
		return errors.New("invalid text mode")
	}
	if mode == "text" {
		if _, ok := protectedTokens(value); !ok {
			return errors.New("malformed protected token")
		}
		value = stripTokens(value)
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
