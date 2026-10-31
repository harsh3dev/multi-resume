package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
	"time"
)

type fakeStreamer struct {
	lines []string
}

func (f fakeStreamer) StreamText(_ context.Context, _, _ string, onLine func(string)) error {
	for _, l := range f.lines {
		onLine(l)
	}
	return nil
}

func TestTailorStreamsAndPersists(t *testing.T) {
	fs := newFakeStore()
	tex := `\documentclass{article}
\begin{document}
\section{Experience}
\begin{itemize}
\item Reduced latency by 34\% using Redis
\end{itemize}
\end{document}`
	// Seed a draft owned by userA.
	resumeID := newResumeForUser(t, fs)
	draftID := uuid.New()
	draft := draftFixture(draftID, resumeID)
	draft.SourceTex = tex
	fs.drafts[draftID] = draft

	// Find the bullet id from a real parse.
	tree, err := parser.Parse(tex, parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var bulletID string
	for _, s := range tree.Sections {
		for _, it := range s.Items {
			if it.Bullets != nil && len(it.Bullets.Bullets) > 0 {
				bulletID = it.Bullets.Bullets[0].ID
			}
		}
	}
	treeJSON, _ := json.Marshal(tree)
	draft.Tree = treeJSON
	fs.drafts[draftID] = draft

	d := Deps{Store: fs, Tailor: fakeStreamer{lines: []string{
		`{"op":"edit","id":"` + bulletID + `","value":"Cut latency by 34% with Redis caching","why":"tighter"}`,
	}}}
	h := testServerWithDeps(d)
	payload, _ := json.Marshal(map[string]string{"jd": "Backend engineer role requiring Redis and Go experience. "})
	req := httptest.NewRequest(http.MethodPost, "/api/drafts/"+draftID.String()+"/tailor", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tailor: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"done":true`) {
		t.Fatal("no terminal snapshot")
	}
	if !strings.Contains(body, bulletID) {
		t.Fatal("change not streamed")
	}
	// run persisted
	var runID uuid.UUID
	for id := range fs.runs {
		runID = id
	}
	run := fs.runs[runID]
	if run.Status != "ready" || len(run.Changes) == 0 {
		t.Fatalf("run not finished properly: %+v", run)
	}
	// decisions write-back
	decPayload := []byte(`{"decisions":[{"index":0,"decision":"accepted"}]}`)
	req = httptest.NewRequest(http.MethodPost, "/api/tailor-runs/"+runID.String()+"/decisions", bytes.NewReader(decPayload))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	testServerWithDeps(d).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("decisions: %d", rec.Code)
	}
	if !strings.Contains(string(fs.runs[runID].Changes), `"decision":"accepted"`) {
		t.Fatalf("decision not persisted: %s", fs.runs[runID].Changes)
	}
}

func TestTailorRateLimited(t *testing.T) {
	fs := newFakeStore()
	resumeID := newResumeForUser(t, fs)
	draftID := uuid.New()
	fs.drafts[draftID] = draftFixture(draftID, resumeID)
	d := Deps{Store: fs, Tailor: fakeStreamer{}}
	h := testServerWithDeps(d)
	// Exhaust the limiter: 10/hour per user.
	limiter := newRateLimiter(2, time.Hour)
	_ = limiter
	payload, _ := json.Marshal(map[string]string{"jd": "x x x x x x x x x x x x x x x x x x x x"})
	// fill the shared limiter up-front by direct calls
	for i := 0; i < 10; i++ {
		tailorLimit.allow(testUserA)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/drafts/"+draftID.String()+"/tailor", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", rec.Code)
	}
}
