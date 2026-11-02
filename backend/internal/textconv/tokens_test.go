package textconv

import (
	"strings"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func TestUnknownCommandBecomesProtectedToken(t *testing.T) {
	src := `Built \texttt{testcontainers-go} pipeline with \textbf{Go} \small tuned`
	text := ToValue(src, model.Span{0, len(src)})
	if text.Mode != "text" {
		t.Fatalf("a field with unknown commands must stay editable: %+v", text)
	}
	want := "Built ⟦\\texttt{testcontainers-go}⟧ pipeline with **Go** ⟦\\small⟧ tuned"
	if text.Value != want {
		t.Fatalf("value %q, want %q", text.Value, want)
	}
	// Editing the plain words keeps every protected piece byte for byte.
	got, err := ToLatex(text, strings.Replace(text.Value, "pipeline", "workflow", 1), src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `\texttt{testcontainers-go}`) || !strings.Contains(got, `\small`) || !strings.Contains(got, "workflow") {
		t.Fatalf("protected pieces lost: %q", got)
	}
}

func TestProtectedTokenCannotBeInvented(t *testing.T) {
	src := `Built \texttt{x} thing`
	text := ToValue(src, model.Span{0, len(src)})
	for _, value := range []string{
		"Built ⟦\\input{/etc/passwd}⟧ thing",      // new token
		"Built ⟦\\texttt{y}⟧ thing",               // changed token
		"Built ⟦\\texttt{x}⟧ ⟦\\texttt{x}⟧ thing", // duplicated token
		"Built ⟦\\texttt{x} thing",                // unbalanced
	} {
		if _, err := ToLatex(text, value, src); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	// A brand-new value (no original tokens) can never carry one.
	if _, err := ToLatex(model.Text{Mode: "text"}, "x ⟦\\input{a}⟧", ""); err == nil {
		t.Error("new value accepted a token")
	}
	// Removing a token is allowed: the user deleted that piece.
	if _, err := ToLatex(text, "Built thing", src); err != nil {
		t.Error(err)
	}
}

func TestOnlyTokensStaysRaw(t *testing.T) {
	src := `\texttt{x}`
	if text := ToValue(src, model.Span{0, len(src)}); text.Mode != "raw" {
		t.Fatalf("nothing to edit, want raw: %+v", text)
	}
}

func TestCommentInsideStaysRaw(t *testing.T) {
	src := "a % hidden\nb"
	if text := ToValue(src, model.Span{0, len(src)}); text.Mode != "raw" {
		t.Fatalf("%+v", text)
	}
}
