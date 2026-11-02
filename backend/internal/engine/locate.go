package engine

import (
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// textRef is a mutable handle to an editable text node anywhere in the tree:
// entry fields, titles, header items, skill parts, paragraph text, and the
// text of a bullet (addressed by the bullet's own ID).
type textRef struct {
	id   string
	text model.Text
	set  func(model.Text)
}

func findTextRef(tree *model.Resume, id string) *textRef {
	var found *textRef
	walkTextRefs(tree, func(r *textRef) bool {
		if r.id == id {
			found = r
			return true
		}
		return false
	})
	return found
}

// walkTextRefs visits every editable text node in the tree. Returning true
// from fn stops the walk.
func walkTextRefs(tree *model.Resume, fn func(*textRef) bool) {
	fieldRef := func(f *model.Field) *textRef {
		if f == nil {
			return nil
		}
		return &textRef{
			id:   f.ID,
			text: f.Text(),
			set:  func(t model.Text) { f.Value, f.Mode, f.URL = t.Value, t.Mode, t.URL },
		}
	}
	visit := func(r *textRef) bool { return r != nil && fn(r) }
	if tree.Header != nil {
		if visit(fieldRef(tree.Header.Name)) {
			return
		}
		for i := range tree.Header.Items {
			if visit(fieldRef(&tree.Header.Items[i])) {
				return
			}
		}
	}
	for si := range tree.Sections {
		s := &tree.Sections[si]
		if visit(fieldRef(&s.Title)) {
			return
		}
		for ii := range s.Items {
			it := &s.Items[ii]
			stop := false
			walkItemTexts(it, func(r *textRef) { stop = stop || fn(r) })
			if stop {
				return
			}
		}
	}
}

func walkItemTexts(it *model.SectionItem, fn func(*textRef)) {
	var bulletRefs func(*model.BulletList)
	bulletRefs = func(list *model.BulletList) {
		if list == nil {
			return
		}
		for i := range list.Bullets {
			b := &list.Bullets[i]
			fn(&textRef{id: b.ID, text: b.Text, set: func(t model.Text) { b.Text = t }})
			for j := range b.Children {
				bulletRefs(&b.Children[j])
			}
		}
	}
	switch {
	case it.Entry != nil:
		for fi := range it.Entry.Fields {
			f := &it.Entry.Fields[fi]
			fn(&textRef{id: f.ID, text: f.Text(), set: func(t model.Text) { f.Value, f.Mode, f.URL = t.Value, t.Mode, t.URL }})
		}
		bulletRefs(it.Entry.Bullets)
	case it.Bullets != nil:
		bulletRefs(it.Bullets)
	case it.Skill != nil:
		for _, f := range []*model.Field{&it.Skill.Label, &it.Skill.Value} {
			fn(&textRef{id: f.ID, text: f.Text(), set: func(t model.Text) { f.Value, f.Mode, f.URL = t.Value, t.Mode, t.URL }})
		}
	case it.Paragraph != nil:
		f := &it.Paragraph.Text
		fn(&textRef{id: f.ID, text: f.Text(), set: func(t model.Text) { f.Value, f.Mode, f.URL = t.Value, t.Mode, t.URL }})
	}
}

// bulletLoc locates a bullet inside its parent list.
type bulletLoc struct {
	list   *model.BulletList
	index  int
	bullet *model.Bullet
}

func findBulletInList(list *model.BulletList, id string) *bulletLoc {
	if list == nil {
		return nil
	}
	for i := range list.Bullets {
		if list.Bullets[i].ID == id {
			return &bulletLoc{list: list, index: i, bullet: &list.Bullets[i]}
		}
	}
	return nil
}

func eachList(tree *model.Resume, fn func(list *model.BulletList, entry *model.Entry, section *model.Section) bool) {
	var visit func(*model.BulletList, *model.Entry, *model.Section) bool
	visit = func(list *model.BulletList, entry *model.Entry, section *model.Section) bool {
		if fn(list, entry, section) {
			return true
		}
		for i := range list.Bullets {
			for j := range list.Bullets[i].Children {
				if visit(&list.Bullets[i].Children[j], nil, section) {
					return true
				}
			}
		}
		return false
	}
	for si := range tree.Sections {
		s := &tree.Sections[si]
		for ii := range s.Items {
			it := &s.Items[ii]
			if it.Entry != nil && it.Entry.Bullets != nil {
				if visit(it.Entry.Bullets, it.Entry, s) {
					return
				}
			}
			if it.Bullets != nil {
				if visit(it.Bullets, nil, s) {
					return
				}
			}
		}
	}
}

func findBullet(tree *model.Resume, id string) *bulletLoc {
	var found *bulletLoc
	eachList(tree, func(list *model.BulletList, _ *model.Entry, _ *model.Section) bool {
		if bl := findBulletInList(list, id); bl != nil {
			found = bl
			return true
		}
		return false
	})
	return found
}

// listLoc locates a bullet list and its owner (entry or bare in a section).
type listLoc struct {
	list    *model.BulletList
	entry   *model.Entry
	section *model.Section
}

func findList(tree *model.Resume, id string) *listLoc {
	var found *listLoc
	eachList(tree, func(list *model.BulletList, entry *model.Entry, section *model.Section) bool {
		if list.ID == id {
			found = &listLoc{list: list, entry: entry, section: section}
			return true
		}
		return false
	})
	return found
}

// firstList returns the first bullet list in document order; the engine uses
// its open/close/bulletCommand as the template when move must create a list.
func firstList(tree *model.Resume) *model.BulletList {
	var found *model.BulletList
	eachList(tree, func(list *model.BulletList, _ *model.Entry, _ *model.Section) bool {
		found = list
		return true
	})
	return found
}

func findEntry(tree *model.Resume, id string) (*model.Entry, *model.Section) {
	for si := range tree.Sections {
		s := &tree.Sections[si]
		for ii := range s.Items {
			if s.Items[ii].Entry != nil && s.Items[ii].Entry.ID == id {
				return s.Items[ii].Entry, s
			}
		}
	}
	return nil, nil
}

func findSection(tree *model.Resume, id string) *model.Section {
	for si := range tree.Sections {
		if tree.Sections[si].ID == id {
			return &tree.Sections[si]
		}
	}
	return nil
}

func findSkill(tree *model.Resume, id string) *model.SkillLine {
	for si := range tree.Sections {
		for ii := range tree.Sections[si].Items {
			if sk := tree.Sections[si].Items[ii].Skill; sk != nil && sk.ID == id {
				return sk
			}
		}
	}
	return nil
}

func findStyle(tree *model.Resume, id string) *model.StyleSetting {
	for i := range tree.Styles {
		if tree.Styles[i].ID == id {
			return &tree.Styles[i]
		}
	}
	return nil
}

func findAsset(tree *model.Resume, id string) *model.AssetRef {
	for i := range tree.Assets {
		if tree.Assets[i].ID == id {
			return &tree.Assets[i]
		}
	}
	return nil
}

// itemSpan returns the source block span of any section item.
func itemSpan(it model.SectionItem) (model.Span, bool) {
	switch {
	case it.Entry != nil:
		return it.Entry.Span, true
	case it.Bullets != nil:
		return it.Bullets.Span, true
	case it.Skill != nil:
		return it.Skill.Span, true
	case it.Paragraph != nil:
		return it.Paragraph.Span, true
	case it.Opaque != nil:
		return it.Opaque.Span, true
	}
	return model.Span{}, false
}

// itemID returns the ID of any section item.
func itemID(it model.SectionItem) string {
	switch {
	case it.Entry != nil:
		return it.Entry.ID
	case it.Bullets != nil:
		return it.Bullets.ID
	case it.Skill != nil:
		return it.Skill.ID
	case it.Paragraph != nil:
		return it.Paragraph.ID
	case it.Opaque != nil:
		return it.Opaque.ID
	}
	return ""
}

func findParagraph(tree *model.Resume, id string) *model.Paragraph {
	for i := range tree.Sections {
		for j := range tree.Sections[i].Items {
			if p := tree.Sections[i].Items[j].Paragraph; p != nil && p.ID == id {
				return p
			}
		}
	}
	return nil
}

// walkHideable visits every node that can be hidden, with its span start and
// a setter for its hidden flag.
func walkHideable(t *model.Resume, fn func(start int, set func(bool))) {
	var list func(*model.BulletList)
	list = func(l *model.BulletList) {
		fn(l.Span[0], func(h bool) { l.Hidden = h })
		for i := range l.Bullets {
			b := &l.Bullets[i]
			fn(b.Span[0], func(h bool) { b.Hidden = h })
			for j := range b.Children {
				list(&b.Children[j])
			}
		}
	}
	for i := range t.Sections {
		s := &t.Sections[i]
		fn(s.Span[0], func(h bool) { s.Hidden = h })
		for j := range s.Items {
			it := &s.Items[j]
			switch {
			case it.Entry != nil:
				e := it.Entry
				fn(e.Span[0], func(h bool) { e.Hidden = h })
				if e.Bullets != nil {
					list(e.Bullets)
				}
			case it.Bullets != nil:
				list(it.Bullets)
			case it.Skill != nil:
				sk := it.Skill
				fn(sk.Span[0], func(h bool) { sk.Hidden = h })
			case it.Paragraph != nil:
				p := it.Paragraph
				fn(p.Span[0], func(h bool) { p.Hidden = h })
			}
		}
	}
}
