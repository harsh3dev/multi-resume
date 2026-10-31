package engine

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/harsh3dev/multi-resume/backend/internal/fonts"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/textconv"
)

// maxValueLen caps any user-supplied value, matching textconv's limit.
const maxValueLen = 10000

// RootParentID is the reorder parentId for sections within the document.
const RootParentID = "root"

// planned couples the source patches of an op with the pure-tree mirror that
// produces the expected tree. Both come from one planner so they cannot
// diverge. The mirror re-locates nodes by ID because it runs on a deep copy.
type planned struct {
	patches []Patch
	mirror  func(tree *model.Resume) error
}

func planOp(src string, tree *model.Resume, op model.Op) (planned, error) {
	switch op.Kind {
	case "edit":
		return planEdit(src, tree, op)
	case "editUrl":
		return planEditURL(src, tree, op)
	case "add":
		return planAdd(src, tree, op)
	case "delete":
		return planDelete(src, tree, op)
	case "reorder":
		return planReorder(src, tree, op)
	case "move":
		return planMove(src, tree, op)
	case "hide":
		return planHide(src, tree, op, true)
	case "show":
		return planHide(src, tree, op, false)
	case "replaceImage":
		return planReplaceImage(tree, op)
	case "setStyle":
		return planSetStyle(src, tree, op)
	}
	return planned{}, fmt.Errorf("unknown op %q", op.Kind)
}

// patchesForOp turns one op into source patches (data-model §4 step 2).
func patchesForOp(src string, tree *model.Resume, op model.Op) ([]Patch, error) {
	p, err := planOp(src, tree, op)
	return p.patches, err
}

// applyOpToTree is the pure-JSON mirror of an op; it produces the expected
// tree. src is the pre-op source, used to recompute exact expected text
// values the same way the parser will decode the patched source.
func applyOpToTree(tree *model.Resume, src string, op model.Op) error {
	p, err := planOp(src, tree, op)
	if err != nil {
		return err
	}
	return p.mirror(tree)
}

// --- source line helpers (byte offsets throughout) ---

func lineStartOf(src string, pos int) int {
	for pos > 0 && src[pos-1] != '\n' {
		pos--
	}
	return pos
}

func lineEndOf(src string, pos int) int {
	for pos < len(src) && src[pos] != '\n' {
		pos++
	}
	return pos
}

func whitespaceOnly(s string) bool {
	return strings.Trim(s, " \t\r\n") == ""
}

func indentOfLine(src string, pos int) string {
	start := lineStartOf(src, pos)
	end := start
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	return src[start:end]
}

// lineBlock expands a span to whole lines: [line start of span start, line
// end of span end). The trailing newline is excluded.
func lineBlock(src string, span model.Span) (int, int) {
	return lineStartOf(src, span[0]), lineEndOf(src, span[1])
}

// blockRange expands a span to whole lines. end excludes the trailing
// newline (commenting range); delEnd includes it (removal range). A span
// ending exactly at a line start does not claim that line. It is an error
// when the node shares a line with other content.
func blockRange(src string, span model.Span) (start, end, delEnd int, err error) {
	start = lineStartOf(src, span[0])
	if !whitespaceOnly(src[start:span[0]]) {
		return 0, 0, 0, errors.New("node shares its line with other content")
	}
	if span[1] > span[0] && lineStartOf(src, span[1]) == span[1] {
		end = span[1]
		if end > start && src[end-1] == '\n' {
			end--
		}
		delEnd = span[1]
		return start, end, delEnd, nil
	}
	end = lineEndOf(src, span[1])
	if !whitespaceOnly(src[span[1]:end]) {
		return 0, 0, 0, errors.New("node shares its line with other content")
	}
	delEnd = end
	if delEnd < len(src) && src[delEnd] == '\n' {
		delEnd++
	}
	return start, end, delEnd, nil
}

// blockRangeRemoval is blockRange for cut/delete callers: [start, delEnd).
func blockRangeRemoval(src string, span model.Span) (int, int, error) {
	start, _, delEnd, err := blockRange(src, span)
	return start, delEnd, err
}

