package parser

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

var update = flag.Bool("update", false, "regenerate golden files from current parser output")

var idPattern = regexp.MustCompile(`"id": "[a-z]+_[0-9a-f]+"`)

// goldenJSON marshals the full tree with random IDs replaced by stable,
// sequential placeholders so the output is byte-stable across runs.
func goldenJSON(t *testing.T, r *model.Resume) []byte {
	t.Helper()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	counters := map[string]int{}
	normalized := idPattern.ReplaceAllStringFunc(string(data), func(match string) string {
		prefix := regexp.MustCompile(`[a-z]+`).FindString(match[len(`"id": "`):])
		counters[prefix]++
		return fmt.Sprintf(`"id": "%s_%03d"`, prefix, counters[prefix])
	})
	return []byte(normalized + "\n")
}

// Golden files pin the full parsed tree (IDs normalized). Regenerate with:
//
//	go test ./internal/parser/ -run TestGolden -update
//
// and review the diff by eye — wrong goldens are worse than none.
func TestGolden(t *testing.T) {
	files, err := filepath.Glob("../../testdata/resumes/*.tex")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("fixture corpus not present (backend/testdata is not versioned)")
	}
	if len(files) < 50 {
		t.Fatalf("corpus gate: %d/50 distinct files", len(files))
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := Parse(string(source), ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			data := goldenJSON(t, r)
			golden := filepath.Join("../../testdata/golden", filepath.Base(path)+".json")
			if *update {
				if err := os.WriteFile(golden, data, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("missing golden (run with -update): %v", err)
			}
			if string(want) != string(data) {
				t.Fatalf("golden mismatch for %s (review diff, regenerate with -update)", path)
			}
		})
	}
}
