package engine

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/checks"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

func loadJake(t *testing.T) (string, *model.Resume) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/resumes/jake.tex")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := parser.Parse(string(data), parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return string(data), tree
}

// synth has single-line entries, a bare itemize list (bullet command \item),
// and an entry without any bullet list.
const synth = `\documentclass{article}
\newcommand{\resumeItem}[1]{\item{#1}}
\newcommand{\resumeSubheading}[4]{\textbf{#1} & #2 \\ #3 & #4}
\newcommand{\resumeItemListStart}{\begin{itemize}}
\newcommand{\resumeItemListEnd}{\end{itemize}}
\begin{document}
\section{Experience}
\resumeSubheading{Acme}{Remote}{Dev}{2020 -- 2021}
\resumeItemListStart
\resumeItem{Did things}
\resumeItem{Did more}
\resumeItemListEnd
\section{Projects}
\begin{itemize}
\item First project fact
\item Second project fact
\end{itemize}
\section{Solo}
\resumeSubheading{Lone}{Here}{Dev}{2024}
\end{document}`

func loadSynth(t *testing.T) (string, *model.Resume) {
	t.Helper()
	tree, err := parser.Parse(synth, parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Sections) != 3 {
		t.Fatalf("synthetic fixture: want 3 sections, got %d", len(tree.Sections))
	}
	return synth, tree
}

func strptr(s string) *string { return &s }

func applyOne(t *testing.T, src string, tree *model.Resume, op model.Op) *ApplyResult {
	t.Helper()
	res, err := ApplyOps(src, tree, []model.Op{op})
	if err != nil {
		t.Fatalf("ApplyOps(%s) failed: %v", op.Kind, err)
	}
	assertConsistent(t, src, tree, res)
	return res
}

// assertConsistent checks the post-op invariants: hash matches, the result
// re-parses to an equal tree, and round-trip check 1 has no new failures.
func assertConsistent(t *testing.T, oldSrc string, oldTree *model.Resume, res *ApplyResult) {
	t.Helper()
	if res.Tree.SourceHash != sourceHash(res.Source) {
		t.Fatal("result tree sourceHash does not match result source")
	}
	reparsed, err := parser.Parse(res.Source, parser.ParseOptions{})
	if err != nil {
		t.Fatalf("result source does not parse: %v", err)
	}
	if !treesEqual(reparsed, res.Tree) {
		t.Fatal("result tree does not match a fresh re-parse of the result source")
	}
	baseline := roundTripFailures(oldSrc, oldTree)
	fresh := roundTripFailures(res.Source, res.Tree)
	if fresh > baseline {
		t.Fatalf("round-trip failures grew from %d to %d", baseline, fresh)
	}
}

// roundTripFailures runs check 1 on a copy (the check mutates the tree).
func roundTripFailures(src string, tree *model.Resume) int {
	dup, err := deepCopy(tree)
	if err != nil {
		return -1
	}
	return len(checks.CheckRoundTrip(src, dup))
}

func firstBullet(t *testing.T, tree *model.Resume) (*model.Entry, *model.Bullet) {
	t.Helper()
	for _, s := range tree.Sections {
		for _, it := range s.Items {
			if it.Entry != nil && it.Entry.Bullets != nil && len(it.Entry.Bullets.Bullets) > 0 {
				return it.Entry, &it.Entry.Bullets.Bullets[0]
			}
		}
	}
	t.Fatal("no bullet found")
	return nil, nil
}

// --- edit ---

func TestEditBullet(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	res := applyOne(t, src, tree, model.Op{Kind: "edit", ID: b.ID, Value: "Built a REST API from scratch"})
	if !strings.Contains(res.Source, `\resumeItem{Built a REST API from scratch}`) {
		t.Fatal("new source missing the edited bullet")
	}
	if got := res.Tree.Sections[1].Items[0].Entry.Bullets.Bullets[0].Text.Value; got != "Built a REST API from scratch" {
		t.Fatalf("tree value = %q", got)
	}
	// IDs are stable across the op.
	if res.Tree.Sections[1].Items[0].Entry.Bullets.Bullets[0].ID != b.ID {
		t.Fatal("bullet ID changed")
	}
}

