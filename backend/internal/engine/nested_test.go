package engine

import (
	"strings"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

const nestedSource = `\documentclass{article}
\begin{document}
\section{Experience}
\begin{itemize}
\item Parent one
\begin{itemize}
\item Child one
\item Child two
\end{itemize}
\item Parent two
\end{itemize}
\end{document}`

func nestedTree(t *testing.T) *model.Resume {
	t.Helper()
	tree, err := parser.Parse(nestedSource, parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestNestedBulletOperations(t *testing.T) {
	for _, kind := range []string{"edit", "hide", "reorder", "add", "delete"} {
		t.Run(kind, func(t *testing.T) {
			tree := nestedTree(t)
			child := &tree.Sections[0].Items[0].Bullets.Bullets[0].Children[0]
			id := child.Bullets[0].ID
			var op model.Op
			switch kind {
			case "edit":
				op = model.Op{Kind: "edit", ID: id, Value: "Updated child"}
			case "hide":
				op = model.Op{Kind: "hide", ID: id}
			case "reorder":
				op = model.Op{Kind: "reorder", ParentID: child.ID, Order: []string{child.Bullets[1].ID, id}}
			case "add":
				op = model.Op{Kind: "add", ListID: child.ID, After: &id, Value: "New child"}
			case "delete":
				op = model.Op{Kind: "delete", ID: id}
			}
			res := applyOne(t, nestedSource, tree, op)
			if strings.Contains(res.Source, "Updated child") != (kind == "edit") {
				t.Fatalf("unexpected edit result: %s", res.Source)
			}
			if kind == "hide" {
				if !res.Tree.Sections[0].Items[0].Bullets.Bullets[0].Children[0].Bullets[0].Hidden {
					t.Fatal("child did not become hidden")
				}
				back := applyOne(t, res.Source, res.Tree, model.Op{Kind: "show", ID: id})
				if back.Source != nestedSource {
					t.Fatal("hide/show changed original bytes")
				}
			}
		})
	}
}

func TestNestedLastVisibleBulletGuard(t *testing.T) {
	tree := nestedTree(t)
	child := &tree.Sections[0].Items[0].Bullets.Bullets[0].Children[0]
	first := child.Bullets[0].ID
	res := applyOne(t, nestedSource, tree, model.Op{Kind: "hide", ID: first})
	remaining := res.Tree.Sections[0].Items[0].Bullets.Bullets[0].Children[0].Bullets[1].ID
	for _, kind := range []string{"hide", "delete"} {
		if _, err := ApplyOps(res.Source, res.Tree, []model.Op{{Kind: kind, ID: remaining}}); err == nil || !strings.Contains(err.Error(), "last visible bullet") {
			t.Fatalf("%s should reject last visible nested bullet: %v", kind, err)
		}
	}
}

func TestNestedParentOperations(t *testing.T) {
	for _, kind := range []string{"hide", "reorder", "delete", "add"} {
		t.Run(kind, func(t *testing.T) {
			tree := nestedTree(t)
			list := tree.Sections[0].Items[0].Bullets
			parent := list.Bullets[0]
			var op model.Op
			switch kind {
			case "hide":
				op = model.Op{Kind: "hide", ID: parent.ID}
			case "reorder":
				op = model.Op{Kind: "reorder", ParentID: list.ID, Order: []string{list.Bullets[1].ID, parent.ID}}
			case "delete":
				op = model.Op{Kind: "delete", ID: parent.ID}
			case "add":
				op = model.Op{Kind: "add", ListID: list.ID, After: &parent.ID, Value: "Fresh parent"}
			}
			res := applyOne(t, nestedSource, tree, op)
			if kind == "hide" {
				child := res.Tree.Sections[0].Items[0].Bullets.Bullets[0].Children[0]
				if !child.Hidden || !child.Bullets[0].Hidden {
					t.Fatal("nested list must be hidden with parent")
				}
				back := applyOne(t, res.Source, res.Tree, model.Op{Kind: "show", ID: parent.ID})
				if back.Source != nestedSource {
					t.Fatal("parent hide/show changed bytes")
				}
			}
			if kind == "add" && len(res.Tree.Sections[0].Items[0].Bullets.Bullets[1].Children) != 0 {
				t.Fatal("new bullet duplicated nested lists")
			}
		})
	}
}

func TestNestedListHideShowAndGuard(t *testing.T) {
	tree := nestedTree(t)
	child := &tree.Sections[0].Items[0].Bullets.Bullets[0].Children[0]
	hidden := applyOne(t, nestedSource, tree, model.Op{Kind: "hide", ID: child.ID})
	got := &hidden.Tree.Sections[0].Items[0].Bullets.Bullets[0].Children[0]
	if !got.Hidden || !got.Bullets[0].Hidden || !got.Bullets[1].Hidden {
		t.Fatal("hiding a child list must hide its bullets")
	}
	if _, err := ApplyOps(hidden.Source, hidden.Tree, []model.Op{{Kind: "add", ListID: child.ID, Value: "New"}}); err == nil {
		t.Fatal("adding to a hidden list must be rejected")
	}
	back := applyOne(t, hidden.Source, hidden.Tree, model.Op{Kind: "show", ID: child.ID})
	if back.Source != nestedSource {
		t.Fatal("child-list hide/show changed bytes")
	}
}

func TestNestedReorderKeepsStableIDs(t *testing.T) {
	tree := nestedTree(t)
	parent := tree.Sections[0].Items[0].Bullets.Bullets[0]
	child := parent.Children[0]
	result := applyOne(t, nestedSource, tree, model.Op{Kind: "reorder", ParentID: child.ID, Order: []string{child.Bullets[1].ID, child.Bullets[0].ID}})
	got := result.Tree.Sections[0].Items[0].Bullets.Bullets[0].Children[0]
	if got.ID != child.ID || got.Bullets[0].ID != child.Bullets[1].ID || got.Bullets[1].ID != child.Bullets[0].ID {
		t.Fatal("nested IDs changed during reorder")
	}
	second := applyOne(t, result.Source, result.Tree, model.Op{Kind: "edit", ID: child.Bullets[0].ID, Value: "Edit after reorder"})
	if !strings.Contains(second.Source, "Edit after reorder") {
		t.Fatal("child cannot be edited by the same ID after reorder")
	}
}
