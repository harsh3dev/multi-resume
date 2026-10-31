package latex

type TokenKind int

const (
	TokText TokenKind = iota
	TokCommand
	TokComment
	TokOpenBrace
	TokCloseBrace
	TokOpenBracket
	TokCloseBracket
	TokMath
	TokNewline
	TokEnv
)

type Token struct {
	Kind  TokenKind
	Start int
	End   int
	Raw   string
}

// Tokenize never normalizes source; all offsets refer to the original bytes.
func Tokenize(src string) []Token {
	tokens := make([]Token, 0, len(src)/6)
	add := func(kind TokenKind, start, end int) {
		tokens = append(tokens, Token{Kind: kind, Start: start, End: end, Raw: src[start:end]})
	}
	for i := 0; i < len(src); {
		start := i
		switch src[i] {
		case '%':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			add(TokComment, start, i)
		case '\n':
			i++
			add(TokNewline, start, i)
		case '{':
			i++
			add(TokOpenBrace, start, i)
		case '}':
			i++
			add(TokCloseBrace, start, i)
		case '[':
			i++
			add(TokOpenBracket, start, i)
		case ']':
			i++
			add(TokCloseBracket, start, i)
		case '$':
			i++
			end := byte('$')
			if i < len(src) && src[i] == '$' {
				i++
				end = 0
			}
			for i < len(src) {
				if src[i] == '\\' && i+1 < len(src) {
					i += 2
					continue
				}
				if src[i] == '$' {
					i++
					if end == 0 && i < len(src) && src[i] == '$' {
						i++
					}
					break
				}
				i++
			}
			add(TokMath, start, i)
		case '\\':
			i++
			if i < len(src) && ((src[i] >= 'a' && src[i] <= 'z') || (src[i] >= 'A' && src[i] <= 'Z') || src[i] == '@') {
				for i < len(src) && ((src[i] >= 'a' && src[i] <= 'z') || (src[i] >= 'A' && src[i] <= 'Z') || src[i] == '@') {
					i++
				}
				if i < len(src) && src[i] == '*' {
					i++
				}
			} else if i < len(src) {
				i++
			}
			kind := TokCommand
			if src[start:i] == `\begin` || src[start:i] == `\end` {
				kind = TokEnv
			}
			add(kind, start, i)
		default:
			for i < len(src) && !special(src[i]) {
				i++
			}
			add(TokText, start, i)
		}
	}
	return tokens
}

func special(c byte) bool {
	switch c {
	case '%', '\n', '{', '}', '[', ']', '$', '\\':
		return true
	}
	return false
}