func TestEditBulletMarkdown(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	res := applyOne(t, src, tree, model.Op{Kind: "edit", ID: b.ID, Value: "Cut latency by **40%** using caching"})
	if !strings.Contains(res.Source, `\textbf{40\%}`) {
		t.Fatalf("expected encoded bold and escaped %%, got source:\n%s", snippet(res.Source, "latency"))
	}
}

func TestEditEntryFieldAndTitle(t *testing.T) {
	src, tree := loadJake(t)
	entry := tree.Sections[1].Items[0].Entry
	res := applyOne(t, src, tree, model.Op{Kind: "edit", ID: entry.Fields[0].ID, Value: "Senior Research Assistant"})
	if !strings.Contains(res.Source, "{Senior Research Assistant}") {
		t.Fatal("entry field not spliced")
	}
	res = applyOne(t, src, tree, model.Op{Kind: "edit", ID: tree.Sections[0].Title.ID, Value: "Education & Awards"})
	if !strings.Contains(res.Source, `\section{Education \& Awards}`) {
		t.Fatal("section title not spliced")
	}
}

func TestEditSkillValue(t *testing.T) {
	src, tree := loadJake(t)
	var sk *model.SkillLine
	for _, s := range tree.Sections {
		for _, it := range s.Items {
			if it.Skill != nil {
				sk = it.Skill
			}
		}
	}
	if sk == nil {
		t.Fatal("no skill line in jake")
	}
	res := applyOne(t, src, tree, model.Op{Kind: "edit", ID: sk.Value.ID, Value: "Go, Python, Rust"})
	if !strings.Contains(res.Source, "Go, Python, Rust") {
		t.Fatal("skill value not spliced")
	}
}

func TestEditRejectsBadInput(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	for name, op := range map[string]model.Op{
		"unknown id":    {Kind: "edit", ID: "b_nope", Value: "x"},
		"too long":      {Kind: "edit", ID: b.ID, Value: strings.Repeat("a", 10001)},
		"braces":        {Kind: "edit", ID: b.ID, Value: "inject{evil}"},
		"raw backslash": {Kind: "edit", ID: b.ID, Value: `inject \evil here`},
		"unknown kind":  {Kind: "explode", ID: b.ID, Value: "x"},
	} {
		if _, err := ApplyOps(src, tree, []model.Op{op}); err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
	}
}

// --- editUrl ---

func TestEditURL(t *testing.T) {
	src, tree := loadJake(t)
	var target *model.Field
	for i := range tree.Header.Items {
		if tree.Header.Items[i].URL != nil {
			target = &tree.Header.Items[i]
			break
		}
	}
	if target == nil {
		t.Fatal("no header link in jake")
	}
	res := applyOne(t, src, tree, model.Op{Kind: "editUrl", ID: target.ID, URL: "https://example.com/new"})
	if !strings.Contains(res.Source, `\href{https://example.com/new}`) {
		t.Fatal("url not spliced")
	}
	// The visible text value follows the link change.
	found := false
	for _, it := range res.Tree.Header.Items {
		if it.ID == target.ID && it.URL != nil && it.URL.Value == "https://example.com/new" &&
			strings.Contains(it.Value, "](https://example.com/new)") {
			found = true
		}
	}
	if !found {
		t.Fatal("tree link value not updated")
	}
}

func TestEditURLSchemeAllowlist(t *testing.T) {
	src, tree := loadJake(t)
	var target *model.Field
	for i := range tree.Header.Items {
		if tree.Header.Items[i].URL != nil {
			target = &tree.Header.Items[i]
			break
		}
	}
	for _, url := range []string{
		"javascript:alert(1)", "data:text/html,<script>1</script>", "ftp://x",
		"file:///etc/passwd", "//protocol-relative", "https://ok but has space",
	} {
		if _, err := ApplyOps(src, tree, []model.Op{{Kind: "editUrl", ID: target.ID, URL: url}}); err == nil {
			t.Fatalf("url %q: expected rejection", url)
		}
	}
	for _, url := range []string{"https://a.b/c", "http://a.b", "mailto:x@y.z"} {
		if _, err := ApplyOps(src, tree, []model.Op{{Kind: "editUrl", ID: target.ID, URL: url}}); err != nil {
			t.Fatalf("url %q: %v", url, err)
		}
	}
}

