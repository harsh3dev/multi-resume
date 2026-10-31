package tailor

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

func testTree() *model.Resume {
	return &model.Resume{
		Sections: []model.Section{{
			ID:    "s_1",
			Title: model.Field{ID: "f_title", Value: "Experience"},
			Items: []model.SectionItem{{
				Entry: &model.Entry{
					Kind: "entry", ID: "e_1",
					Fields: []model.Field{{ID: "f_company", Value: "Acme"}},
					Bullets: &model.BulletList{
						Kind: "bullets", ID: "l_1", BulletCommand: `\item`,
						Bullets: []model.Bullet{
							{ID: "b_1", Text: model.Text{Value: "Reduced latency by 34% using Redis"}},
							{ID: "b_2", Hidden: true, Text: model.Text{Value: "Wrote internal docs"}},
						},
					},
				},
			}},
		}},
	}
}

func TestBuildAIView(t *testing.T) {
	view := BuildAIView(testTree())
	var bullet, company *AIItem
	for i := range view.Items {
		switch view.Items[i].ID {
		case "b_1":
			bullet = &view.Items[i]
		case "f_company":
			company = &view.Items[i]
		}
	}
	if bullet == nil || bullet.ReadOnly {
		t.Fatal("bullet must be editable")
	}
	if company == nil || !company.ReadOnly {
		t.Fatal("entry field must be read-only")
	}
	if bullet.MaxLen != len(bullet.Text)+len(bullet.Text)/10 {
		t.Fatalf("maxLen: %d", bullet.MaxLen)
	}
	// Hidden bullet included (bullet bank).
	found := false
	for _, it := range view.Items {
		if it.ID == "b_2" && it.Hidden {
			found = true
		}
	}
	if !found {
		t.Fatal("hidden bullet missing from AI view")
	}
	// Payload size sanity: well under 2k tokens for a small tree.
	view2 := BuildAIView(testTree())
	data, _ := json.Marshal(view2)
	if len(data)/4 > 2000 {
		t.Fatalf("payload too big: %d est tokens", len(data)/4)
	}
}

func TestRunStreamsValidatedChanges(t *testing.T) {
	deps := Deps{
		Corpus: "Reduced latency by 34% using Redis Wrote internal docs",
		Now:    time.Now,
		StreamText: func(_ context.Context, _, _ string, onLine func(string)) error {
			onLine(`{"op":"edit","id":"b_1","value":"Cut latency by 34% with Redis caching","why":"tighter"}`)
			onLine(`{"op":"edit","id":"f_company","value":"Evil Corp","why":"attack"}`) // locked → dropped
			onLine(`not json`)                                                          // malformed → dropped
			onLine(`{"op":"show","id":"b_2","why":"relevant"}`)
			onLine(`{"op":"delete","id":"b_1"}`) // disallowed op → dropped
			return nil
		},
	}
	var buf bytes.Buffer
	changes, err := Run(context.Background(), &buf, testTree(), "backend engineer JD", "run_1", deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("want 2 changes, got %d: %+v", len(changes), changes)
	}
	out := buf.String()
	if !strings.Contains(out, `"done":true`) {
		t.Fatal("no terminal tail")
	}
	if strings.Contains(out, "f_company") {
		t.Fatal("locked field edit leaked into stream")
	}
	// The whole stream must be ONE valid JSON document (useObject accumulates
	// the text and partial-parses it; repeated snapshots break that).
	var doc struct {
		Changes []Change `json:"changes"`
		Done    bool     `json:"done"`
		RunID   string   `json:"runId"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stream is not one JSON document: %v\n%s", err, out)
	}
	if len(doc.Changes) != 2 || !doc.Done || doc.RunID != "run_1" {
		t.Fatalf("bad document: %+v", doc)
	}
	if strings.Count(out, `"changes":[`) != 1 {
		t.Fatal("stream must open the changes array exactly once")
	}
}

func TestRunFlagsNumbers(t *testing.T) {
	deps := Deps{
		Corpus: "Reduced latency by 34% using Redis",
		Now:    time.Now,
		StreamText: func(_ context.Context, _, _ string, onLine func(string)) error {
			onLine(`{"op":"edit","id":"b_1","value":"Improved latency by 99%","why":"x"}`)
			return nil
		},
	}
	var buf bytes.Buffer
	changes, _ := Run(context.Background(), &buf, testTree(), "jd", "", deps)
	if len(changes) != 1 || len(changes[0].Flags) == 0 || changes[0].Flags[0] != "number_changed" {
		t.Fatalf("invented number not flagged: %+v", changes)
	}
}

func TestRunModelError(t *testing.T) {
	deps := Deps{
		Now: time.Now,
		StreamText: func(context.Context, string, string, func(string)) error {
			return context.DeadlineExceeded
		},
	}
	var buf bytes.Buffer
	_, err := Run(context.Background(), &buf, testTree(), "jd", "", deps)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(buf.String(), `"error":true`) {
		t.Fatal("error snapshot missing")
	}
}
