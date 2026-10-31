package latex

import "testing"

func TestTokenize(t *testing.T) {
	for _, src := range []string{
		"comment % here\ntext", `\% escaped % comment`,
		`\newcommand{\x}[2]{\textbf{#1{#2}}}`,
		`\begin{itemize}\item a\end{itemize}`,
		`Math $x+\$y$ and $$z$$`,
		"utf8 résumé {clé}",
	} {
		tokens := Tokenize(src)
		cursor := 0
		for _, tok := range tokens {
			if tok.Start != cursor || tok.End > len(src) || tok.End <= tok.Start || src[tok.Start:tok.End] != tok.Raw {
				t.Fatalf("bad token %#v for %q", tok, src)
			}
			cursor = tok.End
		}
		if cursor != len(src) {
			t.Fatalf("uncovered bytes in %q", src)
		}
	}
	tokens := Tokenize(`\% safe % comment`)
	if tokens[0].Kind != TokCommand || tokens[len(tokens)-1].Kind != TokComment {
		t.Fatalf("escape/comment: %#v", tokens)
	}
}