// --- hide / show ---

func TestHideShowBullet(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	res := applyOne(t, src, tree, model.Op{Kind: "hide", ID: b.ID})
	line := `%        	`
	_ = line
	if !strings.Contains(res.Source, "% ") || !strings.Contains(res.Source, "%         \\resumeItem{") {
		t.Fatalf("bullet not commented out:\n%s", snippet(res.Source, "resumeItem"))
	}
	hidden := res.Tree.Sections[1].Items[0].Entry.Bullets.Bullets[0]
	if !hidden.Hidden || hidden.ID != b.ID {
		t.Fatal("tree bullet should be hidden with the same ID")
	}
	back := applyOne(t, res.Source, res.Tree, model.Op{Kind: "show", ID: b.ID})
	if back.Source != src {
		t.Fatal("hide then show should restore the exact source")
	}
	if back.Tree.Sections[1].Items[0].Entry.Bullets.Bullets[0].Hidden {
		t.Fatal("bullet still hidden after show")
	}
}

func TestHideShowEntrySingleLine(t *testing.T) {
	src, tree := loadSynth(t)
	solo := tree.Sections[2].Items[0].Entry
	res := applyOne(t, src, tree, model.Op{Kind: "hide", ID: solo.ID})
	if !strings.Contains(res.Source, "% \\resumeSubheading{Lone}") {
		t.Fatalf("entry not commented:\n%s", res.Source)
	}
	if !res.Tree.Sections[2].Items[0].Entry.Hidden {
		t.Fatal("entry not hidden in tree")
	}
	back := applyOne(t, res.Source, res.Tree, model.Op{Kind: "show", ID: solo.ID})
	if back.Source != src {
		t.Fatal("hide then show should restore the exact source")
	}
}

func TestHideEntryWithBulletsRejected(t *testing.T) {
	// Parser limitation (tasks/FINAL-BLOCKERS.md): a multi-line commented
	// entry is dropped on re-parse, so the double-apply check rejects it.
	src, tree := loadJake(t)
	entry := tree.Sections[1].Items[0].Entry
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "hide", ID: entry.ID}}); err == nil {
		t.Fatal("expected rejection")
	}
}

func TestHideSectionRejected(t *testing.T) {
	// Parser limitation (tasks/FINAL-BLOCKERS.md): commented \section lines
	// vanish from the tree, so hiding a section is rejected.
	src, tree := loadJake(t)
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "hide", ID: tree.Sections[0].ID}}); err == nil {
		t.Fatal("expected rejection")
	}
}

// --- add / delete ---

func TestAddBulletFirst(t *testing.T) {
	src, tree := loadJake(t)
	entry, b := firstBullet(t, tree)
	res := applyOne(t, src, tree, model.Op{Kind: "add", ListID: entry.Bullets.ID, Value: "Brand new bullet"})
	list := res.Tree.Sections[1].Items[0].Entry.Bullets
	if len(list.Bullets) != len(entry.Bullets.Bullets)+1 {
		t.Fatal("bullet count did not grow")
	}
	if list.Bullets[0].Text.Value != "Brand new bullet" || list.Bullets[1].ID != b.ID {
		t.Fatal("new bullet should be first, existing IDs preserved")
	}
	if !strings.Contains(res.Source, "        \\resumeItem{Brand new bullet}\n") {
		t.Fatalf("clone format wrong:\n%s", snippet(res.Source, "Brand new"))
	}
}

func TestAddBulletAfterSibling(t *testing.T) {
	src, tree := loadJake(t)
	entry, _ := firstBullet(t, tree)
	mid := entry.Bullets.Bullets[1]
	res := applyOne(t, src, tree, model.Op{Kind: "add", ListID: entry.Bullets.ID, After: strptr(mid.ID), Value: "Inserted bullet"})
	list := res.Tree.Sections[1].Items[0].Entry.Bullets
	if list.Bullets[2].Text.Value != "Inserted bullet" {
		t.Fatalf("bullet inserted at wrong position: %q", list.Bullets[2].Text.Value)
	}
}