// commentLines prefixes every line with "% ".
func commentLines(block string) string {
	lines := strings.Split(block, "\n")
	for i := range lines {
		lines[i] = "% " + lines[i]
	}
	return strings.Join(lines, "\n")
}

// uncommentLines strips one comment prefix ("% " or "%") per line, keeping
// indentation. Lines that are not comments are left unchanged; the
// double-apply check rejects the op if the result does not re-parse.
func uncommentLines(block string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		j := 0
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		if j < len(line) && line[j] == '%' {
			cut := j + 1
			if cut < len(line) && line[cut] == ' ' {
				cut++
			}
			lines[i] = line[:j] + line[cut:]
		}
	}
	return strings.Join(lines, "\n")
}

func docEndIndex(src string) int {
	if i := strings.LastIndex(src, `\end{document}`); i >= 0 {
		return i
	}
	return len(src)
}

// decodedValue computes the text the parser will see after splicing piece
// into the source, mirroring textconv.ToValue on the new span.
func decodedValue(piece string) model.Text {
	return textconv.ToValue(piece, model.Span{0, len(piece)})
}

// spliceAncestorMirror returns a mirror step that updates every text node
// whose span strictly contains the edited span: their decoded values change
// with the splice even though the op did not target them (the parser nests
// skill lines inside a raw bullet's span, for example).
func spliceAncestorMirror(src string, span model.Span, replacement, skipID string) func(*model.Resume) error {
	return func(t *model.Resume) error {
		walkTextRefs(t, func(r *textRef) bool {
			s := r.text.Span
			if r.id == skipID || s[0] > span[0] || s[1] < span[1] || s == span {
				return false
			}
			raw := src[s[0]:s[1]]
			rel0, rel1 := span[0]-s[0], span[1]-s[0]
			r.set(decodedValue(raw[:rel0] + replacement + raw[rel1:]))
			return false
		})
		return nil
	}
}

// chainMirrors runs mirror steps in order.
func chainMirrors(steps ...func(*model.Resume) error) func(*model.Resume) error {
	return func(t *model.Resume) error {
		for _, step := range steps {
			if err := step(t); err != nil {
				return err
			}
		}
		return nil
	}
}

// wrapBullet places raw source text into a bullet command. Commands that
// take the text as an argument get braces; bare \item gets a space.
func wrapBullet(tree *model.Resume, command, rawText string) string {
	if command == `\item` {
		return command + " " + rawText
	}
	for _, c := range tree.Meta.Commands {
		if c.Name == command {
			if c.ArgCount >= 1 {
				return command + "{" + rawText + "}"
			}
			return command + " " + rawText
		}
	}
	return command + "{" + rawText + "}"
}

// mirrorRemoveBullet removes a bullet from its list in the expected tree.
// A bullet-less list re-parses with the parser's default bullet command, so
// the mirror matches that.
func mirrorRemoveBullet(list *model.BulletList, index int) {
	list.Bullets = append(list.Bullets[:index], list.Bullets[index+1:]...)
	if len(list.Bullets) == 0 {
		list.BulletCommand = `\item`
	}
}

// --- edit / editUrl ---

func planEdit(src string, tree *model.Resume, op model.Op) (planned, error) {
	ref := findTextRef(tree, op.ID)
	if ref == nil {
		return planned{}, fmt.Errorf("edit: unknown id %q", op.ID)
	}
	if err := textconv.Validate(ref.text.Mode, op.Value); err != nil {
		return planned{}, fmt.Errorf("edit: %w", err)
	}
	latex, err := textconv.ToLatex(ref.text, op.Value, src)
	if err != nil {
		return planned{}, fmt.Errorf("edit: %w", err)
	}
	expected := decodedValue(latex)
	return planned{
		patches: []Patch{{Start: ref.text.Span[0], End: ref.text.Span[1], Replacement: latex}},
		mirror: chainMirrors(func(t *model.Resume) error {
			ref := findTextRef(t, op.ID)
			if ref == nil {
				return fmt.Errorf("edit: unknown id %q", op.ID)
			}
			ref.set(expected)
			return nil
		}, spliceAncestorMirror(src, ref.text.Span, latex, op.ID)),
	}, nil
}

