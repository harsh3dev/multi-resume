package parser

import (
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

type call struct {
	name       string
	start, end int
	args       []model.Span
	commented  bool
}

func parseBody(src string, body model.Span, defs []definition, macros macroTexts, resume *model.Resume) error {
	counts := map[string]int{`\section`: 1, `\section*`: 1, `\item`: 0}
	roles := map[string]string{`\section`: "section", `\section*`: "section", `\item`: "bullet"}
	counts[`\textbf`] = 1
	counts[`\begin`] = 1
	counts[`\end`] = 1
	slots := map[string][]model.CommandArg{}
	for _, def := range defs {
		counts[def.Name] = def.ArgCount
		roles[def.Name] = def.Role
		slots[def.Name] = def.Args
	}
	calls := readCalls(src, body, counts)
	roles[`\begin`] = "listOpen"
	roles[`\end`] = "listClose"
	first := body[1]
	for _, c := range calls {
		if roles[c.name] == "section" {
			first = c.start
			break
		}
	}
	if first > body[0] {
		resume.Header = &model.Header{ID: newID("h"), Span: model.Span{body[0], first}, Items: []model.Field{}}
		extractHeader(src, resume.Header, macros)
	}
	var section *model.Section
	var entry *model.Entry
	var list *model.BulletList
	for _, c := range calls {
		if c.start < first {
			continue
		}
		role := roles[c.name]
		switch role {
		case "section":
			if len(c.args) == 0 || c.commented {
				continue
			}
			if section != nil {
				section.Span[1] = c.start
				dropRedundantSkillBullets(section, &resume.Report)
				resume.Sections = append(resume.Sections, *section)
			}
			section = &model.Section{ID: newID("s"), Span: model.Span{c.start, body[1]}, Title: macros.field(src, c.args[0], ""), Items: []model.SectionItem{}}
			entry = nil
			list = nil
		case "entry":
			if section == nil || len(c.args) == 0 {
				continue
			}
			item := model.Entry{Kind: "entry", ID: newID("e"), Span: model.Span{c.start, c.end}, Hidden: c.commented, Command: &c.name, Fields: []model.Field{}}
			for i, arg := range c.args {
				var slot model.Slot
				if i < len(slots[c.name]) {
					slot = slots[c.name][i].Slot
				}
				item.Fields = append(item.Fields, macros.field(src, arg, slot))
			}
			section.Items = append(section.Items, model.SectionItem{Entry: &item})
			entry = section.Items[len(section.Items)-1].Entry
			list = nil
			resume.Report.Entries++
		case "listOpen":
			if section == nil {
				continue
			}
			if strings.Contains(strings.ToLower(c.name), "subheading") {
				continue
			}
			if c.name == `\begin` && len(c.args) == 1 && strings.TrimSpace(src[c.args[0][0]:c.args[0][1]]) != "itemize" {
				continue
			}
			open := c.name
			if c.name == `\begin` {
				if len(c.args) == 0 || src[c.args[0][0]:c.args[0][1]] != "itemize" {
					continue
				}
				open = `\begin{itemize}`
			}
			newList := &model.BulletList{Kind: "bullets", ID: newID("l"), Span: model.Span{c.start, c.end}, Open: open, BulletCommand: `\item`, Bullets: []model.Bullet{}}
			if entry != nil {
				entry.Bullets = newList
				entry.Span[1] = c.end
				list = entry.Bullets
			} else {
				section.Items = append(section.Items, model.SectionItem{Bullets: newList})
				list = section.Items[len(section.Items)-1].Bullets
			}
		case "bullet":
			if section == nil || list == nil || c.commented && len(c.args) == 0 {
				continue
			}
			if len(c.args) == 0 {
				if c.name != `\item` {
					continue
				}
				stop := listTextEnd(src, c.end, calls, roles)
				start := c.end
				for start < stop && (src[start] == ' ' || src[start] == '\t' || src[start] == '\n') {
					start++
				}
				for stop > start && (src[stop-1] == ' ' || src[stop-1] == '\t' || src[stop-1] == '\n') {
					stop--
				}
				if start == stop {
					continue
				}
				c.args = []model.Span{{start, stop}}
				c.end = stop
			}
			bullet := model.Bullet{ID: newID("b"), Span: model.Span{c.start, c.end}, Hidden: c.commented, Text: macros.field(src, c.args[0], "").Text()}
			list.Bullets = append(list.Bullets, bullet)
			list.BulletCommand = c.name
			list.Span[1] = c.end
			if entry != nil {
				entry.Span[1] = c.end
			}
			resume.Report.Bullets++
		case "listClose":
			if c.name == `\end` && (len(c.args) == 0 || src[c.args[0][0]:c.args[0][1]] != "itemize") {
				continue
			}
			if strings.Contains(strings.ToLower(c.name), "subheading") {
				continue
			}
			if list != nil {
				list.Close = c.name
				if c.name == `\end` {
					list.Close = `\end{itemize}`
				}
				list.Span[1] = c.end
				if entry != nil {
					entry.Span[1] = c.end
				}
				list = nil
			}
		case "skill":
			if section == nil || len(c.args) < 2 {
				continue
			}
			section.Items = append(section.Items, model.SectionItem{Skill: &model.SkillLine{Kind: "skill", ID: newID("sk"), Span: model.Span{c.start, c.end}, Hidden: c.commented, Label: macros.field(src, c.args[0], ""), Value: macros.field(src, c.args[1], "")}})
		default:
			if c.name != `\textbf` || section == nil || len(c.args) != 1 || !strings.Contains(strings.ToLower(section.Title.Value), "skill") {
				continue
			}
			if c.start < section.Title.Span[1] {
				continue
			}
			start := c.end
			for start < len(src) && (src[start] == ' ' || src[start] == '\t') {
				start++
			}
			if start >= len(src) || src[start] != '{' {
				continue
			}
			stop, ok := closingCommentGroup(src, start+1, section.Span[1])
			if !ok || stop >= section.Span[1] {
				continue
			}
			value := model.Span{start + 1, stop}
			if value[0] < value[1] && src[value[0]] == ':' {
				value[0]++
			}
			for value[0] < value[1] && src[value[0]] == ' ' {
				value[0]++
			}
			section.Items = append(section.Items, model.SectionItem{Skill: &model.SkillLine{Kind: "skill", ID: newID("sk"), Span: model.Span{c.start, stop + 1}, Label: macros.field(src, c.args[0], ""), Value: macros.field(src, value, "")}})
		}
	}
	if section != nil {
		dropRedundantSkillBullets(section, &resume.Report)
		resume.Sections = append(resume.Sections, *section)
	}
	// Keep unsupported source between recognized nodes explicitly read-only.
	for i := range resume.Sections {
		section := &resume.Sections[i]
		if len(section.Items) == 0 {
			start := section.Title.Span[1] + 1
			if start < section.Span[1] && hasUnrecognizedText(src[start:section.Span[1]]) {
				section.Items = append(section.Items, model.SectionItem{Opaque: &model.Opaque{Kind: "opaque", ID: newID("o"), Span: model.Span{start, section.Span[1]}, Reason: "unsupported LaTeX"}})
				resume.Report.ReadOnlyRegions++
			}
			continue
		}
		end := section.Title.Span[1] + 1
		items := make([]model.SectionItem, 0, len(section.Items)*2)
		for _, item := range section.Items {
			var start, stop int
			switch {
			case item.Entry != nil:
				start, stop = item.Entry.Span[0], item.Entry.Span[1]
			case item.Bullets != nil:
				start, stop = item.Bullets.Span[0], item.Bullets.Span[1]
			case item.Skill != nil:
				start, stop = item.Skill.Span[0], item.Skill.Span[1]
			case item.Paragraph != nil:
				start, stop = item.Paragraph.Span[0], item.Paragraph.Span[1]
			default:
				continue
			}
			if start > end && containsMeaningful(src[end:start]) && hasUnrecognizedText(src[end:start]) {
				items = append(items, model.SectionItem{Opaque: &model.Opaque{Kind: "opaque", ID: newID("o"), Span: model.Span{end, start}, Reason: "unsupported LaTeX"}})
				resume.Report.ReadOnlyRegions++
			}
			items = append(items, item)
			if stop > end {
				end = stop
			}
		}
		section.Items = items
		if end < section.Span[1] && hasUnrecognizedText(src[end:section.Span[1]]) {
			section.Items = append(section.Items, model.SectionItem{Opaque: &model.Opaque{Kind: "opaque", ID: newID("o"), Span: model.Span{end, section.Span[1]}, Reason: "unsupported LaTeX"}})
			resume.Report.ReadOnlyRegions++
		}
	}
	for i := range resume.Sections {
		for j := range resume.Sections[i].Items {
			item := &resume.Sections[i].Items[j]
			if item.Entry != nil && item.Entry.Bullets != nil && item.Entry.Bullets.Close == "" {
				item.Entry.Bullets.Close = `\end{itemize}`
			}
			if item.Bullets != nil && item.Bullets.Close == "" {
				item.Bullets.Close = `\end{itemize}`
			}
		}
	}
	return nil
}

// dropRedundantSkillBullets removes raw bullets that merely wrap parsed
// skill lines — the braced `{\textbf{Label}{: values} \\ ...}` block common
// in skills sections. The skill items already expose the same content as
// editable label/value fields; keeping the bullet would render raw LaTeX in
// the UI. A bullets item is dropped only when EVERY bullet in it is raw and
// contains at least one skill line; mixed lists are left alone.
func dropRedundantSkillBullets(section *model.Section, report *model.Report) {
	var skillSpans []model.Span
	for _, it := range section.Items {
		if it.Skill != nil {
			skillSpans = append(skillSpans, it.Skill.Span)
		}
	}
	if len(skillSpans) == 0 {
		return
	}
	wrapsSkill := func(s model.Span) bool {
		for _, k := range skillSpans {
			if s[0] <= k[0] && k[1] <= s[1] {
				return true
			}
		}
		return false
	}
	kept := section.Items[:0]
	for _, it := range section.Items {
		if it.Bullets != nil && len(it.Bullets.Bullets) > 0 {
			redundant := true
			for _, b := range it.Bullets.Bullets {
				if b.Text.Mode != "raw" || !wrapsSkill(b.Span) {
					redundant = false
					break
				}
			}
			if redundant {
				report.Bullets -= len(it.Bullets.Bullets)
				continue
			}
		}
		kept = append(kept, it)
	}
	section.Items = kept
}

func readCalls(src string, body model.Span, counts map[string]int) []call {
	tokens := latex.Tokenize(src[body[0]:body[1]])
	for i := range tokens {
		tokens[i].Start += body[0]
		tokens[i].End += body[0]
	}
	result := make([]call, 0)
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.Kind == latex.TokComment {
			line := strings.TrimSpace(strings.TrimPrefix(t.Raw, "%"))
			if !strings.HasPrefix(line, `\`) {
				continue
			}
			if line == `\item` || strings.HasPrefix(line, `\item `) {
				// Commented bare \item: text runs to end of the comment line.
				textStart := t.Start + strings.Index(t.Raw, `\item`) + len(`\item`)
				for textStart < t.End && (src[textStart] == ' ' || src[textStart] == '\t') {
					textStart++
				}
				if textStart < t.End {
					result = append(result, call{name: `\item`, start: t.Start, end: t.End, args: []model.Span{{textStart, t.End}}, commented: true})
				}
				continue
			}
			for name, n := range counts {
				if n < 1 || !strings.HasPrefix(line, name) {
					continue
				}
				if strings.HasPrefix(name, `\section`) || name == `\begin` || name == `\end` || name == `\textbf` {
					continue
				}
				if len(line) > len(name) && line[len(name)] != '{' && line[len(name)] != ' ' {
					continue
				}
				if !strings.HasPrefix(line[len(name):], "{") && !strings.HasPrefix(strings.TrimSpace(line[len(name):]), "{") {
					continue
				}
				start := t.Start + strings.Index(t.Raw, name) + len(name)
				args := make([]model.Span, 0, n)
				for j := 0; j < n; j++ {
					open := strings.IndexByte(src[start:t.End], '{')
					if open < 0 {
						break
					}
					start += open + 1
					close, ok := closingCommentGroup(src, start, t.End)
					if !ok {
						break
					}
					args = append(args, model.Span{start, close})
					start = close + 1
				}
				if len(args) != n {
					continue
				}
				result = append(result, call{name: name, start: t.Start, end: t.End, args: args, commented: true})
				break
			}
			continue
		}
		if t.Kind != latex.TokCommand && t.Kind != latex.TokEnv {
			continue
		}
		count, known := counts[t.Raw]
		if !known {
			continue
		}
		c := call{name: t.Raw, start: t.Start, end: t.End, args: []model.Span{}}
		cursor := i + 1
		if t.Raw == `\section` && cursor < len(tokens) && tokens[cursor].Kind == latex.TokText && strings.HasPrefix(tokens[cursor].Raw, "*") {
			// \section*{Title}: the star is a separate text token.
			c.name = `\section*`
			if tokens[cursor].Raw == "*" {
				cursor++
			} else {
				tokens[cursor].Raw = tokens[cursor].Raw[1:]
				tokens[cursor].Start++
			}
		}
		for j := 0; j < count; j++ {
			span, next, ok := group(tokens, cursor, src, latex.TokOpenBrace, latex.TokCloseBrace)
			if !ok {
				break
			}
			c.args = append(c.args, span)
			c.end = tokens[next-1].End
			cursor = next
		}
		if len(c.args) == count {
			result = append(result, c)
			i = cursor - 1
		}
	}
	return result
}

func containsMeaningful(src string) bool { return strings.TrimSpace(src) != "" }

func hasUnrecognizedText(src string) bool {
	// Text inside an option group ([leftmargin=0.15in, label={}]) is
	// structural boilerplate, not resume content.
	depth := 0
	for _, t := range latex.Tokenize(src) {
		switch t.Kind {
		case latex.TokOpenBracket:
			depth++
		case latex.TokCloseBracket:
			if depth > 0 {
				depth--
			}
		case latex.TokText:
			if depth > 0 {
				continue
			}
			if strings.TrimSpace(t.Raw) != "" && strings.IndexFunc(t.Raw, func(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' }) >= 0 && strings.TrimSpace(t.Raw) != "document" && strings.TrimSpace(t.Raw) != "itemize" {
				return true
			}
		}
	}
	return false
}

func listTextEnd(src string, start int, calls []call, roles map[string]string) int {
	end := len(src)
	for _, next := range calls {
		if next.start <= start {
			continue
		}
		if roles[next.name] == "bullet" || roles[next.name] == "listClose" || roles[next.name] == "section" || roles[next.name] == "entry" {
			if next.start < end {
				end = next.start
			}
		}
	}
	return end
}

func closingCommentGroup(src string, start, end int) (int, bool) {
	depth := 1
	for i := start; i < end; i++ {
		if src[i] == '\\' && i+1 < end {
			i++
			continue
		}
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}