func TestAddBulletItemCommand(t *testing.T) {
	src, tree := loadSynth(t)
	projects := tree.Sections[1].Items[0].Bullets
	res := applyOne(t, src, tree, model.Op{Kind: "add", ListID: projects.ID, Value: "Third project fact"})
	if !strings.Contains(res.Source, "\n\\item Third project fact\n") {
		t.Fatalf("expected \\item clone:\n%s", snippet(res.Source, "Third"))
	}
}

func TestAddRejects(t *testing.T) {
	src, tree := loadJake(t)
	entry, b := firstBullet(t, tree)
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "add", ListID: "l_nope", Value: "x"}}); err == nil {
		t.Fatal("unknown list id should fail")
	}
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "add", ListID: entry.Bullets.ID, After: strptr("b_nope"), Value: "x"}}); err == nil {
		t.Fatal("foreign after id should fail")
	}
	// A bullet from another list is not a valid after anchor.
	other := tree.Sections[1].Items[1].Entry.Bullets
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "add", ListID: other.ID, After: strptr(b.ID), Value: "x"}}); err == nil {
		t.Fatal("after id from another list should fail")
	}
}

func TestDeleteBullet(t *testing.T) {
	src, tree := loadJake(t)
	entry, b := firstBullet(t, tree)
	res := applyOne(t, src, tree, model.Op{Kind: "delete", ID: b.ID})
	list := res.Tree.Sections[1].Items[0].Entry.Bullets
	if len(list.Bullets) != len(entry.Bullets.Bullets)-1 {
		t.Fatal("bullet count did not shrink")
	}
	if strings.Contains(res.Source, "Developed a REST API") {
		t.Fatal("deleted text still present")
	}
	// No blank line left behind: the next bullet follows the list open.
	if !strings.Contains(res.Source, "\\resumeItemListStart\n        \\resumeItem{Developed a full-stack") {
		t.Fatalf("delete left a gap:\n%s", snippet(res.Source, "resumeItemListStart"))
	}
}

func TestDeleteRejectsNonBullet(t *testing.T) {
	src, tree := loadJake(t)
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "delete", ID: tree.Sections[0].ID}}); err == nil {
		t.Fatal("deleting a section must fail")
	}
}

// --- reorder ---

func TestReorderBullets(t *testing.T) {
	src, tree := loadJake(t)
	entry, _ := firstBullet(t, tree)
	l := entry.Bullets
	order := []string{l.Bullets[2].ID, l.Bullets[0].ID, l.Bullets[1].ID}
	res := applyOne(t, src, tree, model.Op{Kind: "reorder", ParentID: l.ID, Order: order})
	got := res.Tree.Sections[1].Items[0].Entry.Bullets.Bullets
	if got[0].ID != order[0] || got[1].ID != order[1] || got[2].ID != order[2] {
		t.Fatal("tree order wrong")
	}
	first := strings.Index(res.Source, "Explored ways to visualize")
	second := strings.Index(res.Source, "Developed a REST API")
	if first < 0 || second < 0 || first > second {
		t.Fatal("source order wrong")
	}
}

func TestReorderEntries(t *testing.T) {
	src, tree := loadJake(t)
	sec := tree.Sections[1]
	ids := []string{sec.Items[2].Entry.ID, sec.Items[0].Entry.ID, sec.Items[1].Entry.ID}
	res := applyOne(t, src, tree, model.Op{Kind: "reorder", ParentID: sec.ID, Order: ids})
	got := res.Tree.Sections[1].Items
	if got[0].Entry.ID != ids[0] || got[1].Entry.ID != ids[1] || got[2].Entry.ID != ids[2] {
		t.Fatal("entry order wrong")
	}
}

