package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

// TestEditMatrixAllFixtures (Phase 5.3): on every fixture, every hideable
// node is hidden and shown again, every parent is reordered, and every text
// field is edited — each through ApplyOps, which re-parses and compares the
// tree. Hide followed by show must give back the original bytes.
func TestEditMatrixAllFixtures(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/resumes/*.tex")
	if len(files) == 0 {
		t.Skip("no corpus")
	}
	sort.Strings(files)
	var failures []string
	var hides, reorders, edits, moves int
	for _, f := range files {
		data, _ := os.ReadFile(f)
		src := string(data)
		tree, err := parser.Parse(src, parser.ParseOptions{})
		if err != nil {
			continue
		}
		name := filepath.Base(f)
		var skippedRawNested int
		fail := func(what string, err error) {
			failures = append(failures, fmt.Sprintf("%s: %s: %v", name, what, err))
		}
		for _, id := range hideable(tree) {
			// Lists embedded inside a raw entry argument are visible in the
			// parsed tree but cannot be safely extracted as source blocks. The
			// parent list's known rejections remain within their original budget.
			if b := findBullet(tree, id); b != nil && nestedInRawArgument(tree, b.list) {
				skippedRawNested++
				continue
			}
			if l := findList(tree, id); l != nil && nestedInRawArgument(tree, l.list) {
				skippedRawNested++
				continue
			}
			hides++
			res, err := ApplyOps(src, tree, []model.Op{{Kind: "hide", ID: id}})
			if err != nil {
				if strings.Contains(err.Error(), "whole list") || strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "shares its line") {
					continue // guarded by design: last bullet, already hidden, or sharing a line with other content (not hideable as a block)
				}
				fail("hide "+id, err)
				continue
			}
			back, err := ApplyOps(res.Source, res.Tree, []model.Op{{Kind: "show", ID: id}})
			if err != nil {
				fail("show "+id, err)
				continue
			}
			if back.Source != src {
				fail("hide+show "+id, fmt.Errorf("bytes differ"))
			}
		}
		for _, op := range reverseOps(tree) {
			if l := findList(tree, op.ParentID); l != nil && nestedInRawArgument(tree, l.list) {
				skippedRawNested++
				continue
			}
			reorders++
			if _, err := ApplyOps(src, tree, []model.Op{op}); err != nil {
				fail("reorder "+op.ParentID, err)
			}
		}
		for id, text := range editable(tree) {
			edits++
			if _, err := ApplyOps(src, tree, []model.Op{{Kind: "edit", ID: id, Value: text + " x"}}); err != nil {
				fail("edit "+id, err)
			}
		}
		_ = moves
		if skippedRawNested > 0 {
			t.Logf("%s: %d operations inside raw macro arguments not safely block-editable", name, skippedRawNested)
		}
	}
	t.Logf("matrix: %d hide+show, %d reorder, %d edit attempted; %d rejected", hides, reorders, edits, len(failures))
	// Every rejection leaves the source untouched (ApplyOps verifies by
	// re-parsing), so a rejection is a limitation, not corruption. Limits are
	// budgeted per file: Jake-style files must have none; the others may not
	// get worse than recorded here.
	counts := map[string]int{}
	for _, f := range failures {
		counts[strings.SplitN(f, ":", 2)[0]]++
	}
	for file, n := range counts {
		if n > knownRejections[file] {
			t.Errorf("%s: %d rejected operations, budget %d; e.g.:", file, n, knownRejections[file])
			for _, f := range failures {
				if strings.HasPrefix(f, file+":") {
					t.Error("   ", f)
					break
				}
			}
		}
	}
}

func nestedInRawArgument(tree *model.Resume, list *model.BulletList) bool {
	if list == nil {
		return false
	}
	withinRaw := false
	for _, section := range tree.Sections {
		for _, item := range section.Items {
			if item.Entry == nil {
				continue
			}
			for _, field := range item.Entry.Fields {
				if field.Mode == "raw" && field.Span[0] <= list.Span[0] && list.Span[1] <= field.Span[1] {
					withinRaw = true
				}
			}
		}
	}
	if !withinRaw {
		return false
	}
	nested := false
	eachList(tree, func(other *model.BulletList, _ *model.Entry, _ *model.Section) bool {
		if other != list && other.Span[0] <= list.Span[0] && list.Span[1] <= other.Span[1] {
			nested = true
		}
		return false
	})
	return nested
}

// knownRejections is the per-file budget of operations the engine rejects
// (never corrupts). Reasons: bullets that live inside a raw macro argument
// (moderncv \cventry), sections split across minipage columns (Deedy), and
// templates whose entries and "items" share one span (sb2nov).
var knownRejections = map[string]int{
	"moderncv-template.tex":    5,
	"moderncv-template-es.tex": 5,
	"sb2nov.tex":               3,
	"deedy-openfont.tex":       1,
	"deedy-macfonts.tex":       1,
}

func hideable(tree *model.Resume) []string {
	var ids []string
	for _, s := range tree.Sections {
		if !s.Hidden {
			ids = append(ids, s.ID)
		}
		for _, it := range s.Items {
			switch {
			case it.Entry != nil:
				if !it.Entry.Hidden {
					ids = append(ids, it.Entry.ID)
				}
			case it.Skill != nil:
				if !it.Skill.Hidden {
					ids = append(ids, it.Skill.ID)
				}
			case it.Paragraph != nil:
				if !it.Paragraph.Hidden {
					ids = append(ids, it.Paragraph.ID)
				}
			}
		}
	}
	eachList(tree, func(l *model.BulletList, _ *model.Entry, _ *model.Section) bool {
		if !l.Hidden {
			ids = append(ids, l.ID)
		}
		for _, b := range l.Bullets {
			if !b.Hidden {
				ids = append(ids, b.ID)
			}
		}
		return false
	})
	return ids
}

func reverseOps(tree *model.Resume) []model.Op {
	var ops []model.Op
	rev := func(parent string, ids []string) {
		if len(ids) < 2 {
			return
		}
		order := make([]string, len(ids))
		for i, id := range ids {
			order[len(ids)-1-i] = id
		}
		ops = append(ops, model.Op{Kind: "reorder", ParentID: parent, Order: order})
	}
	var secs []string
	for _, s := range tree.Sections {
		secs = append(secs, s.ID)
		var items []string
		for _, it := range s.Items {
			if it.Opaque != nil {
				items = nil
				break // an unsupported block pins the order of its section
			}
			items = append(items, itemID(it))
		}
		rev(s.ID, items)
	}
	rev(RootParentID, secs)
	eachList(tree, func(l *model.BulletList, _ *model.Entry, _ *model.Section) bool {
		var ids []string
		for _, b := range l.Bullets {
			ids = append(ids, b.ID)
		}
		rev(l.ID, ids)
		return false
	})
	return ops
}

func editable(tree *model.Resume) map[string]string {
	out := map[string]string{}
	walkTextRefs(tree, func(r *textRef) bool {
		if r.text.Mode == "text" && !strings.ContainsAny(r.text.Value, "⟦") {
			out[r.id] = r.text.Value
		}
		return false
	})
	return out
}
