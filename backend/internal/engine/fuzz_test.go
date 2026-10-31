package engine

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

// randomOp builds a random valid op against the current tree. The second
// return value is false when no op could be built (e.g. all bullets hidden).
func randomOp(rng *rand.Rand, tree *model.Resume) (model.Op, bool) {
	type bulletTarget struct {
		bullet *model.Bullet
		list   *model.BulletList
		entry  *model.Entry
	}
	var visible, hidden []bulletTarget
	var lists []*model.BulletList
	var entries []*model.Entry
	for si := range tree.Sections {
		for ii := range tree.Sections[si].Items {
			it := &tree.Sections[si].Items[ii]
			if it.Entry != nil {
				entries = append(entries, it.Entry)
			}
		}
	}
	eachList(tree, func(list *model.BulletList, entry *model.Entry, _ *model.Section) bool {
		lists = append(lists, list)
		for i := range list.Bullets {
			bt := bulletTarget{bullet: &list.Bullets[i], list: list, entry: entry}
			if list.Bullets[i].Hidden {
				hidden = append(hidden, bt)
			} else {
				visible = append(visible, bt)
			}
		}
		return false
	})
	var choices []model.Op
	if len(visible) > 0 {
		bt := visible[rng.Intn(len(visible))]
		word := fmt.Sprintf("Fuzz%d", rng.Intn(1000))
		choices = append(choices,
			model.Op{Kind: "edit", ID: bt.bullet.ID, Value: bt.bullet.Text.Value[:1] + " " + word},
			model.Op{Kind: "hide", ID: bt.bullet.ID},
			model.Op{Kind: "delete", ID: bt.bullet.ID},
		)
		if bt.entry != nil && len(entries) > 1 {
			for _, e := range entries {
				if e != bt.entry {
					choices = append(choices, model.Op{Kind: "move", ID: bt.bullet.ID, ToEntryID: e.ID})
					break
				}
			}
		}
	}
	if len(hidden) > 0 {
		choices = append(choices, model.Op{Kind: "show", ID: hidden[rng.Intn(len(hidden))].bullet.ID})
	}
	if len(lists) > 0 {
		l := lists[rng.Intn(len(lists))]
		if len(l.Bullets) >= 2 {
			order := make([]string, len(l.Bullets))
			for i, b := range l.Bullets {
				order[i] = b.ID
			}
			rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
			choices = append(choices, model.Op{Kind: "reorder", ParentID: l.ID, Order: order})
		}
		choices = append(choices, model.Op{Kind: "add", ListID: l.ID, Value: fmt.Sprintf("Added bullet %d", rng.Intn(1000))})
	}
	if len(tree.Styles) > 0 {
		st := tree.Styles[rng.Intn(len(tree.Styles))]
		if st.Span != nil && st.Key == "fontSize" {
			choices = append(choices, model.Op{Kind: "setStyle", ID: st.ID, Value: []string{"10pt", "11pt", "12pt"}[rng.Intn(3)]})
		}
	}
	if len(choices) == 0 {
		return model.Op{}, false
	}
	return choices[rng.Intn(len(choices))], true
}

// fuzzState verifies every accepted intermediate state: the returned tree is
// exactly what a fresh parse of the returned source produces, the hash
// matches, and the round-trip check reports no failures.
func fuzzStep(t *testing.T, src string, tree *model.Resume, op model.Op) (string, *model.Resume, bool) {
	t.Helper()
	res, err := ApplyOps(src, tree, []model.Op{op})
	if err != nil {
		return src, tree, false
	}
	if res.Tree.SourceHash != sourceHash(res.Source) {
		t.Fatalf("hash mismatch after %s", op.Kind)
	}
	reparsed, err := parser.Parse(res.Source, parser.ParseOptions{})
	if err != nil {
		t.Fatalf("state does not re-parse after %s: %v", op.Kind, err)
	}
	if !treesEqual(reparsed, res.Tree) {
		t.Fatalf("intermediate state mismatch after %s", op.Kind)
	}
	if n := roundTripFailures(res.Source, res.Tree); n != 0 {
		t.Fatalf("round-trip check reports %d failures after %s", n, op.Kind)
	}
	return res.Source, res.Tree, true
}