// validateEditURL enforces the scheme allowlist (https, http, mailto) and
// rejects characters that break \href parsing, matching textconv's rules.
func validateEditURL(url string) error {
	if len(url) > maxValueLen || !utf8.ValidString(url) {
		return errors.New("invalid url length or encoding")
	}
	ok := false
	for _, scheme := range []string{"https://", "http://", "mailto:"} {
		if strings.HasPrefix(url, scheme) {
			ok = true
			break
		}
	}
	if !ok {
		return errors.New("url must use https, http, or mailto")
	}
	if strings.ContainsAny(url, "{}\\ \t\n()") {
		return errors.New("url contains unsupported characters")
	}
	return nil
}

func planEditURL(src string, tree *model.Resume, op model.Op) (planned, error) {
	ref := findTextRef(tree, op.ID)
	if ref == nil {
		return planned{}, fmt.Errorf("editUrl: unknown id %q", op.ID)
	}
	if ref.text.URL == nil {
		return planned{}, fmt.Errorf("editUrl: id %q has no link", op.ID)
	}
	if err := validateEditURL(op.URL); err != nil {
		return planned{}, fmt.Errorf("editUrl: %w", err)
	}
	urlSpan := ref.text.URL.Span
	if urlSpan[0] < ref.text.Span[0] || urlSpan[1] > ref.text.Span[1] {
		return planned{}, errors.New("editUrl: link span outside text span")
	}
	// The whole text re-decodes after the splice; compute the expected text.
	raw := src[ref.text.Span[0]:ref.text.Span[1]]
	rel := [2]int{urlSpan[0] - ref.text.Span[0], urlSpan[1] - ref.text.Span[0]}
	expected := decodedValue(raw[:rel[0]] + op.URL + raw[rel[1]:])
	return planned{
		patches: []Patch{{Start: urlSpan[0], End: urlSpan[1], Replacement: op.URL}},
		mirror: chainMirrors(func(t *model.Resume) error {
			ref := findTextRef(t, op.ID)
			if ref == nil {
				return fmt.Errorf("editUrl: unknown id %q", op.ID)
			}
			ref.set(expected)
			return nil
		}, spliceAncestorMirror(src, urlSpan, op.URL, op.ID)),
	}, nil
}

// --- hide / show ---

func planHide(src string, tree *model.Resume, op model.Op, hide bool) (planned, error) {
	name := "show"
	if hide {
		name = "hide"
	}
	// Locate a hideable node: bullet, entry, skill line, or section.
	var span model.Span
	var hidden bool
	found := true
	if bl := findBullet(tree, op.ID); bl != nil {
		span, hidden = bl.bullet.Span, bl.bullet.Hidden
	} else if e, _ := findEntry(tree, op.ID); e != nil {
		span, hidden = e.Span, e.Hidden
	} else if sk := findSkill(tree, op.ID); sk != nil {
		span, hidden = sk.Span, sk.Hidden
	} else if s := findSection(tree, op.ID); s != nil {
		span, hidden = s.Span, s.Hidden
	} else {
		found = false
	}
	if !found {
		return planned{}, fmt.Errorf("%s: unknown id %q", name, op.ID)
	}
	if hidden == hide {
		return planned{}, fmt.Errorf("%s: node is already in that state", name)
	}

	if hide {
		// Never comment out \end{document} when a section runs to EOF.
		if span[1] > docEndIndex(src) {
			span[1] = lineStartOf(src, docEndIndex(src))
		}
		start, end, _, err := blockRange(src, span)
		if err != nil {
			return planned{}, fmt.Errorf("hide: %w", err)
		}
		return planned{
			patches: []Patch{{Start: start, End: end, Replacement: commentLines(src[start:end])}},
			mirror:  setHiddenMirror(op.ID, true),
		}, nil
	}
	return planned{
		patches: []Patch{{Start: span[0], End: span[1], Replacement: uncommentLines(src[span[0]:span[1]])}},
		mirror:  setHiddenMirror(op.ID, false),
	}, nil
}

