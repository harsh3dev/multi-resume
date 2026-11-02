package model

import (
	"bytes"
	"encoding/json"
	"errors"
)

// SectionItem is a strict discriminated union; unknown kinds are rejected.
type SectionItem struct {
	Entry     *Entry      `json:"-"`
	Bullets   *BulletList `json:"-"`
	Skill     *SkillLine  `json:"-"`
	Paragraph *Paragraph  `json:"-"`
	Opaque    *Opaque     `json:"-"`
}

func (i SectionItem) MarshalJSON() ([]byte, error) {
	switch {
	case i.Entry != nil:
		return json.Marshal(i.Entry)
	case i.Bullets != nil:
		return json.Marshal(i.Bullets)
	case i.Skill != nil:
		return json.Marshal(i.Skill)
	case i.Paragraph != nil:
		return json.Marshal(i.Paragraph)
	case i.Opaque != nil:
		return json.Marshal(i.Opaque)
	default:
		return nil, errors.New("empty section item")
	}
}
func (i *SectionItem) UnmarshalJSON(data []byte) error {
	var tag struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &tag); err != nil {
		return err
	}
	*i = SectionItem{}
	var target any
	switch tag.Kind {
	case "entry":
		i.Entry = &Entry{}
		target = i.Entry
	case "bullets":
		i.Bullets = &BulletList{}
		target = i.Bullets
	case "skill":
		i.Skill = &SkillLine{}
		target = i.Skill
	case "paragraph":
		i.Paragraph = &Paragraph{}
		target = i.Paragraph
	case "opaque":
		i.Opaque = &Opaque{}
		target = i.Opaque
	default:
		return errors.New("unknown section item kind")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(target)
}

type Entry struct {
	Kind    string      `json:"kind" tstype:"'entry'"`
	ID      string      `json:"id"`
	Span    Span        `json:"span"`
	Hidden  bool        `json:"hidden"`
	Command *string     `json:"command" tstype:"string | null,required"`
	Fields  []Field     `json:"fields"`
	Bullets *BulletList `json:"bullets" tstype:"BulletList | null,required"`
}
type BulletList struct {
	Kind          string   `json:"kind" tstype:"'bullets'"`
	ID            string   `json:"id"`
	Span          Span     `json:"span"`
	Hidden        bool     `json:"hidden"`
	Open          string   `json:"open"`
	Close         string   `json:"close"`
	BulletCommand string   `json:"bulletCommand"`
	Bullets       []Bullet `json:"bullets"`
}
type Bullet struct {
	ID     string `json:"id"`
	Span   Span   `json:"span"`
	Hidden bool   `json:"hidden"`
	Text   Text   `json:"text"`
	// Children are lists nested inside this bullet. Span covers the bullet and
	// its children; Text covers only the bullet's own words.
	Children []BulletList `json:"children,omitempty"`
}
type SkillLine struct {
	Kind   string `json:"kind" tstype:"'skill'"`
	ID     string `json:"id"`
	Span   Span   `json:"span"`
	Hidden bool   `json:"hidden"`
	Label  Field  `json:"label"`
	Value  Field  `json:"value"`
}
type Paragraph struct {
	Kind   string `json:"kind" tstype:"'paragraph'"`
	ID     string `json:"id"`
	Span   Span   `json:"span"`
	Hidden bool   `json:"hidden"`
	Text   Field  `json:"text"`
}
type Opaque struct {
	Kind   string `json:"kind" tstype:"'opaque'"`
	ID     string `json:"id"`
	Span   Span   `json:"span"`
	Reason string `json:"reason"`
}
