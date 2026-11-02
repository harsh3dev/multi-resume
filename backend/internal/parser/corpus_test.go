package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/textconv"
)

// corpusRow is one line of the Phase 0 score table.
type corpusRow struct {
	File      string   `json:"-"`
	Parsed    bool     `json:"parsed"`
	Name      string   `json:"name"`
	Contacts  int      `json:"contacts"`
	Sections  int      `json:"sections"`
	Entries   int      `json:"entries"`
	Bullets   int      `json:"bullets"`
	RawFields int      `json:"rawFields"`
	ReadOnly  int      `json:"readOnly"`
	Coverage  float64  `json:"coverage"`
	Flags     []string `json:"-"`
	Err       string   `json:"-"`
}

// visitFields calls fn for every editable text in the tree (kind is for
// diagnostics only).
func visitFields(r *model.Resume, fn func(kind string, text model.Text)) {
	var visitList func(*model.BulletList)
	visitList = func(list *model.BulletList) {
		for _, b := range list.Bullets {
			fn("bullet", b.Text)
			for i := range b.Children {
				visitList(&b.Children[i])
			}
		}
	}
	if r.Header != nil {
		if r.Header.Name != nil {
			fn("name", r.Header.Name.Text())
		}
		for _, f := range r.Header.Items {
			fn("contact", f.Text())
		}
	}
	for _, s := range r.Sections {
		fn("section", s.Title.Text())
		for _, it := range s.Items {
			switch {
			case it.Entry != nil:
				for _, f := range it.Entry.Fields {
					fn("entry", f.Text())
				}
				if it.Entry.Bullets != nil {
					visitList(it.Entry.Bullets)
				}
			case it.Bullets != nil:
				visitList(it.Bullets)
			case it.Skill != nil:
				fn("skill", it.Skill.Label.Text())
				fn("skill", it.Skill.Value.Text())
			case it.Paragraph != nil:
				fn("paragraph", it.Paragraph.Text.Text())
			}
		}
	}
}