func setHiddenMirror(id string, hidden bool) func(*model.Resume) error {
	return func(t *model.Resume) error {
		if bl := findBullet(t, id); bl != nil {
			bl.bullet.Hidden = hidden
			return nil
		}
		if e, _ := findEntry(t, id); e != nil {
			e.Hidden = hidden
			return nil
		}
		if sk := findSkill(t, id); sk != nil {
			sk.Hidden = hidden
			return nil
		}
		if s := findSection(t, id); s != nil {
			s.Hidden = hidden
			return nil
		}
		return fmt.Errorf("unknown id %q", id)
	}
}

// --- add / delete ---

func planAdd(src string, tree *model.Resume, op model.Op) (planned, error) {
	ll := findList(tree, op.ListID)
	if ll == nil {
		return planned{}, fmt.Errorf("add: unknown list id %q", op.ListID)
	}
	if err := textconv.Validate("text", op.Value); err != nil {
		return planned{}, fmt.Errorf("add: %w", err)
	}
	var after *bulletLoc
	if op.After != nil {
		after = findBulletInList(ll.list, *op.After)
		if after == nil {
			return planned{}, fmt.Errorf("add: after id %q is not in the list", *op.After)
		}
	}
	// Clone a visible sibling's format: the after-bullet, else the first
	// visible bullet. With no sibling, synthesize from the list's command.
	var template *model.Bullet
	if after != nil && !after.bullet.Hidden {
		template = after.bullet
	}
	if template == nil {
		for i := range ll.list.Bullets {
			if !ll.list.Bullets[i].Hidden {
				template = &ll.list.Bullets[i]
				break
			}
		}
	}
	var newLatex, newLine string
	if template != nil {
		original := model.Text{Value: template.Text.Value, Mode: "text", Span: template.Text.Span}
		latex, err := textconv.ToLatex(original, op.Value, src)
		if err != nil {
			return planned{}, fmt.Errorf("add: %w", err)
		}
		newLatex = latex
		start, end := lineBlock(src, template.Span)
		raw := src[start:end]
		rel := [2]int{template.Text.Span[0] - start, template.Text.Span[1] - start}
		if rel[0] < 0 || rel[1] > len(raw) {
			return planned{}, errors.New("add: template text span outside its block")
		}
		newLine = raw[:rel[0]] + newLatex + raw[rel[1]:]
		if template.Hidden {
			newLine = uncommentLines(newLine)
		}
	} else {
		latex, err := textconv.ToLatex(model.Text{Mode: "text"}, op.Value, src)
		if err != nil {
			return planned{}, fmt.Errorf("add: %w", err)
		}
		newLatex = latex
		newLine = indentOfLine(src, ll.list.Span[0]) + "  " + wrapBullet(tree, ll.list.BulletCommand, newLatex)
	}
	point := lineEndOf(src, ll.list.Span[0])
	insertAt := 0
	if after != nil {
		point = lineEndOf(src, after.bullet.Span[1])
		insertAt = after.index + 1
	}
	expected := decodedValue(newLatex)
	return planned{
		patches: []Patch{{Start: point, End: point, Replacement: "\n" + newLine}},
		mirror: func(t *model.Resume) error {
			ll := findList(t, op.ListID)
			if ll == nil {
				return fmt.Errorf("add: unknown list id %q", op.ListID)
			}
			bullet := model.Bullet{ID: newID("b"), Text: expected}
			ll.list.Bullets = append(ll.list.Bullets, model.Bullet{})
			copy(ll.list.Bullets[insertAt+1:], ll.list.Bullets[insertAt:])
			ll.list.Bullets[insertAt] = bullet
			return nil
		},
	}, nil
}

func planDelete(src string, tree *model.Resume, op model.Op) (planned, error) {
	bl := findBullet(tree, op.ID)
	if bl == nil {
		return planned{}, fmt.Errorf("delete: unknown bullet id %q (only bullets can be deleted)", op.ID)
	}
	start, _, delEnd, err := blockRange(src, bl.bullet.Span)
	if err != nil {
		return planned{}, fmt.Errorf("delete: %w", err)
	}
	return planned{
		patches: []Patch{{Start: start, End: delEnd}},
		mirror: func(t *model.Resume) error {
			bl := findBullet(t, op.ID)
			if bl == nil {
				return fmt.Errorf("delete: unknown bullet id %q", op.ID)
			}
			mirrorRemoveBullet(bl.list, bl.index)
			return nil
		},
	}, nil
}

