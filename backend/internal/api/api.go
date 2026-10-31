// Package api wires HTTP handlers. Every route except /healthz sits behind
// RequireAuth; every store call is filtered by the caller's user ID.
package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/harsh3dev/multi-resume/backend/internal/auth"
	"github.com/harsh3dev/multi-resume/backend/internal/checks"
	"github.com/harsh3dev/multi-resume/backend/internal/compile"
	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/parser"
	"github.com/harsh3dev/multi-resume/backend/internal/store"
)

const (
	maxTexSize   = 512 * 1024
	maxImageSize = 5 * 1024 * 1024
	maxJSONBody  = 64 * 1024
)

// Store is the handler-facing DB contract (implemented by sqlc queries).
type Store interface {
	InsertResume(ctx context.Context, arg store.InsertResumeParams) (store.Resume, error)
	ListResumesByUser(ctx context.Context, userID uuid.UUID) ([]store.Resume, error)
	GetResumeForUser(ctx context.Context, arg store.GetResumeForUserParams) (store.Resume, error)
	SoftDeleteResume(ctx context.Context, arg store.SoftDeleteResumeParams) error
	RenameResume(ctx context.Context, arg store.RenameResumeParams) (store.Resume, error)
	InsertVariation(ctx context.Context, arg store.InsertVariationParams) (store.Variation, error)
	ListVariationsForUser(ctx context.Context, arg store.ListVariationsForUserParams) ([]store.ListVariationsForUserRow, error)
	GetVariationForUser(ctx context.Context, arg store.GetVariationForUserParams) (store.Variation, error)
	GetDefaultVariationForResume(ctx context.Context, arg store.GetDefaultVariationForResumeParams) (store.Variation, error)
	DeleteVariation(ctx context.Context, id uuid.UUID) error
	RenameVariation(ctx context.Context, arg store.RenameVariationParams) (store.Variation, error)
	CloneDraftForVariation(ctx context.Context, arg store.CloneDraftForVariationParams) (store.Draft, error)
	GetDraftByVariationForUser(ctx context.Context, arg store.GetDraftByVariationForUserParams) (store.Draft, error)
	InsertDraft(ctx context.Context, arg store.InsertDraftParams) (store.Draft, error)
	GetDraftForUser(ctx context.Context, arg store.GetDraftForUserParams) (store.Draft, error)
	GetDraftByResumeForUser(ctx context.Context, arg store.GetDraftByResumeForUserParams) (store.Draft, error)
	InsertFailedUpload(ctx context.Context, arg store.InsertFailedUploadParams) error
	InsertAsset(ctx context.Context, arg store.InsertAssetParams) (store.Asset, error)
	UpdateDraftWithRevision(ctx context.Context, arg store.UpdateDraftWithRevisionParams) (store.Draft, error)
	InsertJob(ctx context.Context, arg store.InsertJobParams) (store.Job, error)
	InsertTailorRun(ctx context.Context, arg store.InsertTailorRunParams) (store.TailorRun, error)
	FinishTailorRun(ctx context.Context, arg store.FinishTailorRunParams) error
	UpdateTailorRunChanges(ctx context.Context, arg store.UpdateTailorRunChangesParams) error
	GetTailorRunForUser(ctx context.Context, arg store.GetTailorRunForUserParams) (store.TailorRun, error)
}

// Compiler runs compiles (internal/compile).
type Compiler interface {
	Compile(ctx context.Context, tex string, assets map[string]string, compiler string) (*compile.Result, error)
}

// Storage stores blobs (internal/storage).
type Storage interface {
	PutObject(ctx context.Context, key, contentType string, data []byte) error
}

// Presigner signs short-lived object URLs (storage.Client).
type Presigner interface {
	PresignGet(ctx context.Context, key string) (string, error)
}

// Deps are the handler dependencies; DB-less dev mode leaves Store nil.
type Deps struct {
	Auth     *auth.Middleware
	Store    Store
	Compiler Compiler // may be nil until compile is wired
	Storage  Storage  // may be nil without R2 config
	Presign  Presigner
	Tailor   TailorStreamer
	Ping     func(ctx context.Context) error // DB health check
}

