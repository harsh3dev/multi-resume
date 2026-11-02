package model

import (
	"encoding/json"
	"errors"
)

// Span uses byte offsets into the exact source identified by Resume.SourceHash.
type Span [2]int

func (s Span) MarshalJSON() ([]byte, error) { return json.Marshal([2]int(s)) }
func (s *Span) UnmarshalJSON(data []byte) error {
	var pair [2]int
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	if pair[0] < 0 || pair[1] < pair[0] {
		return errors.New("invalid span")
	}
	*s = Span(pair)
	return nil
}

type Text struct {
	Value string `json:"value"`
	Mode  string `json:"mode" tstype:"'text' | 'raw'"`
	Span  Span   `json:"span"`
	URL   *URL   `json:"url,omitempty"`
}
type URL struct {
	Value string `json:"value"`
	Span  Span   `json:"span"`
}
type Slot string

const (
	Bold   Slot = "bold"
	Italic Slot = "italic"
	Right  Slot = "right"
	Left   Slot = "left"
	Plain  Slot = "plain"
)

type Field struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Mode  string `json:"mode" tstype:"'text' | 'raw'"`
	Span  Span   `json:"span"`
	URL   *URL   `json:"url,omitempty"`
	Slot  Slot   `json:"slot,omitempty"`
}

func (f Field) Text() Text { return Text{Value: f.Value, Mode: f.Mode, Span: f.Span, URL: f.URL} }

type Resume struct {
	SchemaVersion int            `json:"schemaVersion"`
	ParserVersion string         `json:"parserVersion"`
	SourceHash    string         `json:"sourceHash"`
	Meta          Meta           `json:"meta"`
	Header        *Header        `json:"header" tstype:"Header | null,required"`
	Sections      []Section      `json:"sections"`
	Styles        []StyleSetting `json:"styles"`
	Assets        []AssetRef     `json:"assets"`
	Report        Report         `json:"report"`
}
type Meta struct {
	Compiler      string        `json:"compiler" tstype:"'pdflatex' | 'xelatex' | 'lualatex'"`
	DocumentClass string        `json:"documentClass"`
	Family        *string       `json:"family" tstype:"string | null,required"`
	Commands      []CommandInfo `json:"commands"`
}
type CommandInfo struct {
	Name            string       `json:"name"`
	ArgCount        int          `json:"argCount"`
	OptionalDefault *string      `json:"optionalDefault" tstype:"string | null,required"`
	Role            string       `json:"role" tstype:"'entry' | 'bullet' | 'listOpen' | 'listClose' | 'section' | 'skill' | 'link' | 'paragraph' | 'structure' | 'contact' | 'other'"`
	Args            []CommandArg `json:"args"`
}
type CommandArg struct {
	Index int    `json:"index"`
	Use   string `json:"use" tstype:"'text' | 'url' | 'length' | 'unused' | 'unknown'"`
	Slot  Slot   `json:"slot,omitempty"`
}
type Header struct {
	ID    string  `json:"id"`
	Span  Span    `json:"span"`
	Name  *Field  `json:"name" tstype:"Field | null,required"`
	Items []Field `json:"items"`
}
type Section struct {
	ID     string        `json:"id"`
	Span   Span          `json:"span"`
	Title  Field         `json:"title"`
	Hidden bool          `json:"hidden"`
	Items  []SectionItem `json:"items" tstype:"SectionNode[]"`
}
type StyleSetting struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Span  *Span  `json:"span" tstype:"Span | null,required"`
}
type AssetRef struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Span     Span   `json:"span"`
	Uploaded bool   `json:"uploaded"`
}
type Report struct {
	Sections        int     `json:"sections"`
	Entries         int     `json:"entries"`
	Bullets         int     `json:"bullets"`
	ReadOnlyRegions int     `json:"readOnlyRegions"`
	Coverage        float64 `json:"coverage"`
}
