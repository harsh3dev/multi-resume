package textconv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
	"github.com/harsh3dev/multi-resume/backend/internal/textconv"
)

// Property test (data-model §3): every text-mode span in every fixture must
// convert value → LaTeX back to the exact original source slice.
func TestPropertyAllFixtureSpans(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/resumes/*.tex")
	if err != nil || len(paths) == 0 {
		t.Fatalf("missing fixtures: %v", err)
	}
	checked := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		tree, err := parser.Parse(src, parser.ParseOptions{})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		texts := collectText(tree)
		for _, text := range texts {
			if text.Mode != "text" {
				continue
			}
			got, err := textconv.ToLatex(text, text.Value, src)
			if err != nil {
				t.Fatalf("%s span %v: %v", path, text.Span, err)
			}
			if got != src[text.Span[0]:text.Span[1]] {
				t.Fatalf("%s span %v: %q != %q", path, text.Span, got, src[text.Span[0]:text.Span[1]])
			}
			checked++
		}
	}
	t.Logf("verified %d text-mode spans across %d fixtures", checked, len(paths))
}

func collectText(r *model.Resume) []model.Text {
	var out []model.Text
	add := func(f model.Field) { out = append(out, f.Text()) }
	if r.Header != nil {
		if r.Header.Name != nil {
			add(*r.Header.Name)
		}
		for _, f := range r.Header.Items {
			add(f)
		}
	}
	for _, s := range r.Sections {
		add(s.Title)
		for _, item := range s.Items {
			if item.Entry != nil {
				for _, f := range item.Entry.Fields {
					add(f)
				}
				if item.Entry.Bullets != nil {
					for _, b := range item.Entry.Bullets.Bullets {
						out = append(out, b.Text)
					}
				}
			}
			if item.Bullets != nil {
				for _, b := range item.Bullets.Bullets {
					out = append(out, b.Text)
				}
			}
			if item.Skill != nil {
				add(item.Skill.Label)
				add(item.Skill.Value)
			}
			if item.Paragraph != nil {
				add(item.Paragraph.Text)
			}
		}
	}
	return out
}
