package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/harsh3dev/multi-resume/backend/internal/store"
)

// seedResumeWithDefaultVariation creates a resume, its "Original" variation
// and a draft, bypassing the upload flow.
func seedResumeWithDefaultVariation(t *testing.T, fs *fakeStore) (resumeID, variationID, draftID uuid.UUID) {
	t.Helper()
	resumeID = newResumeForUser(t, fs)
	v, err := fs.InsertVariation(context.Background(), store.InsertVariationParams{
		ResumeID: resumeID, Name: "Original", IsDefault: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	d := draftFixture(uuid.New(), resumeID)
	d.VariationID = v.ID
	fs.drafts[d.ID] = d
	return resumeID, v.ID, d.ID
}

func TestUploadCreatesOriginalVariation(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	tex, err := os.ReadFile("../../testdata/resumes/jake.tex")
	if err != nil {
		t.Skip("fixture missing")
	}

	body, contentType := uploadBody(t, "resume.tex", string(tex))
	req := httptest.NewRequest(http.MethodPost, "/api/resumes", body)
	req.Header.Set("Content-Type", contentType)
	rec := serve(h, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ResumeID    string `json:"resumeId"`
		VariationID string `json:"variationId"`
		DraftID     string `json:"draftId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.VariationID == "" || created.DraftID == "" {
		t.Fatalf("upload response missing variationId/draftId: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resumes/"+created.ResumeID+"/variations", nil)
	rec = serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list variations: %d", rec.Code)
	}
	var list struct {
		Variations []struct {
			ID        string  `json:"id"`
			Name      string  `json:"name"`
			IsDefault bool    `json:"isDefault"`
			DraftID   *string `json:"draftId"`
		} `json:"variations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Variations) != 1 {
		t.Fatalf("want 1 variation, got %d", len(list.Variations))
	}
	v := list.Variations[0]
	if v.Name != "Original" || !v.IsDefault || v.ID != created.VariationID {
		t.Fatalf("unexpected default variation: %+v", v)
	}
	if v.DraftID == nil || *v.DraftID != created.DraftID {
		t.Fatalf("variation not linked to draft: %+v", v)
	}
}

func TestCreateVariationClonesDraft(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	resumeID, _, _ := seedResumeWithDefaultVariation(t, fs)

	body, _ := json.Marshal(map[string]string{"name": "Acme"})
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/"+resumeID.String()+"/variations", bytes.NewReader(body))
	rec := serve(h, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create variation: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		VariationID string `json:"variationId"`
		DraftID     string `json:"draftId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	// The clone must carry the same source.
	clone := fs.drafts[uuid.MustParse(created.DraftID)]
	if clone.SourceTex != "x" {
		t.Fatalf("clone source mismatch: %q", clone.SourceTex)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/resumes/"+resumeID.String()+"/variations", nil)
	rec = serve(h, req)
	var list struct {
		Variations []struct {
			Name string `json:"name"`
		} `json:"variations"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Variations) != 2 {
		t.Fatalf("want 2 variations, got %d", len(list.Variations))
	}
}

func TestCreateVariationDuplicateName(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	resumeID, _, _ := seedResumeWithDefaultVariation(t, fs)

	body, _ := json.Marshal(map[string]string{"name": "Original"})
	req := httptest.NewRequest(http.MethodPost, "/api/resumes/"+resumeID.String()+"/variations", bytes.NewReader(body))
	rec := serve(h, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate name: got %d, want 409", rec.Code)
	}
	if rec.Body.String() != "{\"error\":\"name_taken\"}\n" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestDeleteVariationGuardsDefault(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	resumeID, defaultID, _ := seedResumeWithDefaultVariation(t, fs)

	// Default variation: rejected.
	req := httptest.NewRequest(http.MethodDelete, "/api/variations/"+defaultID.String(), nil)
	rec := serve(h, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("delete default: got %d, want 422", rec.Code)
	}

	// Non-default variation: deleted with its draft.
	body, _ := json.Marshal(map[string]string{"name": "Acme"})
	req = httptest.NewRequest(http.MethodPost, "/api/resumes/"+resumeID.String()+"/variations", bytes.NewReader(body))
	rec = serve(h, req)
	var created struct {
		VariationID string `json:"variationId"`
		DraftID     string `json:"draftId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/variations/"+created.VariationID, nil)
	rec = serve(h, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete variation: got %d, want 204", rec.Code)
	}
	if _, ok := fs.drafts[uuid.MustParse(created.DraftID)]; ok {
		t.Fatal("draft was not cascaded")
	}
}

func TestGetVariationEditor(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	resumeID, variationID, draftID := seedResumeWithDefaultVariation(t, fs)

	req := httptest.NewRequest(http.MethodGet, "/api/resumes/"+resumeID.String()+"/variations/"+variationID.String(), nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get variation editor: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		DraftID       string `json:"draftId"`
		VariationID   string `json:"variationId"`
		VariationName string `json:"variationName"`
		SourceTex     string `json:"sourceTex"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.DraftID != draftID.String() || resp.VariationName != "Original" || resp.SourceTex != "x" {
		t.Fatalf("unexpected editor payload: %+v", resp)
	}

	// A variation id that belongs to another resume must 404 under this
	// resume's URL (no cross-contamination via guessed ids).
	_, otherVariationID, _ := seedResumeWithDefaultVariation(t, fs)
	req = httptest.NewRequest(http.MethodGet, "/api/resumes/"+resumeID.String()+"/variations/"+otherVariationID.String(), nil)
	if rec := serve(h, req); rec.Code != http.StatusNotFound {
		t.Fatalf("mismatched resume: got %d, want 404", rec.Code)
	}
}

func TestVariationsForeignUser(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)

	// Seed a resume owned by a different user directly.
	foreignResume := uuid.New()
	fs.resumes[foreignResume] = resumeFixture(foreignResume)
	fs.owner[foreignResume] = uuid.New()
	fv, err := fs.InsertVariation(context.Background(), store.InsertVariationParams{
		ResumeID: foreignResume, Name: "Original", IsDefault: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		method, path string
		want         int
	}{
		// List of a foreign resume is empty, not an error (no enumeration).
		{http.MethodGet, "/api/resumes/" + foreignResume.String() + "/variations", http.StatusOK},
		{http.MethodPost, "/api/resumes/" + foreignResume.String() + "/variations", http.StatusNotFound},
		{http.MethodGet, "/api/resumes/" + foreignResume.String() + "/variations/" + fv.ID.String(), http.StatusNotFound},
		{http.MethodDelete, "/api/variations/" + fv.ID.String(), http.StatusNotFound},
	}
	for _, tc := range cases {
		var body *bytes.Reader
		if tc.method == http.MethodPost {
			b, _ := json.Marshal(map[string]string{"name": "x"})
			body = bytes.NewReader(b)
		} else {
			body = bytes.NewReader(nil)
		}
		req := httptest.NewRequest(tc.method, tc.path, body)
		rec := serve(h, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s: got %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

func TestCreateVariationFromAnother(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	resumeID, _, _ := seedResumeWithDefaultVariation(t, fs)
	create := func(payload map[string]string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		return serve(h, httptest.NewRequest(http.MethodPost, "/api/resumes/"+resumeID.String()+"/variations", bytes.NewReader(body)))
	}

	rec := create(map[string]string{"name": "A"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create A: %d %s", rec.Code, rec.Body.String())
	}
	var a struct {
		VariationID string `json:"variationId"`
		DraftID     string `json:"draftId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	// Edit A's draft, then clone from A: B must carry A's edited source.
	draft := fs.drafts[uuid.MustParse(a.DraftID)]
	draft.SourceTex = "edited in A"
	fs.drafts[draft.ID] = draft

	rec = create(map[string]string{"name": "B", "fromVariationId": a.VariationID})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create B: %d %s", rec.Code, rec.Body.String())
	}
	var b struct {
		DraftID string `json:"draftId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if got := fs.drafts[uuid.MustParse(b.DraftID)].SourceTex; got != "edited in A" {
		t.Fatalf("clone source = %q, want %q", got, "edited in A")
	}

	// Unknown / malformed sources are rejected.
	if rec := create(map[string]string{"name": "C", "fromVariationId": uuid.NewString()}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown source: got %d, want 404", rec.Code)
	}
	if rec := create(map[string]string{"name": "D", "fromVariationId": "nope"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed source: got %d, want 400", rec.Code)
	}
}

func TestRenameVariation(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	resumeID, defaultID, _ := seedResumeWithDefaultVariation(t, fs)
	patch := func(path string, payload map[string]string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		return serve(h, httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body)))
	}

	// Create a second variation to collide with.
	body, _ := json.Marshal(map[string]string{"name": "Acme"})
	rec := serve(h, httptest.NewRequest(http.MethodPost, "/api/resumes/"+resumeID.String()+"/variations", bytes.NewReader(body)))
	var created struct {
		VariationID string `json:"variationId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	if rec := patch("/api/variations/"+created.VariationID, map[string]string{"name": "  Acme v2  "}); rec.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if got := fs.variations[uuid.MustParse(created.VariationID)].Name; got != "Acme v2" {
		t.Fatalf("name = %q, want trimmed %q", got, "Acme v2")
	}
	// The default variation can be renamed too.
	if rec := patch("/api/variations/"+defaultID.String(), map[string]string{"name": "Base"}); rec.Code != http.StatusOK {
		t.Fatalf("rename default: %d", rec.Code)
	}
	// Duplicate name within the resume -> 409 name_taken.
	rec = patch("/api/variations/"+created.VariationID, map[string]string{"name": "Base"})
	if rec.Code != http.StatusConflict || rec.Body.String() != "{\"error\":\"name_taken\"}\n" {
		t.Fatalf("duplicate: %d %s", rec.Code, rec.Body.String())
	}
	// Empty / unknown ids.
	if rec := patch("/api/variations/"+created.VariationID, map[string]string{"name": "   "}); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty name: got %d, want 400", rec.Code)
	}
	if rec := patch("/api/variations/"+uuid.NewString(), map[string]string{"name": "x"}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: got %d, want 404", rec.Code)
	}
}

func TestRenameResume(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	resumeID, _, _ := seedResumeWithDefaultVariation(t, fs)
	patch := func(id string, payload map[string]string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		return serve(h, httptest.NewRequest(http.MethodPatch, "/api/resumes/"+id, bytes.NewReader(body)))
	}
	if rec := patch(resumeID.String(), map[string]string{"title": "  My CV  "}); rec.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if got := fs.resumes[resumeID].Title; got != "My CV" {
		t.Fatalf("title = %q, want trimmed", got)
	}
	if rec := patch(resumeID.String(), map[string]string{"title": ""}); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty title: got %d, want 400", rec.Code)
	}
	if rec := patch(uuid.NewString(), map[string]string{"title": "x"}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown resume: got %d, want 404", rec.Code)
	}
}
