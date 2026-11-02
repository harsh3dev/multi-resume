//go:build docker

// Phase 6.1: edits must not break compilation. Run inside the backend image
// (needs latexmk, bubblewrap, prlimit, poppler-utils):
//
//	go test -tags docker ./internal/engine/ -run TestIntegrationEditsStillCompile -v
package engine

import (
	"os"
	"os/exec"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

func TestIntegrationEditsStillCompile(t *testing.T) {
	for _, tool := range []string{"latexmk", "bwrap", "prlimit", "pdfinfo", "pdftotext"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available; run inside the backend image", tool)
		}
	}
	if _, err := os.Stat("../../testdata/resumes/jake.tex"); err != nil {
		t.Skip("local fixture corpus not present; backend/testdata is not versioned")
	}
	c, err := compile.New(compile.Options{
		Cache: compile.NewMemoryCache(), Store: compile.NewMemoryStore(),
		Runner: compile.BwrapRunner{}, PDFTools: compile.PopplerTools{},
	})
	if err != nil {
		t.Fatal(err)
	}
	compiles := func(label, src string) {
		t.Helper()
		if _, err := c.Compile(t.Context(), src, nil, "pdflatex"); err != nil {
			t.Errorf("%s: no longer compiles: %v", label, err)
		}
	}
	for _, name := range []string{"jake.tex", "jakefork-rdey0.tex", "jakefork-petersoapes.tex"} {
		data, err := os.ReadFile("../../testdata/resumes/" + name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		tree, err := parser.Parse(src, parser.ParseOptions{})
		if err != nil {
			t.Fatal(err)
		}
		compiles(name+" (unedited)", src)
		for _, id := range hideable(tree) {
			res, err := ApplyOps(src, tree, []model.Op{{Kind: "hide", ID: id}})
			if err != nil {
				continue // rejected ops never change the source
			}
			compiles(name+" hide "+id, res.Source)
			back, err := ApplyOps(res.Source, res.Tree, []model.Op{{Kind: "show", ID: id}})
			if err != nil || back.Source != src {
				t.Errorf("%s: show %s did not restore the source", name, id)
			}
		}
		for _, op := range reverseOps(tree) {
			if res, err := ApplyOps(src, tree, []model.Op{op}); err == nil {
				compiles(name+" reorder "+op.ParentID, res.Source)
			}
		}
		for id, text := range editable(tree) {
			if res, err := ApplyOps(src, tree, []model.Op{{Kind: "edit", ID: id, Value: text + " x"}}); err == nil {
				compiles(name+" edit "+id, res.Source)
			}
		}
	}
}
