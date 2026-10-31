package checks

import (
	"sort"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/textconv"
)

// CheckRoundTrip tests each distinct editable span in the original source.
// A text that cannot be written back identically is marked raw.
func CheckRoundTrip(src string, tree *model.Resume) []string {
	type candidate struct {
		id    string
		text  *model.Text
		field *model.Field
	}
	items := make([]candidate, 0)
	appendField := func(f *model.Field) {
		if f != nil {
			items = append(items, candidate{id: f.ID, field: f})
		}
	}
	if tree.Header != nil {
		appendField(tree.Header.Name)
		for i := range tree.Header.Items {
			appendField(&tree.Header.Items[i])
		}
	}
	for i := range tree.Sections {
		section := &tree.Sections[i]
		appendField(&section.Title)
		for j := range section.Items {
			item := &section.Items[j]
			if item.Entry != nil {
				for k := range item.Entry.Fields {
					appendField(&item.Entry.Fields[k])
				}
				if item.Entry.Bullets != nil {
					for k := range item.Entry.Bullets.Bullets {
						b := &item.Entry.Bullets.Bullets[k]
						items = append(items, candidate{id: b.ID, text: &b.Text})
					}
				}
			}
			if item.Bullets != nil {
				for k := range item.Bullets.Bullets {
					b := &item.Bullets.Bullets[k]
					items = append(items, candidate{id: b.ID, text: &b.Text})
				}
			}
			if item.Skill != nil {
				appendField(&item.Skill.Label)
				appendField(&item.Skill.Value)
			}
			if item.Paragraph != nil {
				appendField(&item.Paragraph.Text)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return candidateText(items[i]).Span[0] < candidateText(items[j]).Span[0] })
	failed := make([]string, 0)
	for _, c := range items {
		text := candidateText(c)
		if text.Mode == "raw" {
			continue
		}
		value, err := textconv.ToLatex(text, text.Value, src)
		if err != nil || text.Span[0] < 0 || text.Span[1] > len(src) || value != src[text.Span[0]:text.Span[1]] {
			failed = append(failed, c.id)
			if c.field != nil {
				c.field.Mode = "raw"
				if text.Span[0] >= 0 && text.Span[1] <= len(src) {
					c.field.Value = src[text.Span[0]:text.Span[1]]
				}
			}
			if c.text != nil {
				c.text.Mode = "raw"
				if text.Span[0] >= 0 && text.Span[1] <= len(src) {
					c.text.Value = src[text.Span[0]:text.Span[1]]
				}
			}
		}
	}
	return failed
}

func candidateText(c struct {
	id    string
	text  *model.Text
	field *model.Field
}) model.Text {
	if c.field != nil {
		return c.field.Text()
	}
	return *c.text
}
