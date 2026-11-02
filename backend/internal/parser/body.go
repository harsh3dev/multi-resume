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

func parseBody(src string, body model.Span, defs []definition, macros macroTexts, resume *model.Resume, hs *hideState) error {
	counts := map[string]int{`\section`: 1, `\section*`: 1, `\item`: 0}
	roles := map[string]string{`\section`: "section", `\section*`: "section", `\item`: "bullet"}
	counts[`\textbf`] = 1
	counts[`\begin`] = 1
	counts[`\end`] = 1
	slots := map[string][]model.CommandArg{}
	for name, t := range dictionary.Commands {
		counts[name] = len(t.Slots)
		roles[name] = t.Role
		slots[name] = t.args()
	}
	for _, def := range defs {
		counts[def.Name] = def.ArgCount
		roles[def.Name] = def.Role
		slots[def.Name] = def.Args
	}
	noSkip := map[string]bool{}
	for name, role := range roles {
		noSkip[name] = role == "entry"
	}
	calls := readCalls(src, body, counts, noSkip)
	for i := range calls {
		if hs.inBlock(calls[i].start) {
			calls[i].commented = true
			calls[i].start = hs.lift(src, calls[i].start)
		}
	}
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
		extractHeaderWith(src, resume.Header, macros, latexOptions(defs))
	}
	var section *model.Section
	var entry *model.Entry
	// stack holds the open lists, outermost first; list is the innermost.
	// Lists opened inside a bullet become that bullet's Children.
	var stack []*model.BulletList
	var list *model.BulletList
	structDepth, skipDepth := 0, 0
	unwind := func() {
		for len(stack) > 1 {
			closeList(stack, resume)
			stack = stack[:len(stack)-1]
		}
		stack, list = nil, nil
		skipDepth = 0
	}
	listEnvName := func(c call) (string, bool) {
		if c.name != `\begin` && c.name != `\end` {
			return "", false
		}
		if len(c.args) == 0 {
			return "", false
		}
		name := strings.TrimSpace(src[c.args[0][0]:c.args[0][1]])
		return name, isListEnv(name)
	}
	for _, c := range calls {
		if c.start < first {
			continue
		}
		role := roles[c.name]
		switch role {
		case "section":
			if len(c.args) == 0 || c.commented && !hs.inBlock(c.start) {
				continue
			}
			unwind()
			if section != nil {
				section.Span[1] = c.start
				dropRedundantSkillBullets(section, &resume.Report)
				resume.Sections = append(resume.Sections, *section)
			}
			section = &model.Section{ID: hs.takeID("s", c.start), Hidden: hs.inBlock(c.start), Span: model.Span{c.start, body[1]}, Title: macros.field(src, c.args[0], ""), Items: []model.SectionItem{}}
			entry = nil
		case "entry":
			if section == nil || len(c.args) == 0 {
				continue
			}
			unwind()
			item := model.Entry{Kind: "entry", ID: hs.takeID("e", c.start), Span: model.Span{c.start, c.end}, Hidden: c.commented, Command: &c.name, Fields: []model.Field{}}
			for i, arg := range c.args {
				var slot model.Slot
				if i < len(slots[c.name]) {
					slot = slots[c.name][i].Slot
				}
				item.Fields = append(item.Fields, macros.field(src, arg, slot))
			}
			section.Items = append(section.Items, model.SectionItem{Entry: &item})
			entry = section.Items[len(section.Items)-1].Entry
			resume.Report.Entries++
		case "paragraph":
			if section == nil || len(c.args) == 0 || c.commented && !hs.inBlock(c.start) {
				continue
			}
			unwind()
			section.Items = append(section.Items, model.SectionItem{Paragraph: &model.Paragraph{Kind: "paragraph", ID: hs.takeID("p", c.start), Hidden: hs.inBlock(c.start), Span: model.Span{c.start, c.end}, Text: macros.field(src, c.args[0], "")}})
		case "structure":
			structDepth++
		case "listOpen":
			if section == nil {
				continue
			}
			open := c.name
			if c.name == `\begin` {
				name, ok := listEnvName(c)
				if !ok {
					continue
				}
				open = `\begin{` + name + `}`
			}
			newList := &model.BulletList{Kind: "bullets", ID: hs.takeID("l", c.start), Hidden: hs.inBlock(c.start), Span: model.Span{c.start, c.end}, Open: open, BulletCommand: `\item`, Bullets: []model.Bullet{}}
			if list != nil {
				// Nested list: it must hang off a bullet of the enclosing list.
				if len(list.Bullets) == 0 || skipDepth > 0 {
					skipDepth++
					continue
				}
				stack = append(stack, newList)
				list = newList
				continue
			}
			if entry != nil {
				entry.Bullets = newList
				entry.Span[1] = c.end
				list = entry.Bullets
			} else {
				section.Items = append(section.Items, model.SectionItem{Bullets: newList})
				list = section.Items[len(section.Items)-1].Bullets
			}
			stack = []*model.BulletList{list}
		case "bullet":
			if section == nil || list == nil || skipDepth > 0 || c.commented && !hs.inBlock(c.start) && len(c.args) == 0 {
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
			bullet := model.Bullet{ID: hs.takeID("b", c.start), Span: model.Span{c.start, c.end}, Hidden: c.commented, Text: macros.field(src, c.args[0], "").Text()}
			list.Bullets = append(list.Bullets, bullet)
			list.BulletCommand = c.name
			list.Span[1] = c.end
			if entry != nil {
				entry.Span[1] = c.end
			}
			resume.Report.Bullets++
		case "listClose":
			if c.name == `\end` {
				if _, ok := listEnvName(c); !ok {
					continue
				}
			}
			if list == nil {
				if structDepth > 0 {
					structDepth--
				}
				continue
			}
			if skipDepth > 0 {
				skipDepth--
				continue
			}
			list.Close = c.name
			if c.name == `\end` {
				name, _ := listEnvName(c)
				list.Close = `\end{` + name + `}`
			}
			list.Span[1] = c.end
			if len(stack) > 1 {
				closeList(stack, resume)
				stack = stack[:len(stack)-1]
				list = stack[len(stack)-1]
				continue
			}
			if entry != nil {
				entry.Span[1] = c.end
			}
			stack, list = nil, nil
		case "skill":
			if section == nil || len(c.args) < 2 {
				continue
			}
			section.Items = append(section.Items, model.SectionItem{Skill: &model.SkillLine{Kind: "skill", ID: hs.takeID("sk", c.start), Span: model.Span{c.start, c.end}, Hidden: c.commented, Label: macros.field(src, c.args[0], ""), Value: macros.field(src, c.args[1], "")}})
		default:
			if c.name != `\textbf` || section == nil || len(c.args) != 1 || !skillishTitle(section.Title.Value) {
				continue
			}
			if c.start < section.Title.Span[1] {
				continue
			}
			start := c.end
			for start < len(src) && (src[start] == ' ' || src[start] == '\t') {
				start++
			}
			// Labels often end a line and put the value on the next one.
			// Skip only whitespace; inlineValueEnd still stops at the first
			// actual line break following the value.
			if strings.HasSuffix(strings.TrimSpace(src[c.args[0][0]:c.args[0][1]]), ":") && start < len(src) && src[start] == '\n' {
				start++
				for start < len(src) && (src[start] == ' ' || src[start] == '\t') {
					start++
				}
			}
			if start < len(src) && src[start] != '{' {
				// `\textbf{Languages:} Java, Go \\` — the value is the rest of the
				// line, up to a line break command or an unmatched brace.
				if src[start] == '&' {
					continue // a tabular row: handled by tabularSkills
				}
				end := inlineValueEnd(src, start, section.Span[1])
				value := model.Span{start, end}
				if value[0] < value[1] && src[value[0]] == ':' {
					value[0]++
				}
				for value[0] < value[1] && (src[value[0]] == ' ' || src[value[0]] == '\t') {
					value[0]++
				}
				for value[1] > value[0] && (src[value[1]-1] == ' ' || src[value[1]-1] == '\t' || src[value[1]-1] == '\r') {
					value[1]--
				}
				if value[0] < value[1] {
					section.Items = append(section.Items, model.SectionItem{Skill: &model.SkillLine{Kind: "skill", ID: hs.takeID("sk", c.start), Hidden: hs.inBlock(c.start), Span: model.Span{c.start, value[1]}, Label: macros.field(src, c.args[0], ""), Value: macros.field(src, value, "")}})
				}
				continue
			}
			if start >= len(src) {
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
			section.Items = append(section.Items, model.SectionItem{Skill: &model.SkillLine{Kind: "skill", ID: hs.takeID("sk", c.start), Hidden: hs.inBlock(c.start), Span: model.Span{c.start, stop + 1}, Label: macros.field(src, c.args[0], ""), Value: macros.field(src, value, "")}})
		}
	}
	unwind()
	if section != nil {
		dropRedundantSkillBullets(section, &resume.Report)
		resume.Sections = append(resume.Sections, *section)
	}
	for i := range resume.Sections {
		tabularSkills(src, &resume.Sections[i], macros, latexOptions(defs))
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

// readCalls lists the known commands in body. Calls inside the arguments of
// a command are skipped, except for noSkip commands (entries), whose raw
// arguments may hold lists that should still be found.
func readCalls(src string, body model.Span, counts map[string]int, noSkip map[string]bool) []call {
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
			if !noSkip[c.name] {
				i = cursor - 1
			}
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
		if roles[next.name] == "bullet" || roles[next.name] == "listOpen" || roles[next.name] == "listClose" || roles[next.name] == "section" || roles[next.name] == "entry" {
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

// latexOptions tells the tree parser how many arguments each of the
// document's own commands takes, so multi-line calls are read as one node.
func latexOptions(defs []definition) latex.Options {
	opt := latex.Options{Commands: map[string]latex.Arity{}}
	for name, t := range dictionary.Commands {
		opt.Commands[name] = latex.Arity{Req: len(t.Slots)}
	}
	for _, def := range defs {
		a := latex.Arity{Req: def.ArgCount}
		if def.OptionalDefault != nil && def.ArgCount > 0 {
			a = latex.Arity{Opt: 1, Req: def.ArgCount - 1}
		}
		opt.Commands[def.Name] = a
	}
	return opt
}

// closeList attaches the innermost (just closed) list to the last bullet of
// its parent and widens the parent's spans to cover it.
func closeList(stack []*model.BulletList, resume *model.Resume) {
	child, parent := stack[len(stack)-1], stack[len(stack)-2]
	last := &parent.Bullets[len(parent.Bullets)-1]
	last.Children = append(last.Children, *child)
	if child.Span[1] > last.Span[1] {
		last.Span[1] = child.Span[1]
	}
	if last.Span[1] > parent.Span[1] {
		parent.Span[1] = last.Span[1]
	}
}

var skillishWords = []string{"skill", "language", "technolog", "tools", "stack", "competenc", "proficienc", "expertise"}

// skillishTitle reports whether a section title names a skills section.
func skillishTitle(title string) bool {
	title = strings.ToLower(title)
	for _, w := range skillishWords {
		if strings.Contains(title, w) {
			return true
		}
	}
	return false
}

// inlineValueEnd finds where a skill value written on the same line as its
// label stops: at a `\\` line break, an unmatched closing brace, or the end
// of the line.
func inlineValueEnd(src string, start, limit int) int {
	depth := 0
	for i := start; i < limit; i++ {
		switch src[i] {
		case '\\':
			if i+1 < limit && src[i+1] == '\\' && depth == 0 {
				return i
			}
			i++
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		case '\n':
			if depth == 0 {
				return i
			}
		case '%':
			if depth == 0 {
				return i
			}
		}
	}
	return limit
}
