package parser

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/textconv"
)

const parserVersion = "1"

type ParseOptions struct{}

func Parse(src string, _ ParseOptions) (*model.Resume, error) {
	if len(src) > compile.MaxTexBytes {
		return nil, errors.New("source exceeds maximum size")
	}
	if len(src) == 0 {
		return nil, errors.New("empty source")
	}
	if !utf8.ValidString(src) {
		return nil, errors.New("source must be UTF-8")
	}
	// The hash identifies the exact bytes; everything else reads the shadow
	// source, in which hidden blocks are uncommented (same offsets).
	sum := sha256.Sum256([]byte(src))
	src, hs := prepareHide(src)
	pre, body, err := splitPreambleBody(src)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(src[:pre[1]], `\documentclass`) {
		return nil, errors.New("missing document class")
	}
	defs := readDefinitions(latex.Tokenize(src[:pre[1]]), src[:pre[1]])
	learnRoles(defs)
	commands := make([]model.CommandInfo, 0, len(defs))
	for _, def := range defs {
		commands = append(commands, def.CommandInfo)
	}
	class := "article"
	if i := strings.Index(src[:pre[1]], `\documentclass`); i >= 0 {
		if begin := strings.IndexByte(src[i:], '{'); begin >= 0 {
			begin += i + 1
			if end := strings.IndexByte(src[begin:], '}'); end >= 0 {
				class = src[begin : begin+end]
			}
		}
	}
	resume := &model.Resume{
		SchemaVersion: 1, ParserVersion: parserVersion, SourceHash: hex.EncodeToString(sum[:]),
		Meta:     model.Meta{Compiler: "pdflatex", DocumentClass: class, Commands: commands},
		Sections: []model.Section{}, Styles: []model.StyleSetting{}, Assets: []model.AssetRef{},
	}
	for _, line := range strings.Split(src[:pre[1]], "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(line, "% !tex program") {
			if strings.HasSuffix(line, "xelatex") {
				resume.Meta.Compiler = "xelatex"
			}
			if strings.HasSuffix(line, "lualatex") {
				resume.Meta.Compiler = "lualatex"
			}
		}
	}
	if strings.Contains(src[:pre[1]], `\usepackage{fontspec}`) {
		if resume.Meta.Compiler == "pdflatex" {
			resume.Meta.Compiler = "xelatex"
		}
	}
	resume.Styles = findStyles(src, pre)
	resume.Assets = findAssets(src)
	macros := resolveMacros(defs)
	family := detectFamily(class, defs)
	resume.Meta.Family = family
	if err := parseBody(src, body, defs, macros, resume, hs); err != nil {
		return nil, err
	}
	buildReport(src, body, resume, hs)
	return resume, nil
}

// detectFamily recognizes well-known resume templates (data-model §1.2).
func detectFamily(class string, defs []definition) *string {
	if strings.Contains(class, "awesome-cv") {
		family := "awesome-cv"
		return &family
	}
	for _, def := range defs {
		if def.Name == `\resumeSubheading` {
			family := "jakes"
			return &family
		}
	}
	return nil
}

// resolveMacros decodes zero-argument custom commands (often used for i18n)
// so a call like \experienceaitema can be shown as its real text. One level
// of macro-to-macro substitution is applied, matching role learning.
func resolveMacros(defs []definition) macroTexts {
	bodies := make(map[string]string)
	for _, def := range defs {
		if def.ArgCount == 0 {
			bodies[def.Name] = def.Body
		}
	}
	macros := macroTexts{}
	for name, body := range bodies {
		resolved := body
		for inner, text := range bodies {
			if inner != name {
				resolved = strings.ReplaceAll(resolved, inner, text)
			}
		}
		if converted := textconv.ToValue(resolved, model.Span{0, len(resolved)}); converted.Mode == "text" && converted.Value != "" {
			macros[name] = converted.Value
		}
	}
	return macros
}

func newID(prefix string) string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic("cryptographic random source unavailable")
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

// macroTexts maps zero-argument command names to their decoded text.
type macroTexts map[string]string