// Router builds the chi router. /healthz stays public.
func Router(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(recoverer, logRequests)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Deep check: DB ping when a pool is configured.
		if deps.Ping != nil {
			if err := deps.Ping(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]bool{"ok": false})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	if deps.Auth == nil {
		// Auth is mandatory: without middleware configuration every /api route
		// fails closed with 503, never open.
		r.Route("/api", func(r chi.Router) {
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					writeError(w, http.StatusServiceUnavailable, "service unavailable")
				})
			})
			r.Get("/me", func(w http.ResponseWriter, _ *http.Request) {})
		})
		return r
	}
	r.Route("/api", func(r chi.Router) {
		r.Use(deps.Auth.RequireAuth)
		r.Get("/me", getMe)
		r.Get("/meta/fonts", deps.listFonts)
		r.Post("/resumes", deps.createResume)
		r.Get("/resumes", deps.listResumes)
		r.Delete("/resumes/{id}", deps.deleteResume)
		r.Patch("/resumes/{id}", deps.renameResume)
		r.Get("/drafts/{id}", deps.getDraft)
		r.Get("/resumes/{id}/draft", deps.getDraftByResume)
		r.Get("/resumes/{id}/variations", deps.listVariations)
		r.Post("/resumes/{id}/variations", deps.createVariation)
		r.Get("/resumes/{id}/variations/{variationId}", deps.getVariationEditor)
		r.Delete("/variations/{id}", deps.deleteVariation)
		r.Patch("/variations/{id}", deps.renameVariation)
		r.Post("/resumes/{id}/assets", deps.uploadAsset)
		r.Post("/drafts/{id}/ops", deps.applyOps)
		r.Put("/drafts/{id}/source", deps.putSource)
		r.Post("/drafts/{id}/tailor", deps.tailorDraft)
		r.Post("/tailor-runs/{id}/decisions", deps.tailorDecisions)
	})
	return r
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never log the Authorization header or query strings (may carry tokens).
		slog.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func identity(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
	}
	return id, ok
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func parseUUID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return uuid.Nil, false
	}
	return id, true
}

func getMe(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"userId": id.UserID.String(), "email": id.Email})
}

// --- resumes ---

