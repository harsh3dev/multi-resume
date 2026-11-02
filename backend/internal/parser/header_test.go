package parser

import (
	"strings"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func TestHeaderExtraction(t *testing.T) {
	src := `\Huge{Ada Lovelace} \href{mailto:ada@example.com}{\underline{ada@example.com}}`
	header := &model.Header{Span: model.Span{0, len(src)}, Items: []model.Field{}}
	extractHeader(src, header, macroTexts{})
	if header.Name == nil || header.Name.Value != "Ada Lovelace" {
		t.Fatalf("name: %+v", header.Name)
	}
	if len(header.Items) != 1 || header.Items[0].Value != "[ada@example.com](mailto:ada@example.com)" || header.Items[0].URL == nil || header.Items[0].URL.Value != "mailto:ada@example.com" {
		t.Fatalf("items: %+v", header.Items)
	}
}

func TestHeaderContactSplit(t *testing.T) {
	src := `\begin{center}
    \textbf{\Huge \scshape Jane Doe} \\ \vspace{1pt}
    \small 123-456-7890 $|$ \href{mailto:jane@x.com}{\underline{jane@x.com}} $|$ \href{https://jane.dev}{\underline{jane.dev}}
\end{center}`
	header := &model.Header{Span: model.Span{0, len(src)}, Items: []model.Field{}}
	extractHeader(src, header, macroTexts{})
	if len(header.Items) != 3 {
		t.Fatalf("want 3 contact items, got %d: %+v", len(header.Items), header.Items)
	}
	if header.Items[0].Value != "123-456-7890" || header.Items[0].Mode != "text" {
		t.Fatalf("phone item: %+v", header.Items[0])
	}
	if header.Items[1].URL == nil || header.Items[1].URL.Value != "mailto:jane@x.com" {
		t.Fatalf("email item: %+v", header.Items[1])
	}
	if header.Items[2].Value != "[jane.dev](https://jane.dev)" {
		t.Fatalf("site item: %+v", header.Items[2])
	}
	for _, item := range header.Items {
		if strings.Contains(item.Value, "vspace") || strings.Contains(item.Value, "Huge") {
			t.Fatalf("decoration leaked into item: %+v", item)
		}
	}
}

// The header from the plan's "real example": a size switch inside a group,
// an unknown \scshape, and \href arguments on separate lines.
func TestHeaderSwitchGroupAndMultilineHref(t *testing.T) {
	src := "{\\Huge \\textbf{\\scshape Harsh Pandey}} \\\\ \\vspace{5pt}\n" +
		"\\href{mailto:harshpandey.tech@gmail.com}\n{harshpandey.tech@gmail.com} $|$\n" +
		"\\href{https://github.com/harsh3dev}\n{github.com/harsh3dev} $|$ +91 99999 99999\n"
	header := &model.Header{Span: model.Span{0, len(src)}, Items: []model.Field{}}
	extractHeader(src, header, macroTexts{})
	if header.Name == nil || header.Name.Value != "Harsh Pandey" || header.Name.Mode != "text" {
		t.Fatalf("name: %+v", header.Name)
	}
	if got := src[header.Name.Span[0]:header.Name.Span[1]]; got != "Harsh Pandey" {
		t.Fatalf("editable span must be the text only, got %q", got)
	}
	if len(header.Items) != 3 {
		t.Fatalf("want 3 contacts, got %+v", header.Items)
	}
	if header.Items[0].Mode != "text" || header.Items[0].URL == nil || header.Items[0].URL.Value != "mailto:harshpandey.tech@gmail.com" {
		t.Fatalf("multi-line href must parse as a link: %+v", header.Items[0])
	}
	if header.Items[2].Value != "+91 99999 99999" {
		t.Fatalf("phone: %+v", header.Items[2])
	}
}

func TestHeaderNameShapes(t *testing.T) {
	for _, src := range []string{
		`\Huge \scshape Jane Doe \\ a@b.c`,
		`{\LARGE\bfseries Jane Doe}\\ a@b.c`,
		"\\begin{center}\n\\textbf{\\Huge \\scshape Jane Doe} \\\\\n a@b.c\n\\end{center}",
		`\Huge{Jane Doe} \\ a@b.c`,
		`\name{Jane Doe} a@b.c`,
	} {
		header := &model.Header{Span: model.Span{0, len(src)}, Items: []model.Field{}}
		extractHeader(src, header, macroTexts{})
		if header.Name == nil || header.Name.Value != "Jane Doe" {
			t.Errorf("%q: name %+v", src, header.Name)
			continue
		}
		if len(header.Items) != 1 || header.Items[0].Value != "a@b.c" {
			t.Errorf("%q: items %+v", src, header.Items)
		}
	}
}