// --- reorder ---

type reorderChild struct {
	id   string
	span model.Span
}

func planReorder(src string, tree *model.Resume, op model.Op) (planned, error) {
	var children []reorderChild
	var mirrorApply func(t *model.Resume, order []string) error
	switch {
	case op.ParentID == RootParentID:
		for i := range tree.Sections {
			span := tree.Sections[i].Span
			children = append(children, reorderChild{id: tree.Sections[i].ID, span: span})
		}
		if len(children) > 0 {
			// Keep \end{document} anchored at the end of the file.
			last := &children[len(children)-1]
			if last.span[1] > docEndIndex(src) {
				last.span[1] = lineStartOf(src, docEndIndex(src))
			}
		}
		mirrorApply = func(t *model.Resume, order []string) error {
			return reorderSlice(len(t.Sections), order, func(i int) string { return t.Sections[i].ID },
				func(perm []int) {
					next := make([]model.Section, len(t.Sections))
					for pos, old := range perm {
						next[pos] = t.Sections[old]
					}
					t.Sections = next
				})
		}
	case findList(tree, op.ParentID) != nil:
		ll := findList(tree, op.ParentID)
		for i := range ll.list.Bullets {
			children = append(children, reorderChild{id: ll.list.Bullets[i].ID, span: ll.list.Bullets[i].Span})
		}
		mirrorApply = func(t *model.Resume, order []string) error {
			ll := findList(t, op.ParentID)
			if ll == nil {
				return fmt.Errorf("reorder: unknown parent id %q", op.ParentID)
			}
			return reorderSlice(len(ll.list.Bullets), order, func(i int) string { return ll.list.Bullets[i].ID },
				func(perm []int) {
					next := make([]model.Bullet, len(ll.list.Bullets))
					for pos, old := range perm {
						next[pos] = ll.list.Bullets[old]
					}
					ll.list.Bullets = next
				})
		}
	case findSection(tree, op.ParentID) != nil:
		s := findSection(tree, op.ParentID)
		for _, it := range s.Items {
			span, ok := itemSpan(it)
			if !ok {
				return planned{}, errors.New("reorder: section contains an item without a span")
			}
			children = append(children, reorderChild{id: itemID(it), span: span})
		}
		mirrorApply = func(t *model.Resume, order []string) error {
			s := findSection(t, op.ParentID)
			if s == nil {
				return fmt.Errorf("reorder: unknown parent id %q", op.ParentID)
			}
			return reorderSlice(len(s.Items), order, func(i int) string { return itemID(s.Items[i]) },
				func(perm []int) {
					next := make([]model.SectionItem, len(s.Items))
					for pos, old := range perm {
						next[pos] = s.Items[old]
					}
					s.Items = next
				})
		}
	default:
		return planned{}, fmt.Errorf("reorder: unknown parent id %q", op.ParentID)
	}
	perm, err := validatePermutation(children, op.Order)
	if err != nil {
		return planned{}, fmt.Errorf("reorder: %w", err)
	}
	for i := 0; i+1 < len(children); i++ {
		if children[i].span[1] > children[i+1].span[0] {
			return planned{}, errors.New("reorder: child blocks overlap")
		}
	}
	if len(children) == 0 {
		return planned{}, errors.New("reorder: parent has no children")
	}
	start, end := children[0].span[0], children[len(children)-1].span[1]
	var b strings.Builder
	for pos, old := range perm {
		b.WriteString(src[children[old].span[0]:children[old].span[1]])
		if pos+1 < len(perm) {
			// Whitespace slots stay fixed; only the blocks move.
			b.WriteString(src[children[pos].span[1]:children[pos+1].span[0]])
		}
	}
	return planned{
		patches: []Patch{{Start: start, End: end, Replacement: b.String()}},
		mirror: func(t *model.Resume) error {
			return mirrorApply(t, op.Order)
		},
	}, nil
}

