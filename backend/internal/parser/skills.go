package parser

import (
	"sort"
	"strings"

	"github.com/harsh3dev/multi-resume/backend/internal/latex"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// tabularSkills turns the two-column rows of a tabular inside a skills
// section ("Languages & Go, Rust \\") into skill lines. It only runs when the
// section produced no other items, so it never competes with another reading.
func tabularSkills(src string, section *model.Section, macros macroTexts, opt latex.Options) {
	if !skillishTitle(section.Title.Value) || len(section.Items) > 0 {
		return
	}
	from := section.Title.Span[1]
	if from >= section.Span[1] {
		return
	}
	var items []model.SectionItem
	var visit func(nodes []latex.Node)
	visit = func(nodes []latex.Node) {
		for _, n := range nodes {
			switch {
			case n.Kind == latex.KEnv && strings.HasPrefix(n.Name, "tabular") || n.Kind == latex.KEnv && n.Name == "longtable":
				items = append(items, skillRows(src, n.Body, macros)...)
			case n.Kind == latex.KGroup || n.Kind == latex.KEnv:
				visit(n.Body)
			}
		}
	}
	visit(latex.Parse(src, from, section.Span[1], opt))
	section.Items = append(section.Items, items...)
	sort.SliceStable(section.Items, func(i, j int) bool { return itemStart(section.Items[i]) < itemStart(section.Items[j]) })
}

func itemStart(it model.SectionItem) int {
	switch {
	case it.Skill != nil:
		return it.Skill.Span[0]
	case it.Entry != nil:
		return it.Entry.Span[0]
	}
	return 0
}

func skillRows(src string, body []latex.Node, macros macroTexts) []model.SectionItem {
	var items []model.SectionItem
	var row []latex.Node
	flush := func() {
		defer func() { row = nil }()
		cells := splitCells(src, row)
		if len(cells) != 2 {
			return
		}
		ls, le, ok1 := narrow(src, cells[0])
		vs, ve, ok2 := narrow(src, cells[1])
		if !ok1 || !ok2 || !hasVisibleText(src[ls:le]) || !hasVisibleText(src[vs:ve]) {
			return
		}
		items = append(items, model.SectionItem{Skill: &model.SkillLine{
			Kind: "skill", ID: newID("sk"), Span: model.Span{ls, ve},
			Label: macros.field(src, model.Span{ls, le}, ""), Value: macros.field(src, model.Span{vs, ve}, ""),
		}})
	}
	for _, n := range body {
		if n.Kind == latex.KCommand && n.Name == `\\` {
			flush()
			continue
		}
		row = append(row, n)
	}
	flush()
	return items
}

// splitCells splits a row at top-level & characters.
func splitCells(src string, row []latex.Node) [][]latex.Node {
	cells := [][]latex.Node{{}}
	for _, n := range row {
		if n.Kind == latex.KText && strings.Contains(src[n.Start:n.End], "&") {
			pos := n.Start
			for i := n.Start; i < n.End; i++ {
				if src[i] == '&' {
					if i > pos {
						cells[len(cells)-1] = append(cells[len(cells)-1], latex.Node{Kind: latex.KText, Start: pos, End: i})
					}
					cells = append(cells, []latex.Node{})
					pos = i + 1
				}
			}
			if pos < n.End {
				cells[len(cells)-1] = append(cells[len(cells)-1], latex.Node{Kind: latex.KText, Start: pos, End: n.End})
			}
			continue
		}
		cells[len(cells)-1] = append(cells[len(cells)-1], n)
	}
	return cells
}
