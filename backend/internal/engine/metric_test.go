package engine

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

var metricFlag = flag.Bool("engine.metric", false, "print the per-file editability table")

// TestEditabilityMetric is gate 2: for every test-set file that parses,
// attempt edit, hide+show, reorder, and move on every bullet from the parsed
// base state and record the success rate. A file passes when every attempted
// op succeeds. At least 80% of parseable files must pass.
//
// Verbose table: go test ./internal/engine -run TestEditabilityMetric -v
// Standalone table: go test ./internal/engine -run TestEditabilityMetric -args -engine.metric
func TestEditabilityMetric(t *testing.T) {
	files, err := filepath.Glob("../../testdata/resumes/*.tex")
	if err != nil || len(files) == 0 {
		t.Skip("no corpus")
	}
	sort.Strings(files)
	type row struct {
		file                      string
		bullets                   int
		editOK, hideOK, reorderOK int
		editN, hideN, reorderN    int
		moveOK, moveN             int
		pass                      bool
	}
	var rows []row
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		tree, err := parser.Parse(src, parser.ParseOptions{})
		if err != nil {
			continue // unparseable files are phase 02's metric, not ours
		}
		r := row{file: filepath.Base(f), pass: true}

		// Collect bullets and lists from the base tree.
		type bulletRef struct {
			bullet *model.Bullet
			list   *model.BulletList
			entry  *model.Entry
		}
		var bullets []bulletRef
		var lists []*model.BulletList
		var entriesWithLists []*model.Entry
		eachList(tree, func(list *model.BulletList, entry *model.Entry, _ *model.Section) bool {
			lists = append(lists, list)
			if entry != nil {
				entriesWithLists = append(entriesWithLists, entry)
			}
			for i := range list.Bullets {
				bullets = append(bullets, bulletRef{bullet: &list.Bullets[i], list: list, entry: entry})
			}
			return false
		})
		try := func(op model.Op) bool {
			_, err := ApplyOps(src, tree, []model.Op{op})
			return err == nil
		}
		for _, b := range bullets {
			if b.bullet.Hidden || b.bullet.Text.Mode != "text" {
				continue // not user-editable by design (raw or already hidden)
			}
			r.bullets++
			r.editN++
			if try(model.Op{Kind: "edit", ID: b.bullet.ID, Value: b.bullet.Text.Value + " x"}) {
				r.editOK++
			}
			r.hideN++
			if res, err := ApplyOps(src, tree, []model.Op{{Kind: "hide", ID: b.bullet.ID}}); err == nil {
				if _, err := ApplyOps(res.Source, res.Tree, []model.Op{{Kind: "show", ID: b.bullet.ID}}); err == nil {
					r.hideOK++
				}
			}
			// Move: needs another entry with a bullet list.
			for _, e := range entriesWithLists {
				if b.entry != nil && e == b.entry {
					continue
				}
				if b.entry == nil {
					break // standalone list bullets: no same-parent anchor rule, try first entry target
				}
				r.moveN++
				if try(model.Op{Kind: "move", ID: b.bullet.ID, ToEntryID: e.ID}) {
					r.moveOK++
				}
				break
			}
		}
		for _, l := range lists {
			if len(l.Bullets) < 2 {
				continue
			}
			order := make([]string, len(l.Bullets))
			for i, b := range l.Bullets {
				order[len(l.Bullets)-1-i] = b.ID // reverse
			}
			r.reorderN++
			if try(model.Op{Kind: "reorder", ParentID: l.ID, Order: order}) {
				r.reorderOK++
			}
		}
		r.pass = r.editOK == r.editN && r.hideOK == r.hideN && r.reorderOK == r.reorderN && r.moveOK == r.moveN
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		t.Skip("no parseable files")
	}
	passed := 0
	for _, r := range rows {
		if r.pass {
			passed++
		}
		line := fmt.Sprintf("%-38s bullets=%3d edit=%d/%d hide+show=%d/%d reorder=%d/%d move=%d/%d pass=%v",
			r.file, r.bullets, r.editOK, r.editN, r.hideOK, r.hideN, r.reorderOK, r.reorderN, r.moveOK, r.moveN, r.pass)
		if *metricFlag {
			fmt.Println(line)
		}
		t.Log(line)
	}
	rate := float64(passed) / float64(len(rows))
	summary := fmt.Sprintf("editability: %d/%d parseable files fully editable (%.0f%%)", passed, len(rows), rate*100)
	if *metricFlag {
		fmt.Println(summary)
	}
	t.Log(summary)
	if rate < 0.8 {
		t.Fatalf("gate: editability %.0f%% < 80%%", rate*100)
	}
}