func TestReorderSections(t *testing.T) {
	src, tree := loadJake(t)
	var ids []string
	for _, s := range tree.Sections {
		ids = append(ids, s.ID)
	}
	order := []string{ids[3], ids[2], ids[1], ids[0]}
	res := applyOne(t, src, tree, model.Op{Kind: "reorder", ParentID: RootParentID, Order: order})
	for i, s := range res.Tree.Sections {
		if s.ID != order[i] {
			t.Fatal("section order wrong")
		}
	}
	if !strings.HasSuffix(strings.TrimRight(res.Source, "\n"), `\end{document}`) {
		t.Fatal(`\end{document} must stay at the end`)
	}
}

func TestReorderRejectsBadPermutations(t *testing.T) {
	src, tree := loadJake(t)
	entry, _ := firstBullet(t, tree)
	l := entry.Bullets
	cases := map[string][]string{
		"too few":      {l.Bullets[0].ID, l.Bullets[1].ID},
		"duplicate":    {l.Bullets[0].ID, l.Bullets[0].ID, l.Bullets[1].ID},
		"unknown id":   {l.Bullets[0].ID, l.Bullets[1].ID, "b_nope"},
		"too many":     {l.Bullets[0].ID, l.Bullets[1].ID, l.Bullets[2].ID, l.Bullets[0].ID},
		"other parent": {},
	}
	for name, order := range cases {
		parent := l.ID
		if name == "other parent" {
			parent = "x_nope"
			order = []string{l.Bullets[0].ID}
		}
		if _, err := ApplyOps(src, tree, []model.Op{{Kind: "reorder", ParentID: parent, Order: order}}); err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
	}
}

// --- move ---

func TestMoveBulletSameCommand(t *testing.T) {
	src, tree := loadJake(t)
	sec := tree.Sections[1]
	e1, e2 := sec.Items[0].Entry, sec.Items[1].Entry
	b := e1.Bullets.Bullets[0]
	res := applyOne(t, src, tree, model.Op{Kind: "move", ID: b.ID, ToEntryID: e2.ID})
	got1 := res.Tree.Sections[1].Items[0].Entry.Bullets.Bullets
	got2 := res.Tree.Sections[1].Items[1].Entry.Bullets.Bullets
	if len(got1) != 2 || len(got2) != 4 {
		t.Fatalf("list sizes wrong: %d, %d", len(got1), len(got2))
	}
	if got2[0].ID != b.ID || got2[0].Text.Value != b.Text.Value {
		t.Fatal("moved bullet should be first in the target with its ID and text")
	}
}

func TestMoveBulletRewrapsCommand(t *testing.T) {
	// Spec §5: different bullet command → text placed in the target command.
	src, tree := loadSynth(t)
	acme := tree.Sections[0].Items[0].Entry
	b := acme.Bullets.Bullets[0]
	projects := tree.Sections[1].Items[0].Bullets // \item list, not entry-owned
	_ = projects
	// Move \resumeItem bullet into the bare itemize section list: not an
	// entry target, so instead move within an entry-less target — skip.
	// Real wrap test: move an \item bullet into Acme's \resumeItem list.
	itemBullet := projects.Bullets[0]
	res := applyOne(t, src, tree, model.Op{Kind: "move", ID: itemBullet.ID, ToEntryID: acme.ID})
	if !strings.Contains(res.Source, "\\resumeItem{First project fact}") {
		t.Fatalf("bullet not rewrapped:\n%s", res.Source)
	}
	moved := res.Tree.Sections[0].Items[0].Entry.Bullets.Bullets[0]
	if moved.ID != itemBullet.ID || moved.Text.Value != "First project fact" {
		t.Fatal("moved bullet mismatch")
	}
	if b.ID == itemBullet.ID {
		t.Fatal("fixture confusion")
	}
}

func TestMoveCreatesList(t *testing.T) {
	src, tree := loadSynth(t)
	acme := tree.Sections[0].Items[0].Entry
	solo := tree.Sections[2].Items[0].Entry // no bullet list
	b := acme.Bullets.Bullets[1]
	res := applyOne(t, src, tree, model.Op{Kind: "move", ID: b.ID, ToEntryID: solo.ID})
	got := res.Tree.Sections[2].Items[0].Entry.Bullets
	if got == nil || len(got.Bullets) != 1 || got.Bullets[0].ID != b.ID {
		t.Fatal("target entry should have a new list with the moved bullet")
	}
	if got.Open != `\resumeItemListStart` || got.Close != `\resumeItemListEnd` {
		t.Fatalf("list should use the template open/close, got %q %q", got.Open, got.Close)
	}
	if !strings.Contains(res.Source, "\\resumeSubheading{Lone}{Here}{Dev}{2024}\n  \\resumeItemListStart\n") {
		t.Fatalf("list inserted in the wrong place:\n%s", res.Source)
	}
}

