package checks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/parser"
)

func TestJakeRoundTrip(t *testing.T) {
	bytes, err := os.ReadFile("../../testdata/resumes/jake.tex")
	if err != nil {
		t.Fatal(err)
	}
	r, err := parser.Parse(string(bytes), parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if failed := CheckRoundTrip(string(bytes), r); len(failed) != 0 {
		t.Fatalf("fields did not preserve source: %v", failed)
	}
}

func TestAllFixturesRoundTrip(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/resumes/*.tex")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("missing resume fixtures")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			bytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := parser.Parse(string(bytes), parser.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if failed := CheckRoundTrip(string(bytes), r); len(failed) > 0 {
				t.Fatalf("nonidentical spans: %v", failed)
			}
			t.Logf("sections=%d entries=%d bullets=%d readOnly=%d coverage=%.3f", r.Report.Sections, r.Report.Entries, r.Report.Bullets, r.Report.ReadOnlyRegions, r.Report.Coverage)
		})
	}
}