// checkSpans verifies the Phase 0 span invariant: every stored value is
// reproduced by converting the source text at its span, and spans lie inside
// the source.
func checkSpans(orig string, r *model.Resume) []string {
	src := Shadow(orig) // hidden blocks are parsed with their comment layer removed
	var problems []string
	visitFields(r, func(kind string, text model.Text) {
		if text.Span[0] < 0 || text.Span[1] < text.Span[0] || text.Span[1] > len(src) {
			problems = append(problems, fmt.Sprintf("%s: span %v out of bounds", kind, text.Span))
			return
		}
		got := textconv.ToValue(src, text.Span)
		// Macro-decoded fields (zero-arg commands) legitimately differ.
		if got.Mode == "raw" && text.Mode == "text" && strings.HasPrefix(strings.TrimSpace(src[text.Span[0]:text.Span[1]]), `\`) {
			return
		}
		if got.Value != text.Value || got.Mode != text.Mode {
			problems = append(problems, fmt.Sprintf("%s: stored %q (%s) != source %q (%s)", kind, text.Value, text.Mode, got.Value, got.Mode))
		}
	})
	prevEnd := -1
	for _, s := range r.Sections {
		if s.Span[0] < prevEnd || s.Span[1] > len(src) || s.Span[1] < s.Span[0] {
			problems = append(problems, fmt.Sprintf("section span %v overlaps/out of range", s.Span))
		}
		prevEnd = s.Span[1]
	}
	return problems
}

func measure(file string, src string) corpusRow {
	row := corpusRow{File: file}
	r, err := Parse(src, ParseOptions{})
	if err != nil {
		row.Err = err.Error()
		return row
	}
	row.Parsed = true
	row.Sections, row.Entries, row.Bullets = len(r.Sections), r.Report.Entries, r.Report.Bullets
	row.ReadOnly, row.Coverage = r.Report.ReadOnlyRegions, r.Report.Coverage
	if r.Header != nil {
		row.Contacts = len(r.Header.Items)
		if r.Header.Name != nil {
			row.Name = r.Header.Name.Value
		}
	}
	visitFields(r, func(_ string, t model.Text) {
		if t.Mode == "raw" {
			row.RawFields++
		}
	})
	// Failure grouping, keyed to the numbered problems in the plan.
	switch {
	case row.Sections == 0:
		row.Flags = append(row.Flags, "no-sections(6,7)")
	case row.Entries == 0 && row.Bullets == 0:
		row.Flags = append(row.Flags, "no-content(5,7)")
	}
	if row.Name == "" {
		row.Flags = append(row.Flags, "no-name(1,3,6)")
	}
	if row.Name != "" && (strings.Contains(row.Name, `\`) || strings.ContainsAny(row.Name, "{}")) {
		row.Flags = append(row.Flags, "dirty-name(3)")
	}
	if row.RawFields > 0 {
		row.Flags = append(row.Flags, fmt.Sprintf("raw-fields=%d(2,3,4)", row.RawFields))
	}
	if row.ReadOnly > 0 {
		row.Flags = append(row.Flags, fmt.Sprintf("read-only=%d(4)", row.ReadOnly))
	}
	return row
}

const baselinePath = "../../testdata/coverage-baseline.json"

type baselineEntry struct {
	Sections int     `json:"sections"`
	Entries  int     `json:"entries"`
	Bullets  int     `json:"bullets"`
	Coverage float64 `json:"coverage"`
}

// TestCorpus is the Phase 0 score: it parses every fixture, checks spans,
// prints the table (go test -run TestCorpus -v), compares against optional
// expected/<file>.json, and fails if any fixture drops below its recorded
// baseline. Re-record with -update after an intentional improvement.
func TestCorpus(t *testing.T) {
	files, err := filepath.Glob("../../testdata/resumes/*.tex")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("fixture corpus not present (backend/testdata is not versioned)")
	}
	sort.Strings(files)
	baseline := map[string]baselineEntry{}
	if data, err := os.ReadFile(baselinePath); err == nil {
		if err := json.Unmarshal(data, &baseline); err != nil {
			t.Fatalf("bad baseline: %v", err)
		}
	}
	current := map[string]baselineEntry{}
	var table strings.Builder
	fmt.Fprintf(&table, "\n%-34s %-6s %-22s %4s %4s %4s %4s %4s %4s %6s  %s\n", "file", "parsed", "name", "cont", "sect", "entr", "bull", "raw", "ro", "cov", "flags")
	var totalCov float64
	for _, path := range files {
		base := filepath.Base(path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		row := measure(base, src)
		name := row.Name
		if len(name) > 22 {
			name = name[:21] + "…"
		}
		name = strings.ReplaceAll(name, "\n", " ")
		fmt.Fprintf(&table, "%-34s %-6v %-22s %4d %4d %4d %4d %4d %4d %5.0f%%  %s%s\n",
			base, row.Parsed, name, row.Contacts, row.Sections, row.Entries, row.Bullets, row.RawFields, row.ReadOnly,
			row.Coverage*100, strings.Join(row.Flags, " "), row.Err)
		totalCov += row.Coverage
		if !row.Parsed {
			// A clear "not supported" error is acceptable; a panic is not.
			continue
		}
		current[base] = baselineEntry{row.Sections, row.Entries, row.Bullets, row.Coverage}
		r, _ := Parse(src, ParseOptions{})
		for _, p := range checkSpans(src, r) {
			t.Errorf("%s: span check: %s", base, p)
		}
		if want, err := os.ReadFile(filepath.Join("../../testdata/expected", base+".json")); err == nil {
			var exp struct {
				Name     *string  `json:"name"`
				Contacts *int     `json:"contacts"`
				Sections []string `json:"sections"`
				Entries  *int     `json:"entries"`
				Bullets  *int     `json:"bullets"`
			}
			if err := json.Unmarshal(want, &exp); err != nil {
				t.Fatalf("%s: bad expected json: %v", base, err)
			}
			if exp.Name != nil && *exp.Name != row.Name {
				t.Errorf("%s: name %q, want %q", base, row.Name, *exp.Name)
			}
			if exp.Contacts != nil && *exp.Contacts != row.Contacts {
				t.Errorf("%s: contacts %d, want %d", base, row.Contacts, *exp.Contacts)
			}
			if exp.Entries != nil && *exp.Entries != row.Entries {
				t.Errorf("%s: entries %d, want %d", base, row.Entries, *exp.Entries)
			}
			if exp.Bullets != nil && *exp.Bullets != row.Bullets {
				t.Errorf("%s: bullets %d, want %d", base, row.Bullets, *exp.Bullets)
			}
			if exp.Sections != nil {
				var got []string
				for _, s := range r.Sections {
					got = append(got, s.Title.Value)
				}
				if strings.Join(got, "|") != strings.Join(exp.Sections, "|") {
					t.Errorf("%s: sections %q, want %q", base, got, exp.Sections)
				}
			}
		}
	}
	fmt.Fprintf(&table, "mean coverage %.1f%% over %d files\n", totalCov/float64(len(files))*100, len(files))
	t.Log(table.String())
	if *update {
		data, _ := json.MarshalIndent(current, "", "  ")
		if err := os.WriteFile(baselinePath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	for file, was := range baseline {
		now, ok := current[file]
		if !ok {
			t.Errorf("%s: parsed before, now fails", file)
			continue
		}
		// Phase 6: a fixture may not quietly lose structure or coverage.
		if now.Entries < was.Entries || now.Bullets < was.Bullets || now.Sections < was.Sections || now.Coverage < was.Coverage-0.005 {
			t.Errorf("%s regressed: %+v -> %+v", file, was, now)
		}
	}
}