func (d Deps) createResume(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	if !uploadLimit.allow(id.UserID) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxTexSize+64*1024)
	if err := r.ParseMultipartForm(maxTexSize); err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".tex") {
		writeError(w, http.StatusBadRequest, "only .tex files are supported")
		return
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(http.MaxBytesReader(w, file, maxTexSize)); err != nil {
		writeError(w, http.StatusBadRequest, "file too large")
		return
	}
	src := buf.String()
	if !strings.Contains(src, `\documentclass`) {
		writeError(w, http.StatusBadRequest, "not a LaTeX document")
		return
	}
	tree, err := parser.Parse(src, parser.ParseOptions{})
	if err != nil {
		_ = d.Store.InsertFailedUpload(r.Context(), store.InsertFailedUploadParams{
			UserID: id.UserID, SourceTex: src, ErrorLog: "parse failed",
		})
		writeError(w, http.StatusUnprocessableEntity, "template_not_supported")
		return
	}
	// Check 1: round-trip; failures flip fields to raw (check mutates tree).
	checkFailures := checks.CheckRoundTrip(src, tree)
	_ = checkFailures
	// Check 2 (probe compile) requires a live compiler; in dev mode it is
	// skipped and every field is kept.
	title := strings.TrimSuffix(header.Filename, ".tex")
	if title == "" || len(title) > 200 {
		title = "Resume"
	}
	var family pgtype.Text
	if tree.Meta.Family != nil {
		family = pgtype.Text{String: *tree.Meta.Family, Valid: true}
	}
	resume, err := d.Store.InsertResume(r.Context(), store.InsertResumeParams{
		UserID: id.UserID, Title: title, Compiler: tree.Meta.Compiler, Family: family,
	})
	if err != nil {
		slog.Error("insert resume", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	treeJSON, err := json.Marshal(tree)
	if err != nil {
		slog.Error("marshal tree", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Compile best-effort so the preview appears immediately; a compile
	// failure leaves the key empty and the editor shows it after edits.
	var lastGoodPdfKey pgtype.Text
	if d.Compiler != nil {
		if res, cerr := d.Compiler.Compile(r.Context(), src, nil, tree.Meta.Compiler); cerr == nil {
			lastGoodPdfKey = pgtype.Text{String: res.PDFKey, Valid: true}
		} else {
			slog.Warn("upload compile failed", "error", cerr.Error())
		}
	}
	// Every new resume starts with a default "Original" variation owning the
	// initial draft.
	variation, err := d.Store.InsertVariation(r.Context(), store.InsertVariationParams{
		ResumeID: resume.ID, Name: "Original", IsDefault: true,
	})
	if err != nil {
		slog.Error("insert variation", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	draft, err := d.Store.InsertDraft(r.Context(), store.InsertDraftParams{
		ResumeID: resume.ID, VariationID: variation.ID,
		SourceTex: src, SourceSha256: tree.SourceHash, Tree: treeJSON, AssetMap: []byte("{}"),
		LastGoodPdfKey: lastGoodPdfKey,
	})
	if err != nil {
		slog.Error("insert draft", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	missing := []string{}
	for _, a := range tree.Assets {
		if !a.Uploaded {
			missing = append(missing, a.Path)
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"resumeId": resume.ID, "variationId": variation.ID, "draftId": draft.ID, "tree": tree,
		"report": tree.Report, "missingImages": missing,
	})
}

func (d Deps) listResumes(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	rows, err := d.Store.ListResumesByUser(r.Context(), id.UserID)
	if err != nil {
		slog.Error("list resumes", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	type row struct {
		ID        string  `json:"id"`
		Title     string  `json:"title"`
		Compiler  string  `json:"compiler"`
		Family    *string `json:"family"`
		CreatedAt string  `json:"createdAt"`
	}
	out := []row{}
	for _, v := range rows {
		entry := row{
			ID: v.ID.String(), Title: v.Title, Compiler: v.Compiler,
			CreatedAt: v.CreatedAt.Time.Format("2006-01-02 15:04"),
		}
		if v.Family.Valid {
			entry.Family = &v.Family.String
		}
		out = append(out, entry)
	}
	writeJSON(w, http.StatusOK, map[string]any{"resumes": out})
}

func (d Deps) deleteResume(w http.ResponseWriter, r *http.Request) {
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
	// 404 on missing or foreign rows (no enumeration).
	if _, err := d.Store.GetResumeForUser(r.Context(), store.GetResumeForUserParams{ID: resumeID, UserID: id.UserID}); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := d.Store.SoftDeleteResume(r.Context(), store.SoftDeleteResumeParams{ID: resumeID, UserID: id.UserID}); err != nil {
		slog.Error("delete resume", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// renameResume handles PATCH /api/resumes/:id {title}.
func (d Deps) renameResume(w http.ResponseWriter, r *http.Request) {
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
		Title string `json:"title"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len(req.Title) > 200 {
		writeError(w, http.StatusBadRequest, "title required (max 200 chars)")
		return
	}
	// 404 on missing or foreign rows (no enumeration): the update is scoped to
	// the caller's own, non-deleted resume.
	resume, err := d.Store.RenameResume(r.Context(), store.RenameResumeParams{ID: resumeID, UserID: id.UserID, Title: req.Title})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": resume.ID, "title": resume.Title})
}

// --- drafts ---

func (d Deps) getDraft(w http.ResponseWriter, r *http.Request) {
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
	draft, err := d.Store.GetDraftForUser(r.Context(), store.GetDraftForUserParams{ID: draftID, UserID: id.UserID})
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
	resp := map[string]any{"tree": tree, "revision": draft.Revision, "sourceTex": draft.SourceTex}
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

// getDraftByResume resolves the 1:1 draft for a resume (the editor route
// carries the resume id).
func (d Deps) getDraftByResume(w http.ResponseWriter, r *http.Request) {
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
	draft, err := d.Store.GetDraftByResumeForUser(r.Context(), store.GetDraftByResumeForUserParams{ResumeID: resumeID, UserID: id.UserID})
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
	resp := map[string]any{
		"draftId": draft.ID, "tree": tree, "revision": draft.Revision,
		"sourceTex": draft.SourceTex,
	}
	if draft.LastGoodPdfKey.Valid && d.Presign != nil {
		if url, err := d.Presign.PresignGet(r.Context(), draft.LastGoodPdfKey.String); err == nil {
			resp["pdfUrl"] = url
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func currentParserVersion() string {
	// The parser stamps this into every tree; kept in one place here.
	tree, err := parser.Parse(`\documentclass{article}`+"\n"+`\begin{document}x\end{document}`, parser.ParseOptions{})
	if err != nil {
		return "1"
	}
	return tree.ParserVersion
}

// --- assets ---

func (d Deps) uploadAsset(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	resumeID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil || d.Storage == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	if _, err := d.Store.GetResumeForUser(r.Context(), store.GetResumeForUserParams{ID: resumeID, UserID: id.UserID}); err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImageSize+64*1024)
	if err := r.ParseMultipartForm(maxImageSize); err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(http.MaxBytesReader(w, file, maxImageSize)); err != nil {
		writeError(w, http.StatusBadRequest, "file too large")
		return
	}
	data := buf.Bytes()
	mime := http.DetectContentType(data)
	allowed := map[string]string{"image/png": ".png", "image/jpeg": ".jpg"}
	ext, ok := allowed[mime]
	if !ok {
		writeError(w, http.StatusBadRequest, "only png and jpg images are supported")
		return
	}
	texPath := sanitizeTexPath(header.Filename)
	if texPath == "" {
		texPath = "image" + ext
	}
	sumBytes := sha256.Sum256(data)
	sum := hex.EncodeToString(sumBytes[:])
	key := "resume/" + resumeID.String() + "/" + sum + ext
	if err := d.Storage.PutObject(r.Context(), key, mime, data); err != nil {
		slog.Error("asset upload", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	asset, err := d.Store.InsertAsset(r.Context(), store.InsertAssetParams{
		ResumeID: resumeID, TexPath: texPath, StorageKey: key, Sha256: sum, Mime: mime,
	})
	if err != nil {
		slog.Error("insert asset", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"assetId": asset.ID, "texPath": asset.TexPath})
}

// sanitizeTexPath keeps a bare filename: no traversal, no separators.
func sanitizeTexPath(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "\x00{}%") {
		return ""
	}
	if len(name) > 128 {
		return ""
	}
	return name
}
