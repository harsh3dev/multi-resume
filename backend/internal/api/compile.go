package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/singleflight"

	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/store"
)

// compileFlight collapses concurrent compile requests for the same draft
// revision (double clicks, two tabs) into one compile.
var compileFlight singleflight.Group

// compileOutcome is the shared result of one collapsed compile.
type compileOutcome struct {
	pdfKey     string
	compileErr string // client-safe message; empty on success
}

// compileDraft handles POST /api/drafts/:id/compile — the only route that
// compiles after upload. It takes no source from the client: it compiles the
// draft's stored source, so it adds no new input surface.
func (d Deps) compileDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	draftID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil || d.Compiler == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	// Ownership check (IDOR): a foreign or deleted draft is a plain 404.
	draft, err := d.Store.GetDraftForUser(r.Context(), store.GetDraftForUserParams{ID: draftID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	// Nothing changed since the last good compile: answer from the stored PDF.
	// Free, so it does not consume rate-limit budget.
	if !draft.NeedsCompile && draft.LastGoodPdfKey.Valid {
		d.writeCompileResponse(w, r, draft, draft.LastGoodPdfKey.String, "")
		return
	}
	if !compileLimit.allow(id.UserID) || !compileDailyLimit.allow(id.UserID) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}

	var tree model.Resume
	if err := unmarshalTree(draft, &tree); err != nil {
		slog.Error("corrupt tree", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	flightKey := fmt.Sprintf("%s:%d", draft.ID, draft.Revision)
	v, _, _ := compileFlight.Do(flightKey, func() (any, error) {
		res, cerr := d.Compiler.Compile(r.Context(), draft.SourceTex, compileAssets(&tree, draft.AssetMap), tree.Meta.Compiler)
		if cerr != nil {
			return compileOutcome{compileErr: safeCompileMessage(cerr)}, cerr
		}
		return compileOutcome{pdfKey: res.PDFKey}, nil
	})
	out, _ := v.(compileOutcome)
	if r.Context().Err() != nil {
		return // the client left; nothing to record or answer
	}
	if out.pdfKey == "" && out.compileErr == "" {
		out.compileErr = "compile unavailable"
	}

	params := store.SetDraftCompileResultParams{
		ID: draft.ID, UserID: id.UserID, Revision: draft.Revision,
		NeedsCompile: out.pdfKey == "",
	}
	if out.pdfKey != "" {
		params.LastGoodPdfKey = pgtype.Text{String: out.pdfKey, Valid: true}
	} else {
		params.CompileError = pgtype.Text{String: out.compileErr, Valid: true}
	}
	if _, err := d.Store.SetDraftCompileResult(r.Context(), params); err != nil {
		slog.Error("record compile result", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Re-read so the response reflects a save that landed mid-compile.
	fresh, err := d.Store.GetDraftForUser(r.Context(), store.GetDraftForUserParams{ID: draftID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	d.writeCompileResponse(w, r, fresh, out.pdfKey, out.compileErr)
}

func (d Deps) writeCompileResponse(w http.ResponseWriter, r *http.Request, draft store.Draft, pdfKey, compileErr string) {
	resp := map[string]any{"revision": draft.Revision, "pdfStale": draft.NeedsCompile}
	if compileErr != "" {
		resp["compileError"] = compileErr
	}
	// On failure the last good PDF (if any) stays available alongside the error.
	if pdfKey == "" && draft.LastGoodPdfKey.Valid {
		pdfKey = draft.LastGoodPdfKey.String
	}
	if pdfKey != "" {
		pdf := map[string]any{"key": pdfKey}
		if d.Presign != nil {
			if url, err := d.Presign.PresignGet(r.Context(), pdfKey); err == nil {
				pdf["url"] = url
			}
		}
		resp["pdf"] = pdf
	}
	writeJSON(w, http.StatusOK, resp)
}

// safeCompileMessage returns text that is safe to show a user: only the
// parsed LaTeX error from a CompileError. Anything else (storage, transport,
// internal failures) becomes a generic message so internals never leak.
func safeCompileMessage(err error) string {
	var ce *compile.CompileError
	if errors.As(err, &ce) && ce.Message != "" {
		return ce.Message
	}
	return "compile unavailable"
}

func unmarshalTree(d store.Draft, tree *model.Resume) error {
	return json.Unmarshal(d.Tree, tree)
}