func TestFuzzJake(t *testing.T) {
	base, baseTree := loadJake(t)
	for _, seed := range []int64{1, 2, 3} {
		rng := rand.New(rand.NewSource(seed))
		src, tree := base, baseTree
		applied, rejected := 0, 0
		for i := 0; i < 150; i++ {
			op, ok := randomOp(rng, tree)
			if !ok {
				continue
			}
			var stepped bool
			src, tree, stepped = fuzzStep(t, src, tree, op)
			if stepped {
				applied++
			} else {
				rejected++
			}
		}
		t.Logf("seed %d: %d applied, %d rejected", seed, applied, rejected)
		if applied < 75 {
			t.Fatalf("seed %d: only %d/150 ops applied; fuzz is not exercising the engine", seed, applied)
		}
	}
}

func TestFuzzCorpus(t *testing.T) {
	files, err := filepath.Glob("../../testdata/resumes/*.tex")
	if err != nil || len(files) == 0 {
		t.Skip("no corpus")
	}
	sort.Strings(files)
	if len(files) > 10 {
		files = files[:10]
	}
	rng := rand.New(rand.NewSource(42))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		tree, err := parser.Parse(src, parser.ParseOptions{})
		if err != nil {
			continue // parser coverage is phase 02's gate, not ours
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			curSrc, curTree := src, tree
			for i := 0; i < 25; i++ {
				op, ok := randomOp(rng, curTree)
				if !ok {
					continue
				}
				curSrc, curTree, _ = fuzzStep(t, curSrc, curTree, op)
			}
		})
	}
}

func TestUndoRestoresRoundTrip(t *testing.T) {
	// A fuzz-flavored undo chain: random edits followed by their inverses
	// must reproduce the exact original source.
	src, tree := loadJake(t)
	rng := rand.New(rand.NewSource(7))
	type undo struct {
		id    string
		value string
	}
	var undos []undo
	curSrc, curTree := src, tree
	for i := 0; i < 20; i++ {
		var targets []*model.Bullet
		eachList(curTree, func(list *model.BulletList, _ *model.Entry, _ *model.Section) bool {
			for bi := range list.Bullets {
				if !list.Bullets[bi].Hidden && list.Bullets[bi].Text.Mode == "text" {
					targets = append(targets, &list.Bullets[bi])
				}
			}
			return false
		})
		if len(targets) == 0 {
			break
		}
		b := targets[rng.Intn(len(targets))]
		undos = append(undos, undo{id: b.ID, value: b.Text.Value})
		res, err := ApplyOps(curSrc, curTree, []model.Op{{Kind: "edit", ID: b.ID, Value: fmt.Sprintf("Changed %d", i)}})
		if err != nil {
			undos = undos[:len(undos)-1]
			continue
		}
		curSrc, curTree = res.Source, res.Tree
	}
	for i := len(undos) - 1; i >= 0; i-- {
		res, err := ApplyOps(curSrc, curTree, []model.Op{{Kind: "edit", ID: undos[i].id, Value: undos[i].value}})
		if err != nil {
			t.Fatalf("undo %d failed: %v", i, err)
		}
		curSrc, curTree = res.Source, res.Tree
	}
	if !treesEqual(curTree, tree) {
		t.Fatal("undoing all edits should reproduce the original tree")
	}
	if n := roundTripFailures(curSrc, curTree); n != 0 {
		t.Fatalf("undone source fails round-trip check: %d", n)
	}
}

func TestHiddenBulletSurvivesOps(t *testing.T) {
	// Ops around a hidden bullet keep it hidden with its ID.
	src, tree := loadJake(t)
	_, b := firstBullet(t, tree)
	res := applyOne(t, src, tree, model.Op{Kind: "hide", ID: b.ID})
	entry := res.Tree.Sections[1].Items[0].Entry
	l := entry.Bullets
	order := []string{l.Bullets[1].ID, l.Bullets[0].ID, l.Bullets[2].ID}
	res2 := applyOne(t, res.Source, res.Tree, model.Op{Kind: "reorder", ParentID: l.ID, Order: order})
	got := res2.Tree.Sections[1].Items[0].Entry.Bullets.Bullets
	if got[1].ID != b.ID || !got[1].Hidden {
		t.Fatal("hidden bullet lost its ID or state across reorder")
	}
	if !strings.Contains(res2.Source, "%") {
		t.Fatal("comment prefix missing")
	}
}
