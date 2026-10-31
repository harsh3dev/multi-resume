package engine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func newID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic("cryptographic random source unavailable")
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

// deepCopy clones a tree through its JSON form so mirrors never mutate the
// caller's draft.
func deepCopy(tree *model.Resume) (*model.Resume, error) {
	data, err := json.Marshal(tree)
	if err != nil {
		return nil, err
	}
	var out model.Resume
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// carryIDs matches re-parsed nodes to the expected tree by structural
// position (section index, item index, field index) and copies the expected
// IDs in, so IDs survive edits (data-model §4 step 5). Nodes without a
// positional counterpart keep their fresh IDs; treesEqual decides whether
// the structures really match.
func carryIDs(expected, reparsed *model.Resume) {
	if expected.Header != nil && reparsed.Header != nil {
		reparsed.Header.ID = expected.Header.ID
		carryFieldID(expected.Header.Name, reparsed.Header.Name)
		for i := 0; i < len(expected.Header.Items) && i < len(reparsed.Header.Items); i++ {
			reparsed.Header.Items[i].ID = expected.Header.Items[i].ID
		}
	}
	for i := 0; i < len(expected.Sections) && i < len(reparsed.Sections); i++ {
		es, rs := &expected.Sections[i], &reparsed.Sections[i]
		rs.ID = es.ID
		rs.Title.ID = es.Title.ID
		for j := 0; j < len(es.Items) && j < len(rs.Items); j++ {
			carryItemIDs(&es.Items[j], &rs.Items[j])
		}
	}
	for i := 0; i < len(expected.Styles) && i < len(reparsed.Styles); i++ {
		if expected.Styles[i].Key == reparsed.Styles[i].Key {
			reparsed.Styles[i].ID = expected.Styles[i].ID
		}
	}
	for i := 0; i < len(expected.Assets) && i < len(reparsed.Assets); i++ {
		if expected.Assets[i].Path == reparsed.Assets[i].Path {
			reparsed.Assets[i].ID = expected.Assets[i].ID
		}
	}
}

func carryFieldID(expected, reparsed *model.Field) {
	if expected != nil && reparsed != nil {
		reparsed.ID = expected.ID
	}
}

func carryItemIDs(expected, reparsed *model.SectionItem) {
	switch {
	case expected.Entry != nil && reparsed.Entry != nil:
		ee, re := expected.Entry, reparsed.Entry
		re.ID = ee.ID
		for i := 0; i < len(ee.Fields) && i < len(re.Fields); i++ {
			re.Fields[i].ID = ee.Fields[i].ID
		}
		carryListIDs(ee.Bullets, re.Bullets)
	case expected.Bullets != nil && reparsed.Bullets != nil:
		carryListIDs(expected.Bullets, reparsed.Bullets)
	case expected.Skill != nil && reparsed.Skill != nil:
		reparsed.Skill.ID = expected.Skill.ID
		reparsed.Skill.Label.ID = expected.Skill.Label.ID
		reparsed.Skill.Value.ID = expected.Skill.Value.ID
	case expected.Paragraph != nil && reparsed.Paragraph != nil:
		reparsed.Paragraph.ID = expected.Paragraph.ID
		reparsed.Paragraph.Text.ID = expected.Paragraph.Text.ID
	case expected.Opaque != nil && reparsed.Opaque != nil:
		reparsed.Opaque.ID = expected.Opaque.ID
	}
}

func carryListIDs(expected, reparsed *model.BulletList) {
	if expected == nil || reparsed == nil {
		return
	}
	reparsed.ID = expected.ID
	for i := 0; i < len(expected.Bullets) && i < len(reparsed.Bullets); i++ {
		reparsed.Bullets[i].ID = expected.Bullets[i].ID
	}
}

// treesEqual compares structure, values, and hidden flags. Spans, IDs,
// source hash, parser version, and the derived report are ignored; asset
// upload state lives outside the source and is ignored too.
func treesEqual(a, b *model.Resume) bool {
	if !metaEqual(a.Meta, b.Meta) {
		return false
	}
	if !headerEqual(a.Header, b.Header) {
		return false
	}
	if len(a.Sections) != len(b.Sections) {
		return false
	}
	for i := range a.Sections {
		if !sectionEqual(&a.Sections[i], &b.Sections[i]) {
			return false
		}
	}
	if len(a.Styles) != len(b.Styles) {
		return false
	}
	for i := range a.Styles {
		if a.Styles[i].Key != b.Styles[i].Key || a.Styles[i].Value != b.Styles[i].Value {
			return false
		}
	}
	if len(a.Assets) != len(b.Assets) {
		return false
	}
	for i := range a.Assets {
		if a.Assets[i].Path != b.Assets[i].Path {
			return false
		}
	}
	return true
}

func metaEqual(a, b model.Meta) bool {
	if a.Compiler != b.Compiler || a.DocumentClass != b.DocumentClass {
		return false
	}
	if (a.Family == nil) != (b.Family == nil) {
		return false
	}
	if a.Family != nil && *a.Family != *b.Family {
		return false
	}
	if len(a.Commands) != len(b.Commands) {
		return false
	}
	for i := range a.Commands {
		ac, bc := a.Commands[i], b.Commands[i]
		if ac.Name != bc.Name || ac.ArgCount != bc.ArgCount || ac.Role != bc.Role {
			return false
		}
		if (ac.OptionalDefault == nil) != (bc.OptionalDefault == nil) {
			return false
		}
		if ac.OptionalDefault != nil && *ac.OptionalDefault != *bc.OptionalDefault {
			return false
		}
		if len(ac.Args) != len(bc.Args) {
			return false
		}
		for j := range ac.Args {
			if ac.Args[j] != bc.Args[j] {
				return false
			}
		}
	}
	return true
}

func headerEqual(a, b *model.Header) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if !fieldPtrEqual(a.Name, b.Name) || len(a.Items) != len(b.Items) {
		return false
	}
	for i := range a.Items {
		if !fieldEqual(a.Items[i], b.Items[i]) {
			return false
		}
	}
	return true
}