func TestMoveRejects(t *testing.T) {
	src, tree := loadJake(t)
	entry, b := firstBullet(t, tree)
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "move", ID: b.ID, ToEntryID: "e_nope"}}); err == nil {
		t.Fatal("unknown target entry should fail")
	}
	other := tree.Sections[1].Items[1].Entry
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "move", ID: b.ID, ToEntryID: other.ID, After: strptr(b.ID)}}); err == nil {
		t.Fatal("after id outside target list should fail")
	}
	// Hidden bullets must be shown first.
	res := applyOne(t, src, tree, model.Op{Kind: "hide", ID: b.ID})
	if _, err := ApplyOps(res.Source, res.Tree, []model.Op{{Kind: "move", ID: b.ID, ToEntryID: other.ID}}); err == nil {
		t.Fatal("moving a hidden bullet should fail")
	}
	if entry == nil {
		t.Fatal("unreachable")
	}
}

// --- setStyle / replaceImage ---

func TestSetStyleInPlace(t *testing.T) {
	src, tree := loadJake(t)
	var fontSize *model.StyleSetting
	for i := range tree.Styles {
		if tree.Styles[i].Key == "fontSize" {
			fontSize = &tree.Styles[i]
		}
	}
	if fontSize == nil {
		t.Fatal("jake should have a fontSize setting")
	}
	res := applyOne(t, src, tree, model.Op{Kind: "setStyle", ID: fontSize.ID, Value: "12pt"})
	if !strings.Contains(res.Source, `\documentclass[letterpaper,12pt]{article}`) {
		t.Fatal("font size not spliced")
	}
	for _, st := range res.Tree.Styles {
		if st.Key == "fontSize" && st.Value != "12pt" {
			t.Fatalf("tree style value = %q", st.Value)
		}
	}
}

func TestSetStyleAppendsMarkedLine(t *testing.T) {
	src, tree := loadJake(t)
	// The parser only emits span-backed settings; simulate a missing one.
	tree.Styles = append(tree.Styles, model.StyleSetting{ID: "st_test", Key: "bulletSpacing", Value: "0pt"})
	res := applyOne(t, src, tree, model.Op{Kind: "setStyle", ID: "st_test", Value: "2pt"})
	if !strings.Contains(res.Source, "% added by Resume Editor\n\\setlength{\\itemsep}{2pt}\n\\begin{document}") {
		t.Fatalf("marked preamble line missing:\n%s", snippet(res.Source, "Resume Editor"))
	}
}

func TestSetStyleRejects(t *testing.T) {
	src, tree := loadJake(t)
	st := tree.Styles[0]
	for _, value := range []string{"line1\nline2", "has%percent", strings.Repeat("x", 10001)} {
		if _, err := ApplyOps(src, tree, []model.Op{{Kind: "setStyle", ID: st.ID, Value: value}}); err == nil {
			t.Fatalf("value %q: expected rejection", value)
		}
	}
}

func TestReplaceImage(t *testing.T) {
	src := `\documentclass{article}
\begin{document}
\section{Photo}
\includegraphics{logo.png}
\end{document}`
	tree, err := parser.Parse(src, parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(tree.Assets))
	}
	res := applyOne(t, src, tree, model.Op{Kind: "replaceImage", ID: tree.Assets[0].ID, AssetID: "a_uploaded"})
	if res.Source != src {
		t.Fatal("replaceImage must not touch the source")
	}
	if _, err := ApplyOps(src, tree, []model.Op{{Kind: "replaceImage", ID: "a_nope", AssetID: "x"}}); err == nil {
		t.Fatal("unknown asset id should fail")
	}
}

// --- batch semantics ---

