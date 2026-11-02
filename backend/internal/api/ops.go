package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/harsh3dev/multi-resume/backend/internal/engine"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
	"github.com/harsh3dev/multi-resume/backend/internal/store"
)

// opsRequest is POST /api/drafts/:id/ops. Unknown fields are rejected.
type opsRequest struct {
	Revision int        `json:"revision"`
	Ops      []model.Op `json:"ops"`
}

// applyOps handles POST /api/drafts/:id/ops: the one mutation path for edits.
func (d Deps) applyOps(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	draftID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var req opsRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || len(req.Ops) == 0 || len(req.Ops) > 200 {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if !opsLimit.allow(id.UserID) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	draft, err := d.Store.GetDraftForUser(r.Context(), store.GetDraftForUserParams{ID: draftID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if draft.Revision != int32(req.Revision) {
		writeError(w, http.StatusConflict, "stale_revision")
		return
	}
	var tree model.Resume
	if err := json.Unmarshal(draft.Tree, &tree); err != nil {
		slog.Error("corrupt tree", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	result, err := engine.ApplyOps(draft.SourceTex, &tree, req.Ops)
	if err != nil {
		slog.Warn("op batch rejected", "draft", draftID, "ops", len(req.Ops), "error", err.Error())
		writeError(w, http.StatusUnprocessableEntity, "edit_rejected")
		return
	}
	newTreeJSON, err := json.Marshal(result.Tree)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Saving never compiles: the PDF is refreshed only by POST
	// /api/drafts/:id/compile. The stored PDF and last compile error are
	// carried over untouched; the store marks the draft as needing a compile.
	pdfKey := draft.LastGoodPdfKey
	compileErr := draft.CompileError
	updated, err := d.Store.UpdateDraftWithRevision(r.Context(), store.UpdateDraftWithRevisionParams{
		ID:             draft.ID,
		Revision:       draft.Revision,
		SourceTex:      result.Source,
		SourceSha256:   result.Tree.SourceHash,
		Tree:           newTreeJSON,
		AssetMap:       draft.AssetMap,
		LastGoodPdfKey: pdfKey,
		CompileError:   compileErr,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Lost a race against another tab's save.
			writeError(w, http.StatusConflict, "stale_revision")
			return
		}
		slog.Error("draft save", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	markUploaded(result.Tree, draft.AssetMap)
	resp := map[string]any{"revision": updated.Revision, "tree": result.Tree, "sourceTex": result.Source, "pdfStale": true}
	writeJSON(w, http.StatusOK, resp)
}

// putSource handles PUT /api/drafts/:id/source — the Source tab save path.
// It goes through the same parse → save → compile flow as ops.
func (d Deps) putSource(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	draftID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxTexSize)
	var req struct {
		Source   string `json:"source"`
		Revision int    `json:"revision"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Source == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if !opsLimit.allow(id.UserID) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	draft, err := d.Store.GetDraftForUser(r.Context(), store.GetDraftForUserParams{ID: draftID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if draft.Revision != int32(req.Revision) {
		writeError(w, http.StatusConflict, "stale_revision")
		return
	}
	if !bytes.Contains([]byte(req.Source), []byte(`\documentclass`)) {
		writeError(w, http.StatusBadRequest, "not a LaTeX document")
		return
	}
	// Re-parse fresh; the engine's carry-over preserves IDs by structure.
	result, err := engine.ApplyOps(draft.SourceTex, mustTree(draft), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	_ = result
	newTree, err := parser.Parse(req.Source, parser.ParseOptions{})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "template_not_supported")
		return
	}
	newTreeJSON, err := json.Marshal(newTree)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Save only; compiling is a separate, explicit request (see compileDraft).
	pdfKey := draft.LastGoodPdfKey
	compileErr := draft.CompileError
	updated, err := d.Store.UpdateDraftWithRevision(r.Context(), store.UpdateDraftWithRevisionParams{
		ID:             draft.ID,
		Revision:       draft.Revision,
		SourceTex:      req.Source,
		SourceSha256:   newTree.SourceHash,
		Tree:           newTreeJSON,
		AssetMap:       draft.AssetMap,
		LastGoodPdfKey: pdfKey,
		CompileError:   compileErr,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusConflict, "stale_revision")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	markUploaded(newTree, draft.AssetMap)
	resp := map[string]any{"revision": updated.Revision, "tree": newTree, "sourceTex": req.Source, "pdfStale": true}
	writeJSON(w, http.StatusOK, resp)
}

func mustTree(d store.Draft) *model.Resume {
	var tree model.Resume
	_ = json.Unmarshal(d.Tree, &tree)
	return &tree
}