func sectionEqual(a, b *model.Section) bool {
	if a.Hidden != b.Hidden || !fieldEqual(a.Title, b.Title) || len(a.Items) != len(b.Items) {
		return false
	}
	for i := range a.Items {
		if !itemEqual(&a.Items[i], &b.Items[i]) {
			return false
		}
	}
	return true
}

func itemEqual(a, b *model.SectionItem) bool {
	if kindOf(*a) != kindOf(*b) {
		return false
	}
	switch {
	case a.Entry != nil:
		ae, be := a.Entry, b.Entry
		if ae.Hidden != be.Hidden || (ae.Command == nil) != (be.Command == nil) {
			return false
		}
		if ae.Command != nil && *ae.Command != *be.Command {
			return false
		}
		if len(ae.Fields) != len(be.Fields) {
			return false
		}
		for i := range ae.Fields {
			if !fieldEqual(ae.Fields[i], be.Fields[i]) {
				return false
			}
		}
		return listEqual(ae.Bullets, be.Bullets)
	case a.Bullets != nil:
		return listEqual(a.Bullets, b.Bullets)
	case a.Skill != nil:
		return a.Skill.Hidden == b.Skill.Hidden &&
			fieldEqual(a.Skill.Label, b.Skill.Label) && fieldEqual(a.Skill.Value, b.Skill.Value)
	case a.Paragraph != nil:
		return fieldEqual(a.Paragraph.Text, b.Paragraph.Text)
	case a.Opaque != nil:
		return a.Opaque.Reason == b.Opaque.Reason
	}
	return false
}

func kindOf(it model.SectionItem) string {
	switch {
	case it.Entry != nil:
		return "entry"
	case it.Bullets != nil:
		return "bullets"
	case it.Skill != nil:
		return "skill"
	case it.Paragraph != nil:
		return "paragraph"
	case it.Opaque != nil:
		return "opaque"
	}
	return ""
}

func listEqual(a, b *model.BulletList) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if a.Open != b.Open || a.Close != b.Close || a.BulletCommand != b.BulletCommand ||
		len(a.Bullets) != len(b.Bullets) {
		return false
	}
	for i := range a.Bullets {
		if a.Bullets[i].Hidden != b.Bullets[i].Hidden || !textEqual(a.Bullets[i].Text, b.Bullets[i].Text) {
			return false
		}
	}
	return true
}

func fieldEqual(a, b model.Field) bool {
	return a.Slot == b.Slot && textEqual(a.Text(), b.Text())
}

func fieldPtrEqual(a, b *model.Field) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || fieldEqual(*a, *b)
}

func textEqual(a, b model.Text) bool {
	if a.Value != b.Value || a.Mode != b.Mode {
		return false
	}
	if (a.URL == nil) != (b.URL == nil) {
		return false
	}
	return a.URL == nil || a.URL.Value == b.URL.Value
}
