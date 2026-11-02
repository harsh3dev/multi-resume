package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
)

// countingCompiler records how many compiles ran.
type countingCompiler struct {
	calls atomic.Int32
	err   error
}

func (c *countingCompiler) Compile(_ context.Context, _ string, _ map[string]string, _ string) (*compile.Result, error) {
	c.calls.Add(1)
	if c.err != nil {
		return nil, c.err
	}
	return &compile.Result{PDFKey: "pdf/test.pdf", PageCount: 1}, nil
}

// resetCompileLimits gives a test fresh limiter budgets and restores them.
func resetCompileLimits(t *testing.T, perMin, perDay int) {
	t.Helper()
	oldMin, oldDay := compileLimit, compileDailyLimit
	compileLimit = newRateLimiter(perMin, time.Minute)
	compileDailyLimit = newRateLimiter(perDay, 24*time.Hour)
	t.Cleanup(func() { compileLimit, compileDailyLimit = oldMin, oldDay })
}

func postCompile(h http.Handler, draftID string) *httptest.ResponseRecorder {
	return serve(h, httptest.NewRequest(http.MethodPost, "/api/drafts/"+draftID+"/compile", nil))
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return m
}

func TestSaveDoesNotCompileAndMarksStale(t *testing.T) {
	resetCompileLimits(t, 100, 100)
	fs := newFakeStore()
	cc := &countingCompiler{}
	h := testServerWithDeps(Deps{Store: fs, Compiler: cc})
	draftID, rev, tree := seedDraft(t, h)
	if got := cc.calls.Load(); got != 1 {
		t.Fatalf("upload compiles once, got %d", got)
	}
	rec := postOps(t, h, draftID, rev, []model.Op{{Kind: "edit", ID: firstBulletID(t, tree), Value: "New text"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("ops: %d %s", rec.Code, rec.Body.String())
	}
	if got := cc.calls.Load(); got != 1 {
		t.Fatalf("saving must not compile; compiles=%d", got)
	}
	body := decodeBody(t, rec)
	if body["pdfStale"] != true {
		t.Fatalf("pdfStale: %v", body["pdfStale"])
	}
	if _, has := body["pdf"]; has {
		t.Fatal("ops response must not carry a pdf")
	}
}

func TestUploadCompileSetsFlag(t *testing.T) {
	resetCompileLimits(t, 100, 100)
	for name, tc := range map[string]struct {
		err       error
		wantStale bool
	}{
		"ok":     {nil, false},
		"failed": {&compile.CompileError{Message: "boom"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			fs := newFakeStore()
			h := testServerWithDeps(Deps{Store: fs, Compiler: &countingCompiler{err: tc.err}})
			draftID, _, _ := seedDraft(t, h)
			rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/drafts/"+draftID, nil))
			if got := decodeBody(t, rec)["pdfStale"]; got != tc.wantStale {
				t.Fatalf("pdfStale = %v, want %v", got, tc.wantStale)
			}
		})
	}
}

func TestCompileEndpointFlow(t *testing.T) {
	resetCompileLimits(t, 100, 100)
	fs := newFakeStore()
	cc := &countingCompiler{}
	h := testServerWithDeps(Deps{Store: fs, Compiler: cc})
	draftID, rev, tree := seedDraft(t, h)

	// Up to date right after upload: answered from the stored PDF, no compile.
	rec := postCompile(h, draftID)
	if rec.Code != http.StatusOK || cc.calls.Load() != 1 {
		t.Fatalf("clean compile: %d calls=%d", rec.Code, cc.calls.Load())
	}
	if decodeBody(t, rec)["pdfStale"] != false {
		t.Fatal("expected not stale")
	}

	postOps(t, h, draftID, rev, []model.Op{{Kind: "edit", ID: firstBulletID(t, tree), Value: "Changed"}})
	rec = postCompile(h, draftID)
	body := decodeBody(t, rec)
	if rec.Code != http.StatusOK || cc.calls.Load() != 2 {
		t.Fatalf("stale compile: %d calls=%d", rec.Code, cc.calls.Load())
	}
	if body["pdfStale"] != false || body["pdf"] == nil {
		t.Fatalf("after compile: %v", body)
	}

	// Compiling again with nothing changed is free.
	postCompile(h, draftID)
	if cc.calls.Load() != 2 {
		t.Fatalf("idempotent compile ran again: %d", cc.calls.Load())
	}
}

func TestCompileFailureKeepsStaleAndHidesInternals(t *testing.T) {
	resetCompileLimits(t, 100, 100)
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"latex error":    {&compile.CompileError{Message: "Undefined control sequence"}, "Undefined control sequence"},
		"internal error": {errors.New("compile: fetch asset: dial tcp 10.0.0.5: refused"), "compile unavailable"},
	} {
		t.Run(name, func(t *testing.T) {
			fs := newFakeStore()
			cc := &countingCompiler{}
			h := testServerWithDeps(Deps{Store: fs, Compiler: cc})
			draftID, rev, tree := seedDraft(t, h)
			postOps(t, h, draftID, rev, []model.Op{{Kind: "edit", ID: firstBulletID(t, tree), Value: "x"}})
			cc.err = tc.err
			rec := postCompile(h, draftID)
			body := decodeBody(t, rec)
			if rec.Code != http.StatusOK || body["compileError"] != tc.want || body["pdfStale"] != true {
				t.Fatalf("got %d %v", rec.Code, body)
			}
		})
	}
}

func TestCompileForeignDraft404(t *testing.T) {
	resetCompileLimits(t, 100, 100)
	fs := newFakeStore()
	cc := &countingCompiler{}
	resumeID := uuid.New()
	fs.resumes[resumeID] = resumeFixture(resumeID)
	fs.owner[resumeID] = uuid.New() // someone else's
	draftID := uuid.New()
	d := draftFixture(draftID, resumeID)
	d.NeedsCompile = true
	fs.drafts[draftID] = d
	h := testServerWithDeps(Deps{Store: fs, Compiler: cc})
	if rec := postCompile(h, draftID.String()); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign draft: got %d, want 404", rec.Code)
	}
	if cc.calls.Load() != 0 {
		t.Fatal("compiled someone else's draft")
	}
}

func TestCompileRateLimited(t *testing.T) {
	resetCompileLimits(t, 2, 100) // upload uses one, the first click the other
	fs := newFakeStore()
	cc := &countingCompiler{err: &compile.CompileError{Message: "boom"}}
	h := testServerWithDeps(Deps{Store: fs, Compiler: cc})
	draftID, _, _ := seedDraft(t, h) // upload compile fails → draft stays stale
	if rec := postCompile(h, draftID); rec.Code != http.StatusOK {
		t.Fatalf("first compile: %d", rec.Code)
	}
	if rec := postCompile(h, draftID); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second compile: got %d, want 429", rec.Code)
	}
}

func TestCompileWithoutCompiler503(t *testing.T) {
	fs := newFakeStore()
	h := testServerWithDeps(Deps{Store: fs})
	if rec := postCompile(h, uuid.NewString()); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rec.Code)
	}
}
