package textconv

import (
	"strings"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func TestRoundTrip(t *testing.T) {
	for _, src := range []string{`34\% faster`, `A -- B`, `C --- D`, `hello~world`, `$\sim$`, `\textbf{bold}`, `\emph{italic}`, `safe text`, `a\&b`, `\href{mailto:a@b.c}{mail me}`, `\href{https://x.dev}{Site}`} {
		text := ToValue(src, model.Span{0, len(src)})
		got, err := ToLatex(text, text.Value, src)
		if err != nil || got != src {
			t.Fatalf("%q (%s) => %q: %v", src, text.Mode, got, err)
		}
	}
}

func TestHrefToValue(t *testing.T) {
	src := `\href{https://github.com/x}{github.com/x}`
	text := ToValue(src, model.Span{0, len(src)})
	if text.Mode != "text" || text.Value != "[github.com/x](https://github.com/x)" {
		t.Fatalf("href value: %+v", text)
	}
	if text.URL == nil || text.URL.Value != "https://github.com/x" {
		t.Fatalf("href url: %+v", text.URL)
	}
	if src[text.URL.Span[0]:text.URL.Span[1]] != "https://github.com/x" {
		t.Fatalf("url span mismatch: %q", src[text.URL.Span[0]:text.URL.Span[1]])
	}
}

func TestFormattingConversion(t *testing.T) {
	text := ToValue(`\textbf{Redis}`, model.Span{0, len(`\textbf{Redis}`)})
	if text.Mode != "text" || text.Value != "**Redis**" {
		t.Fatalf("formatting: %+v", text)
	}
	got, err := ToLatex(text, "**Memcached** now", `\textbf{Redis}`)
	if err != nil || got != `\textbf{Memcached} now` {
		t.Fatalf("bold rewrite: %q %v", got, err)
	}
	// Italic rewrites keep the command used by the original source.
	emph := ToValue(`\emph{Go}`, model.Span{0, len(`\emph{Go}`)})
	got, err = ToLatex(emph, "*Rust*", `\emph{Go}`)
	if err != nil || got != `\emph{Rust}` {
		t.Fatalf("emph rewrite: %q %v", got, err)
	}
	it := ToValue(`\textit{Go}`, model.Span{0, len(`\textit{Go}`)})
	got, err = ToLatex(it, "*Rust*", `\textit{Go}`)
	if err != nil || got != `\textit{Rust}` {
		t.Fatalf("textit rewrite: %q %v", got, err)
	}
	// Link label edits regenerate the href with the same URL.
	link := ToValue(`\href{https://a.dev}{old}`, model.Span{0, len(`\href{https://a.dev}{old}`)})
	got, err = ToLatex(link, "[new](https://a.dev)", `\href{https://a.dev}{old}`)
	if err != nil || got != `\href{https://a.dev}{new}` {
		t.Fatalf("href rewrite: %q %v", got, err)
	}
	// Dangerous URL schemes are rejected.
	if _, err = ToLatex(link, "[x](javascript:alert(1))", `\href{https://a.dev}{old}`); err == nil {
		t.Fatal("accepted javascript: URL")
	}
}

func TestUserEscapes(t *testing.T) {
	text := ToValue(`plain`, model.Span{0, 5})
	got, err := ToLatex(text, `a \* b \[c\]`, `plain`)
	if err != nil || got != `a * b [c]` {
		t.Fatalf("user escapes: %q %v", got, err)
	}
	for _, bad := range []string{`a \n b`, `a \% b`, `a \`} {
		if err := Validate("text", bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func FuzzUnchangedRoundTrip(f *testing.F) {
	for _, sample := range []string{`hello`, `a\%b`, `\textbf{Redis}`, `\emph{Go}`, `a--b`, `$\sim$`, "UTF-8 résumé", `\href{https://a.dev}{site}`} {
		f.Add(sample)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 10000 {
			return
		}
		text := ToValue(src, model.Span{0, len(src)})
		if text.Mode != "text" {
			return
		}
		got, err := ToLatex(text, text.Value, src)
		if err != nil || got != src {
			t.Fatalf("unchanged text rewritten: %q => %q (%v)", src, got, err)
		}
	})
}

func TestValidation(t *testing.T) {
	for _, value := range []string{`\input{secret}`, `name{broken}`, strings.Repeat("a", 10001)} {
		if err := Validate("text", value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	text := ToValue(`hello`, model.Span{0, 5})
	got, err := ToLatex(text, "34% -- now", `hello`)
	if err != nil || got != `34\% -- now` {
		t.Fatalf("converted %q: %v", got, err)
	}
}
