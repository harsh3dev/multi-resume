package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/harsh3dev/multi-resume/backend/internal/model"
	"github.com/harsh3dev/multi-resume/backend/internal/store"
	"github.com/harsh3dev/multi-resume/backend/internal/tailor"
)

// TailorStreamer abstracts the model call (OpenAI-compatible endpoints in
// prod, e.g. Groq; fake in tests).
type TailorStreamer interface {
	StreamText(ctx context.Context, system, user string, onLine func(string)) error
}

// rateLimiter is a simple per-user sliding-window limiter.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[uuid.UUID][]time.Time
	max     int
	period  time.Duration
	now     func() time.Time
}

func newRateLimiter(max int, period time.Duration) *rateLimiter {
	return &rateLimiter{windows: map[uuid.UUID][]time.Time{}, max: max, period: period, now: time.Now}
}

func (rl *rateLimiter) allow(user uuid.UUID) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := rl.now().Add(-rl.period)
	kept := rl.windows[user][:0]
	for _, t := range rl.windows[user] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	rl.windows[user] = kept
	if len(kept) >= rl.max {
		return false
	}
	rl.windows[user] = append(rl.windows[user], rl.now())
	return true
}

var (
	tailorLimit = newRateLimiter(10, time.Hour)
	opsLimit    = newRateLimiter(30, time.Minute)
	uploadLimit = newRateLimiter(10, time.Hour)
	// Compiles are the expensive operation: a short burst limit plus a daily cap.
	compileLimit      = newRateLimiter(6, time.Minute)
	compileDailyLimit = newRateLimiter(200, 24*time.Hour)
)

// tailorDraft handles POST /api/drafts/:id/tailor: streams validated AI
// change proposals as incremental JSON snapshots (BUILD-PLAN §9.3).
func (d Deps) tailorDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	draftID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil || d.Tailor == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	if !tailorLimit.allow(id.UserID) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 30*1024)
	var req struct {
		JD string `json:"jd"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || len(req.JD) < 20 {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	draft, err := d.Store.GetDraftForUser(r.Context(), store.GetDraftForUserParams{ID: draftID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var tree model.Resume
	if err := json.Unmarshal(draft.Tree, &tree); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	job, err := d.Store.InsertJob(r.Context(), store.InsertJobParams{
		UserID: id.UserID, JdText: req.JD,
	})
	if err != nil {
		slog.Error("insert job", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	run, err := d.Store.InsertTailorRun(r.Context(), store.InsertTailorRunParams{
		DraftID: draft.ID, JobID: job.ID, BaseSha256: draft.SourceSha256, Model: tailor.PromptVersion,
	})
	if err != nil {
		slog.Error("insert tailor run", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	start := time.Now()
	changes, runErr := tailor.Run(r.Context(), w, &tree, req.JD, run.ID.String(), tailor.Deps{
		StreamText: d.Tailor.StreamText,
		Corpus:     draft.SourceTex,
		Now:        time.Now,
	})
	changesJSON, _ := json.Marshal(changes)
	status := "ready"
	if runErr != nil {
		status = "failed"
		slog.Warn("tailor run failed", "draft", draftID, "run", run.ID, "error", runErr.Error(), "elapsed_ms", time.Since(start).Milliseconds())
	}
	if err := d.Store.FinishTailorRun(r.Context(), store.FinishTailorRunParams{
		ID: run.ID, Status: status, Changes: changesJSON,
		LatencyMs: pgtype.Int4{Int32: int32(time.Since(start).Milliseconds()), Valid: true},
	}); err != nil {
		slog.Error("finish tailor run", "error", err)
	}
}

// tailorDecisions handles POST /api/tailor-runs/:id/decisions — writes back
// accept/reject per change.
func (d Deps) tailorDecisions(w http.ResponseWriter, r *http.Request) {
	id, ok := identity(w, r)
	if !ok {
		return
	}
	runID, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if d.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	var req struct {
		Decisions []struct {
			Index    int    `json:"index"`
			Decision string `json:"decision"`
		} `json:"decisions"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	run, err := d.Store.GetTailorRunForUser(r.Context(), store.GetTailorRunForUserParams{ID: runID, UserID: id.UserID})
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var changes []map[string]any
	if err := json.Unmarshal(run.Changes, &changes); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	for _, dcs := range req.Decisions {
		if dcs.Index < 0 || dcs.Index >= len(changes) {
			continue
		}
		if dcs.Decision != "accepted" && dcs.Decision != "rejected" {
			continue
		}
		changes[dcs.Index]["decision"] = dcs.Decision
	}
	data, _ := json.Marshal(changes)
	if err := d.Store.UpdateTailorRunChanges(r.Context(), store.UpdateTailorRunChangesParams{ID: run.ID, Changes: data}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