func (m macroTexts) field(src string, span model.Span, slot model.Slot) model.Field {
	if trimmed := strings.TrimSpace(src[span[0]:span[1]]); strings.HasPrefix(trimmed, `\`) && !strings.ContainsAny(trimmed, " \t\n{}") {
		if value, ok := m[trimmed]; ok {
			return model.Field{ID: newID("f"), Value: value, Mode: "text", Span: span, Slot: slot}
		}
	}
	text := textconv.ToValue(src, span)
	return model.Field{ID: newID("f"), Value: text.Value, Mode: text.Mode, Span: span, Slot: slot, URL: text.URL}
}

// layoutArgCommands take a length or name as their argument, never content.
var layoutArgCommands = map[string]bool{
	`\vspace`: true, `\vspace*`: true, `\hspace`: true, `\hspace*`: true,
	`\setlength`: true, `\addtolength`: true, `\vskip`: true, `\hskip`: true,
}

// buildReport counts only source-backed spans. Unknown visible content stays
// read-only, rather than being silently counted as editable coverage.
func buildReport(src string, body model.Span, resume *model.Resume, hs *hideState) {
	if hs != nil {
		src = hs.Mask // hidden text is not visible content
	}
	resume.Report.Sections = len(resume.Sections)
	spans := make([]model.Span, 0)
	addField := func(f model.Field) {
		if f.Mode == "text" && f.Span[1] > f.Span[0] {
			spans = append(spans, f.Span)
		}
	}
	var addList func(*model.BulletList)
	addList = func(list *model.BulletList) {
		for i := range list.Bullets {
			b := &list.Bullets[i]
			if !b.Hidden && b.Text.Mode == "text" {
				spans = append(spans, b.Text.Span)
			}
			for j := range b.Children {
				addList(&b.Children[j])
			}
		}
	}
	if resume.Header != nil {
		if resume.Header.Name != nil {
			addField(*resume.Header.Name)
		}
		for _, f := range resume.Header.Items {
			addField(f)
		}
	}
	for _, section := range resume.Sections {
		addField(section.Title)
		for _, item := range section.Items {
			if item.Entry != nil {
				for _, f := range item.Entry.Fields {
					addField(f)
				}
				if item.Entry.Bullets != nil {
					addList(item.Entry.Bullets)
				}
			}
			if item.Bullets != nil {
				addList(item.Bullets)
			}
			if item.Skill != nil {
				addField(item.Skill.Label)
				addField(item.Skill.Value)
			}
			if item.Paragraph != nil {
				addField(item.Paragraph.Text)
			}
		}
	}
	// Visible characters are the letters and digits of plain text in the body:
	// comments, command names and environment names do not count. A character
	// is covered when an editable span contains its byte position, so a
	// span that merely wraps markup gains nothing.
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
	inSpan := func(pos int) bool {
		i := sort.Search(len(spans), func(i int) bool { return spans[i][0] > pos })
		for j := i - 1; j >= 0 && j >= i-8; j-- {
			if pos >= spans[j][0] && pos < spans[j][1] {
				return true
			}
		}
		return false
	}
	tokens := latex.Tokenize(src[body[0]:body[1]])
	visible, covered := 0, 0
	depth := 0 // inside [..] option groups
	for i, token := range tokens {
		switch token.Kind {
		case latex.TokOpenBracket:
			depth++
		case latex.TokCloseBracket:
			if depth > 0 {
				depth--
			}
		}
		if token.Kind != latex.TokText || depth > 0 {
			continue
		}
		if i >= 2 && tokens[i-1].Kind == latex.TokOpenBrace {
			prev := tokens[i-2]
			if prev.Kind == latex.TokEnv || layoutArgCommands[prev.Raw] {
				continue // \begin{name}, \end{name}, \vspace{1pt}: layout, not content
			}
		}
		for off, r := range token.Raw {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				visible++
				if inSpan(body[0] + token.Start + off) {
					covered++
				}
			}
		}
	}
	if visible > 0 {
		resume.Report.Coverage = math.Min(1, float64(covered)/float64(visible))
	}
	if covered < visible && resume.Report.ReadOnlyRegions == 0 {
		resume.Report.ReadOnlyRegions = 1
	}
}
