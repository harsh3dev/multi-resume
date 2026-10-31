// Package tailor implements AI tailoring: project the tree to a small AI view,
// call Anthropic, validate each proposed change, and stream incremental JSON.
package tailor

import (
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// AIItem is one editable or read-only node shown to the model (no spans).
type AIItem struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	MaxLen   int    `json:"maxLen"`
	Hidden   bool   `json:"hidden"`
	ReadOnly bool   `json:"readOnly"`
}

// AIView is the projected resume sent to the model.
type AIView struct {
	Items []AIItem `json:"items"`
}

// maxLenFor is original length × 1.1, rounded up (data-model §5).
func maxLenFor(value string) int {
	n := len(value) + len(value)/10
	if n < len(value)+1 {
		n = len(value) + 1
	}
	return n
}

// BuildAIView projects the tree to ids + values + hidden + maxLen. The AI may
// edit only bullets, skill values, and paragraphs; everything else is marked
// readOnly. Hidden bullets are included (the bullet bank) so the AI can show
// relevant ones.
func BuildAIView(tree *model.Resume) AIView {
	view := AIView{Items: []AIItem{}}
	add := func(id, text string, hidden, readOnly bool) {
		view.Items = append(view.Items, AIItem{ID: id, Text: text, MaxLen: maxLenFor(text), Hidden: hidden, ReadOnly: readOnly})
	}
	if tree.Header != nil {
		if tree.Header.Name != nil {
			add(tree.Header.Name.ID, tree.Header.Name.Value, false, true)
		}
		for _, f := range tree.Header.Items {
			add(f.ID, f.Value, false, true)
		}
	}
	for _, s := range tree.Sections {
		add(s.Title.ID, s.Title.Value, s.Hidden, true)
		for _, it := range s.Items {
			switch {
			case it.Entry != nil:
				for _, f := range it.Entry.Fields {
					add(f.ID, f.Value, it.Entry.Hidden, true)
				}
				if it.Entry.Bullets != nil {
					for _, b := range it.Entry.Bullets.Bullets {
						add(b.ID, b.Text.Value, b.Hidden || it.Entry.Hidden, false)
					}
				}
			case it.Bullets != nil:
				for _, b := range it.Bullets.Bullets {
					add(b.ID, b.Text.Value, b.Hidden, false)
				}
			case it.Skill != nil:
				add(it.Skill.Label.ID, it.Skill.Label.Value, it.Skill.Hidden, true)
				add(it.Skill.Value.ID, it.Skill.Value.Value, it.Skill.Hidden, false)
			case it.Paragraph != nil:
				add(it.Paragraph.Text.ID, it.Paragraph.Text.Value, false, false)
			}
		}
	}
	return view
}

// Editable reports whether the AI may edit this id (bullets, skill values,
// paragraphs only — and only when not hidden-only bank entries? No: hidden
// bullets may be edited; hidden state is separate).
func (v AIView) Editable(id string) (AIItem, bool) {
	for _, it := range v.Items {
		if it.ID == id {
			return it, !it.ReadOnly
		}
	}
	return AIItem{}, false
}
