package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
	"github.com/harsh3dev/multi-resume/backend/internal/store"
)

// listVariations handles GET /api/resumes/:id/variations — the dashboard
// accordion content. Includes the owning draft id per variation.
func (d Deps) listVariations(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	resumeID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	rows, err := d.Store.ListVariationsForUser(r.Context(), store.ListVariationsForUserParams{ResumeID: resumeID, UserID: id.UserID})
	if err != nil {
		slog.Error("list variations", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	type entry struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		IsDefault bool    `json:"isDefault"`
		DraftID   *string `json:"draftId"`
		CreatedAt string  `json:"createdAt"`
	}
	out := []entry{}
	for _, v := range rows {
		e := entry{
			ID: v.ID.String(), Name: v.Name, IsDefault: v.IsDefault,
			CreatedAt: v.CreatedAt.Time.Format("2006-01-02 15:04"),
		}
		if v.DraftID != nil {
			s := v.DraftID.String()
			e.DraftID = &s
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"variations": out})
}

// createVariation handles POST /api/resumes/:id/variations — clones the
// default variation's draft so the new variation starts from the same
// content.
func (d Deps) createVariation(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	resumeID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var req struct {
		Name string `json:"name"`
		// FromVariationID optionally picks the variation to clone (the editor
		// sends the one being edited); the default variation otherwise.
		FromVariationID string `json:"fromVariationId"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 120 {
		writeError(w, http.StatusBadRequest, "name required (max 120 chars)")
		return
	}
	// Ownership + the clone source: the requested variation, else the
	// default. Either way it must belong to this user and this resume.
	var def store.Variation
	var err error
	if req.FromVariationID == "" {
		def, err = d.Store.GetDefaultVariationForResume(r.Context(), store.GetDefaultVariationForResumeParams{ResumeID: resumeID, UserID: id.UserID})
	} else {
		fromID, perr := uuid.Parse(req.FromVariationID)
		if perr != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
			return
		}
		def, err = d.Store.GetVariationForUser(r.Context(), store.GetVariationForUserParams{ID: fromID, UserID: id.UserID})
		if err == nil && def.ResumeID != resumeID {
			err = errors.New("variation belongs to another resume")
		}
	}
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	variation, err := d.Store.InsertVariation(r.Context(), store.InsertVariationParams{
		ResumeID: resumeID, Name: req.Name, IsDefault: false,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "name_taken")
			return
		}
		slog.Error("insert variation", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	draft, err := d.Store.CloneDraftForVariation(r.Context(), store.CloneDraftForVariationParams{
		VariationID:   def.ID,
		VariationID_2: variation.ID,
	})
	if err != nil {
		slog.Error("clone draft", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"variationId": variation.ID, "draftId": draft.ID, "name": variation.Name,
	})
}

// getVariationEditor handles GET /api/resumes/:id/variations/:variationId —
// the editor payload for one variation. Same shape as the draft endpoints
// plus the variation name.
func (d Deps) getVariationEditor(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	resumeID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	variationID, ok := parseUUID(w, r, "variationId")
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	variation, err := d.Store.GetVariationForUser(r.Context(), store.GetVariationForUserParams{ID: variationID, UserID: id.UserID})
	if err != nil || variation.ResumeID != resumeID {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	resume, err := d.Store.GetResumeForUser(r.Context(), store.GetResumeForUserParams{ID: resumeID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	draft, err := d.Store.GetDraftByVariationForUser(r.Context(), store.GetDraftByVariationForUserParams{VariationID: variationID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var tree model.Resume
	if err := json.Unmarshal(draft.Tree, &tree); err != nil {
		slog.Error("corrupt tree", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Stale parser version: re-parse and carry IDs (parser carries by structure).
	if tree.ParserVersion != currentParserVersion() {
		if fresh, err := parser.Parse(draft.SourceTex, parser.ParseOptions{}); err == nil {
			tree = *fresh
		}
	}
	resp := map[string]any{
		"draftId": draft.ID, "variationId": variation.ID, "variationName": variation.Name,
		"resumeTitle": resume.Title, "isDefault": variation.IsDefault,
		"tree": tree, "revision": draft.Revision, "sourceTex": draft.SourceTex,
	}
	if draft.LastGoodPdfKey.Valid && d.Presign != nil {
		if url, err := d.Presign.PresignGet(r.Context(), draft.LastGoodPdfKey.String); err == nil {
			resp["pdfUrl"] = url
		}
	}
	if draft.CompileError.Valid {
		resp["compileError"] = draft.CompileError.String
	}
	writeJSON(w, http.StatusOK, resp)
}

// deleteVariation handles DELETE /api/variations/:id. The default variation
// cannot be deleted (a resume always keeps at least one variation); deleting
// a variation cascades to its draft.
func (d Deps) deleteVariation(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	variationID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	variation, err := d.Store.GetVariationForUser(r.Context(), store.GetVariationForUserParams{ID: variationID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if variation.IsDefault {
		writeError(w, http.StatusUnprocessableEntity, "cannot_delete_default")
		return
	}
	if err := d.Store.DeleteVariation(r.Context(), variationID); err != nil {
		slog.Error("delete variation", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// renameVariation handles PATCH /api/variations/:id {name}. Names are unique
// per resume (409 name_taken). Any variation, the default included, can be
// renamed.
func (d Deps) renameVariation(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	variationID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var req struct {
		Name string `json:"name"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 120 {
		writeError(w, http.StatusBadRequest, "name required (max 120 chars)")
		return
	}
	// Ownership: 404 on missing or foreign rows (no enumeration).
	if _, err := d.Store.GetVariationForUser(r.Context(), store.GetVariationForUserParams{ID: variationID, UserID: id.UserID}); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	v, err := d.Store.RenameVariation(r.Context(), store.RenameVariationParams{ID: variationID, Name: req.Name})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "name_taken")
			return
		}
		slog.Error("rename variation", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": v.ID, "name": v.Name})
}