func TestBatchAllOrNothing(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	before, _ := json.Marshal(tree)
	ops := []model.Op{
		{Kind: "edit", ID: b.ID, Value: "This must not stick"},
		{Kind: "editUrl", ID: b.ID, URL: "javascript:alert(1)"},
	}
	if _, err := ApplyOps(src, tree, ops); err == nil {
		t.Fatal("batch should be rejected")
	}
	after, _ := json.Marshal(tree)
	if string(before) != string(after) {
		t.Fatal("input tree mutated on rejection")
	}
	// The first op's splice must not be visible anywhere: no partial apply.
	res, err := ApplyOps(src, tree, ops[:1])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Source, "This must not stick") {
		t.Fatal("sanity: the single op applies on its own")
	}
	if strings.Contains(src, "This must not stick") {
		t.Fatal("unreachable: src is immutable")
	}
}

func TestStaleSourceHashReparses(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	stale, err := deepCopy(tree)
	if err != nil {
		t.Fatal(err)
	}
	stale.SourceHash = "stale"
	res, err := ApplyOps(src, stale, []model.Op{{Kind: "edit", ID: b.ID, Value: "Edited despite stale tree"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Tree.Sections[1].Items[0].Entry.Bullets.Bullets[0].ID != b.ID {
		t.Fatal("IDs should be carried over after a stale re-parse")
	}
}

func TestNilTreeParses(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	res, err := ApplyOps(src, nil, []model.Op{{Kind: "edit", ID: b.ID, Value: "x"}})
	if err == nil {
		t.Fatal("IDs from a nil tree cannot resolve; expected failure")
	}
	_ = res
}

func TestCorruptedOpRejected(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	before, _ := json.Marshal(tree)
	corruptForTest = func(s string) string {
		// Simulate a corrupted splice: unbalance the braces.
		return strings.Replace(s, "}", "", 1)
	}
	defer func() { corruptForTest = nil }()
	_, err := ApplyOps(src, tree, []model.Op{{Kind: "edit", ID: b.ID, Value: "Corrupted edit"}})
	if err == nil {
		t.Fatal("corrupted op must be rejected")
	}
	after, _ := json.Marshal(tree)
	if string(before) != string(after) {
		t.Fatal("input tree mutated")
	}
	// And the original source still parses to the original tree.
	if _, err := parser.Parse(src, parser.ParseOptions{}); err != nil {
		t.Fatal("original source broken")
	}
}

func TestUndoEditRoundTrips(t *testing.T) {
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	res := applyOne(t, src, tree, model.Op{Kind: "edit", ID: b.ID, Value: "Temporary change"})
	back := applyOne(t, res.Source, res.Tree, model.Op{Kind: "edit", ID: b.ID, Value: b.Text.Value})
	if back.Source != src {
		t.Fatal("editing back should restore the exact source")
	}
}

func snippet(src, needle string) string {
	i := strings.Index(src, needle)
	if i < 0 {
		return src
	}
	start := i - 200
	if start < 0 {
		start = 0
	}
	end := i + 200
	if end > len(src) {
		end = len(src)
	}
	return src[start:end]
}

func TestSetStyleFontAllowlist(t *testing.T) {
	src := "\\documentclass{article}\n\\usepackage{charter}\n\\begin{document}\nhi\n\\end{document}\n"
	tree, err := parser.Parse(src, parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var font *model.StyleSetting
	for i := range tree.Styles {
		if tree.Styles[i].Key == "fontFamily" {
			font = &tree.Styles[i]
		}
	}
	if font == nil || font.Value != "charter" {
		t.Fatalf("expected a charter fontFamily setting, got %+v", tree.Styles)
	}
	res := applyOne(t, src, tree, model.Op{Kind: "setStyle", ID: font.ID, Value: "mathpazo"})
	if !strings.Contains(res.Source, `\usepackage{mathpazo}`) {
		t.Fatalf("font not switched:\n%s", res.Source)
	}
	for _, bad := range []string{"shellesc", "tikz", ""} {
		if _, err := ApplyOps(src, tree, []model.Op{{Kind: "setStyle", ID: font.ID, Value: bad}}); err == nil {
			t.Fatalf("font %q: expected rejection", bad)
		}
	}
}