// validatePermutation maps the requested order to child indexes; the order
// must be exactly the existing child IDs, each once.
func validatePermutation(children []reorderChild, order []string) ([]int, error) {
	if len(order) != len(children) {
		return nil, fmt.Errorf("order has %d ids, parent has %d children", len(order), len(children))
	}
	index := make(map[string]int, len(children))
	for i, c := range children {
		index[c.id] = i
	}
	perm := make([]int, len(order))
	seen := make(map[string]bool, len(order))
	for pos, id := range order {
		old, ok := index[id]
		if !ok {
			return nil, fmt.Errorf("order contains unknown child id %q", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("order repeats child id %q", id)
		}
		seen[id] = true
		perm[pos] = old
	}
	return perm, nil
}

// reorderSlice validates the order against the current mirror tree and then
// applies the permutation.
func reorderSlice(n int, order []string, idAt func(int) string, apply func(perm []int)) error {
	children := make([]reorderChild, n)
	for i := 0; i < n; i++ {
		children[i] = reorderChild{id: idAt(i)}
	}
	perm, err := validatePermutation(children, order)
	if err != nil {
		return err
	}
	apply(perm)
	return nil
}

// --- move ---

func planMove(src string, tree *model.Resume, op model.Op) (planned, error) {
	bl := findBullet(tree, op.ID)
	if bl == nil {
		return planned{}, fmt.Errorf("move: unknown bullet id %q", op.ID)
	}
	if bl.bullet.Hidden {
		return planned{}, errors.New("move: unhide the bullet before moving it")
	}
	entry, _ := findEntry(tree, op.ToEntryID)
	if entry == nil {
		return planned{}, fmt.Errorf("move: unknown target entry id %q", op.ToEntryID)
	}
	rawText := src[bl.bullet.Text.Span[0]:bl.bullet.Text.Span[1]]
	cutStart, cutEnd, err := blockRangeRemoval(src, bl.bullet.Span)
	if err != nil {
		return planned{}, fmt.Errorf("move: %w", err)
	}

	var point int
	var insert string
	var newList *model.BulletList
	if entry.Bullets != nil {
		target := entry.Bullets
		if op.After != nil {
			after := findBulletInList(target, *op.After)
			if after == nil {
				return planned{}, fmt.Errorf("move: after id %q is not in the target entry", *op.After)
			}
			point = lineEndOf(src, after.bullet.Span[1])
		} else {
			point = lineEndOf(src, target.Span[0])
		}
		if target.BulletCommand == bl.list.BulletCommand {
			// Same command: the whole source block moves verbatim.
			start, end := lineBlock(src, bl.bullet.Span)
			insert = "\n" + src[start:end]
		} else {
			// Different command: rewrap the text in the target's command
			// with the target list's indentation (spec §5, locked).
			indent := indentOfLine(src, target.Span[0]) + "  "
			for i := range target.Bullets {
				if !target.Bullets[i].Hidden {
					indent = indentOfLine(src, target.Bullets[i].Span[0])
					break
				}
			}
			insert = "\n" + indent + wrapBullet(tree, target.BulletCommand, rawText)
		}
	} else {
		if op.After != nil {
			return planned{}, errors.New("move: target entry has no bullet list")
		}
		tmpl := firstList(tree)
		if tmpl == nil {
			return planned{}, errors.New("move: no bullet list in the template to copy")
		}
		indent := indentOfLine(src, entry.Span[0])
		point = lineEndOf(src, entry.Span[1])
		insert = "\n" + indent + "  " + tmpl.Open +
			"\n" + indent + "    " + wrapBullet(tree, tmpl.BulletCommand, rawText) +
			"\n" + indent + "  " + tmpl.Close
		newList = &model.BulletList{
			Kind:          "bullets",
			ID:            newID("l"),
			Open:          tmpl.Open,
			Close:         tmpl.Close,
			BulletCommand: tmpl.BulletCommand,
			Bullets:       []model.Bullet{},
		}
	}
	moved := model.Bullet{ID: op.ID, Text: bl.bullet.Text}
	moved.Text.Span = model.Span{}
	return planned{
		patches: []Patch{
			{Start: cutStart, End: cutEnd},
			{Start: point, End: point, Replacement: insert},
		},
		mirror: func(t *model.Resume) error {
			bl := findBullet(t, op.ID)
			if bl == nil {
				return fmt.Errorf("move: unknown bullet id %q", op.ID)
			}
			entry, _ := findEntry(t, op.ToEntryID)
			if entry == nil {
				return fmt.Errorf("move: unknown target entry id %q", op.ToEntryID)
			}
			bl.list.Bullets = append(bl.list.Bullets[:bl.index], bl.list.Bullets[bl.index+1:]...)
			if len(bl.list.Bullets) == 0 && bl.list != entry.Bullets {
				bl.list.BulletCommand = `\item`
			}
			if newList != nil {
				newList.Bullets = []model.Bullet{moved}
				entry.Bullets = newList
				return nil
			}
			target := entry.Bullets
			// Find the insert index after removal: when the bullet moves
			// within its own list, removal shifts the after-bullet's index.
			at := 0
			if op.After != nil {
				after := findBulletInList(target, *op.After)
				if after == nil {
					return fmt.Errorf("move: after id %q is not in the target entry", *op.After)
				}
				at = after.index + 1
			}
			target.Bullets = append(target.Bullets, model.Bullet{})
			copy(target.Bullets[at+1:], target.Bullets[at:])
			target.Bullets[at] = moved
			return nil
		},
	}, nil
}

// --- setStyle / replaceImage ---

// styleAdditionLine maps a missing style setting to one marked preamble
// line. Keys without a safe one-line form are rejected.
func styleAdditionLine(key, value string) (string, error) {
	if strings.ContainsAny(value, "{}\n\r%") {
		return "", errors.New("value contains unsupported characters")
	}
	switch key {
	case "margin":
		return `\geometry{margin=` + value + `}`, nil
	case "pageSize":
		return `\geometry{` + value + `}`, nil
	case "fontFamily":
		return `\usepackage{` + value + `}`, nil
	case "bulletSpacing":
		return `\setlength{\itemsep}{` + value + `}`, nil
	}
	return "", fmt.Errorf("cannot add a new %q setting", key)
}

func planSetStyle(src string, tree *model.Resume, op model.Op) (planned, error) {
	st := findStyle(tree, op.ID)
	if st == nil {
		return planned{}, fmt.Errorf("setStyle: unknown id %q", op.ID)
	}
	if len(op.Value) > maxValueLen || !utf8.ValidString(op.Value) ||
		strings.ContainsAny(op.Value, "\n\r%") {
		return planned{}, errors.New("setStyle: invalid value")
	}
	// A package-font setting may only switch to another listed package font;
	// anything else would write an arbitrary \usepackage name.
	if st.Key == "fontFamily" && st.Span != nil &&
		fonts.IsPackage(src[st.Span[0]:st.Span[1]]) && !fonts.IsPackage(op.Value) {
		return planned{}, errors.New("setStyle: unsupported font")
	}
	var patches []Patch
	if st.Span != nil {
		patches = []Patch{{Start: st.Span[0], End: st.Span[1], Replacement: op.Value}}
	} else {
		line, err := styleAdditionLine(st.Key, op.Value)
		if err != nil {
			return planned{}, fmt.Errorf("setStyle: %w", err)
		}
		idx := strings.Index(src, `\begin{document}`)
		if idx < 0 {
			return planned{}, errors.New("setStyle: no document environment")
		}
		patches = []Patch{{Start: idx, End: idx, Replacement: "% added by Resume Editor\n" + line + "\n"}}
	}
	return planned{
		patches: patches,
		mirror: func(t *model.Resume) error {
			st := findStyle(t, op.ID)
			if st == nil {
				return fmt.Errorf("setStyle: unknown id %q", op.ID)
			}
			st.Value = op.Value
			return nil
		},
	}, nil
}

// planReplaceImage touches no source: the asset map lives outside the .tex
// (phase 05). The engine only validates that the asset reference exists.
func planReplaceImage(tree *model.Resume, op model.Op) (planned, error) {
	if findAsset(tree, op.ID) == nil {
		return planned{}, fmt.Errorf("replaceImage: unknown asset id %q", op.ID)
	}
	return planned{
		patches: nil,
		mirror:  func(t *model.Resume) error { return nil },
	}, nil
}
