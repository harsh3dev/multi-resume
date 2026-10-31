package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/engine"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
	"github.com/harsh3dev/multi-resume/backend/internal/store"
)

// seedDraft uploads jake.tex through the real handler and returns its draft id.
func seedDraft(t *testing.T, h http.Handler) (draftID string, revision int, tree model.Resume) {
	t.Helper()
	tex, err := os.ReadFile("../../testdata/resumes/jake.tex")
	if err != nil {
		t.Skip("fixture missing")
	}
	body, ct := uploadBody(t, "resume.tex", string(tex))
	req := httptest.NewRequest(http.MethodPost, "/api/resumes", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed upload: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		DraftID string       `json:"draftId"`
		Tree    model.Resume `json:"tree"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.DraftID, 0, resp.Tree
}

func postOps(t *testing.T, h http.Handler, draftID string, revision int, ops []model.Op) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"revision": revision, "ops": ops})
	req := httptest.NewRequest(http.MethodPost, "/api/drafts/"+draftID+"/ops", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func firstBulletID(t *testing.T, tree model.Resume) string {
	t.Helper()
	for _, s := range tree.Sections {
		for _, it := range s.Items {
			if it.Entry != nil && it.Entry.Bullets != nil {
				for _, b := range it.Entry.Bullets.Bullets {
					if !b.Hidden && b.Text.Mode == "text" {
						return b.ID
					}
				}
			}
		}
	}
	t.Fatal("no bullet")
	return ""
}

func TestOpsHappyPath(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	draftID, rev, tree := seedDraft(t, h)
	bid := firstBulletID(t, tree)
	rec := postOps(t, h, draftID, rev, []model.Op{{Kind: "edit", ID: bid, Value: "Updated achievement text"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("ops: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Revision int          `json:"revision"`
		Tree     model.Resume `json:"tree"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Revision != 1 {
		t.Fatalf("revision: %d", resp.Revision)
	}
}

func TestOpsStaleRevision409(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	draftID, _, tree := seedDraft(t, h)
	bid := firstBulletID(t, tree)
	rec := postOps(t, h, draftID, 99, []model.Op{{Kind: "edit", ID: bid, Value: "x"}})
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rec.Code)
	}
}

func TestOpsRejected422(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	draftID, rev, _ := seedDraft(t, h)
	rec := postOps(t, h, draftID, rev, []model.Op{{Kind: "edit", ID: "b_nonexistent", Value: "x"}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", rec.Code)
	}
}

func TestOpsRejectsUnknownFields(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	draftID, rev, tree := seedDraft(t, h)
	bid := firstBulletID(t, tree)
	payload := []byte(`{"revision":0,"ops":[{"op":"edit","id":"` + bid + `","value":"x","evil":true}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/drafts/"+draftID+"/ops", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	_ = rev
}

func TestOpsEngineIntegration(t *testing.T) {
	// Direct engine check the API depends on: edit changes source + tree.
	tex, err := os.ReadFile("../../testdata/resumes/jake.tex")
	if err != nil {
		t.Skip("fixture missing")
	}
	src := string(tex)
	tree, err := parser.Parse(src, parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var bid string
	for _, s := range tree.Sections {
		for _, it := range s.Items {
			if it.Entry != nil && it.Entry.Bullets != nil && len(it.Entry.Bullets.Bullets) > 0 {
				bid = it.Entry.Bullets.Bullets[0].ID
			}
		}
	}
	res, err := engine.ApplyOps(src, tree, []model.Op{{Kind: "edit", ID: bid, Value: "New text here"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source == src {
		t.Fatal("source unchanged")
	}
	var _ = store.Draft{}
}
