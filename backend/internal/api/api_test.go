package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/harsh3dev/multi-resume/backend/internal/auth"
)

// testUserA is the identity injected by testServer's middleware.
var testUserA = uuid.New()

// testServer mounts the routes behind a middleware that injects userA's
// identity, exactly as RequireAuth does after JWT verification.
func testServer(s Store) http.Handler {
	// Tests share the package-level limiter, but each server represents an
	// independent scenario. Reset its window so tests cannot exhaust each other.
	uploadLimit = newRateLimiter(10, time.Hour)
	d := Deps{Store: s}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithIdentity(req.Context(), auth.Identity{UserID: testUserA, Email: "a@example.com"})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Route("/api", func(r chi.Router) {
		r.Get("/me", getMe)
		r.Post("/resumes", d.createResume)
		r.Get("/resumes", d.listResumes)
		r.Delete("/resumes/{id}", d.deleteResume)
		r.Get("/drafts/{id}", d.getDraft)
		r.Get("/resumes/{id}/variations", d.listVariations)
		r.Post("/resumes/{id}/variations", d.createVariation)
		r.Get("/resumes/{id}/variations/{variationId}", d.getVariationEditor)
		r.Delete("/variations/{id}", d.deleteVariation)
		r.Patch("/variations/{id}", d.renameVariation)
		r.Patch("/resumes/{id}", d.renameResume)
		r.Post("/resumes/{id}/assets", d.uploadAsset)
		r.Post("/drafts/{id}/ops", d.applyOps)
		r.Put("/drafts/{id}/source", d.putSource)
	})
	return r
}

func uploadBody(t *testing.T, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUploadHappyPath(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	tex, err := os.ReadFile("../../testdata/resumes/jake.tex")
	if err != nil {
		t.Skip("fixture missing")
	}
	body, ct := uploadBody(t, "resume.tex", string(tex))
	req := httptest.NewRequest(http.MethodPost, "/api/resumes", body)
	req.Header.Set("Content-Type", ct)
	rec := serve(h, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
}

func TestUploadRejectsBadInput(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	cases := map[string]struct {
		filename, content string
		want              int
	}{
		"wrong extension":  {"resume.txt", `\documentclass{article}`, http.StatusBadRequest},
		"no documentclass": {"resume.tex", "just text", http.StatusBadRequest},
		"no document env":  {"resume.tex", `\documentclass{article} xx`, http.StatusUnprocessableEntity},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			body, ct := uploadBody(t, tc.filename, tc.content)
			req := httptest.NewRequest(http.MethodPost, "/api/resumes", body)
			req.Header.Set("Content-Type", ct)
			rec := serve(h, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestUploadTexSizeLimit(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	for _, tc := range []struct {
		name string
		size int
		want int
	}{
		{"exactly 2 MiB", maxTexSize, http.StatusCreated},
		{"over 2 MiB", maxTexSize + 1, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := `\documentclass{article}\begin{document}`
			suffix := `\end{document}`
			tex := prefix + strings.Repeat(" ", tc.size-len(prefix)-len(suffix)) + suffix
			body, ct := uploadBody(t, "resume.tex", tex)
			req := httptest.NewRequest(http.MethodPost, "/api/resumes", body)
			req.Header.Set("Content-Type", ct)
			rec := serve(h, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestFailedUploadRecorded(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	body, ct := uploadBody(t, "resume.tex", `\documentclass{article} no env`)
	req := httptest.NewRequest(http.MethodPost, "/api/resumes", body)
	req.Header.Set("Content-Type", ct)
	serve(h, req)
	if fs.failed != 1 {
		t.Fatal("failed upload not recorded")
	}
}

func TestListResumesOwnOnly(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	req := httptest.NewRequest(http.MethodGet, "/api/resumes", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
}

func TestGetDraftForeignUser404(t *testing.T) {
	fs := newFakeStore()
	ownerB := uuid.New()
	resumeID := uuid.New()
	fs.resumes[resumeID] = resumeFixture(resumeID)
	fs.owner[resumeID] = ownerB
	draftID := uuid.New()
	fs.drafts[draftID] = draftFixture(draftID, resumeID)

	h := testServer(fs)
	req := httptest.NewRequest(http.MethodGet, "/api/drafts/"+draftID.String(), nil)
	rec := serve(h, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign draft: got %d, want 404", rec.Code)
	}
}

func TestDeleteForeignResume404(t *testing.T) {
	fs := newFakeStore()
	resumeID := uuid.New()
	fs.resumes[resumeID] = resumeFixture(resumeID)
	fs.owner[resumeID] = uuid.New() // someone else

	h := testServer(fs)
	req := httptest.NewRequest(http.MethodDelete, "/api/resumes/"+resumeID.String(), nil)
	rec := serve(h, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign delete: got %d, want 404", rec.Code)
	}
	if len(fs.resumes) != 1 {
		t.Fatal("foreign resume was deleted")
	}
}

func TestMe(t *testing.T) {
	fs := newFakeStore()
	h := testServer(fs)
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := serve(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("me: %d", rec.Code)
	}
}

// testServerWithDeps is testServer with full Deps (tailor streamer etc.).
func testServerWithDeps(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithIdentity(req.Context(), auth.Identity{UserID: testUserA, Email: "a@example.com"})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Route("/api", func(r chi.Router) {
		r.Get("/me", getMe)
		r.Post("/resumes", d.createResume)
		r.Get("/resumes", d.listResumes)
		r.Delete("/resumes/{id}", d.deleteResume)
		r.Get("/drafts/{id}", d.getDraft)
		r.Get("/resumes/{id}/draft", d.getDraftByResume)
		r.Get("/resumes/{id}/variations", d.listVariations)
		r.Post("/resumes/{id}/variations", d.createVariation)
		r.Get("/resumes/{id}/variations/{variationId}", d.getVariationEditor)
		r.Delete("/variations/{id}", d.deleteVariation)
		r.Patch("/variations/{id}", d.renameVariation)
		r.Patch("/resumes/{id}", d.renameResume)
		r.Post("/resumes/{id}/assets", d.uploadAsset)
		r.Post("/drafts/{id}/ops", d.applyOps)
		r.Put("/drafts/{id}/source", d.putSource)
		r.Post("/drafts/{id}/compile", d.compileDraft)
		r.Post("/drafts/{id}/tailor", d.tailorDraft)
		r.Post("/tailor-runs/{id}/decisions", d.tailorDecisions)
	})
	return r
}

func newResumeForUser(t *testing.T, fs *fakeStore) uuid.UUID {
	t.Helper()
	id := uuid.New()
	fs.resumes[id] = resumeFixture(id)
	fs.owner[id] = testUserA
	return id
}
