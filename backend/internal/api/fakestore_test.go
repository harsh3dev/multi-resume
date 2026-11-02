package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/harsh3dev/multi-resume/backend/internal/store"
)

// fakeStore is an in-memory Store for handler tests.
type fakeStore struct {
	mu         sync.Mutex
	resumes    map[uuid.UUID]store.Resume
	owner      map[uuid.UUID]uuid.UUID // resumeID -> userID
	variations map[uuid.UUID]store.Variation
	drafts     map[uuid.UUID]store.Draft
	assets     []store.Asset
	failed     int
	runs       map[uuid.UUID]*store.TailorRun
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		resumes:    map[uuid.UUID]store.Resume{},
		owner:      map[uuid.UUID]uuid.UUID{},
		variations: map[uuid.UUID]store.Variation{},
		drafts:     map[uuid.UUID]store.Draft{},
	}
}

func (f *fakeStore) InsertResume(_ context.Context, arg store.InsertResumeParams) (store.Resume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := store.Resume{ID: uuid.New(), UserID: arg.UserID, Title: arg.Title, Compiler: arg.Compiler, Family: arg.Family, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	f.resumes[r.ID] = r
	f.owner[r.ID] = arg.UserID
	return r, nil
}

func (f *fakeStore) ListResumesByUser(_ context.Context, userID uuid.UUID) ([]store.Resume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.Resume{}
	for _, r := range f.resumes {
		if f.owner[r.ID] == userID {
			out = append(out, r)
		}
	}
	return out, nil
}

var errNotFound = errors.New("not found")

func (f *fakeStore) GetResumeForUser(_ context.Context, arg store.GetResumeForUserParams) (store.Resume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.resumes[arg.ID]
	if !ok || f.owner[arg.ID] != arg.UserID {
		return store.Resume{}, errNotFound
	}
	return r, nil
}

func (f *fakeStore) SoftDeleteResume(_ context.Context, arg store.SoftDeleteResumeParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.resumes[arg.ID]; !ok || f.owner[arg.ID] != arg.UserID {
		return errNotFound
	}
	delete(f.resumes, arg.ID)
	return nil
}

func (f *fakeStore) RenameResume(_ context.Context, arg store.RenameResumeParams) (store.Resume, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.resumes[arg.ID]
	if !ok || f.owner[arg.ID] != arg.UserID {
		return store.Resume{}, pgx.ErrNoRows
	}
	r.Title = arg.Title
	f.resumes[arg.ID] = r
	return r, nil
}

func (f *fakeStore) RenameVariation(_ context.Context, arg store.RenameVariationParams) (store.Variation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.variations[arg.ID]
	if !ok {
		return store.Variation{}, pgx.ErrNoRows
	}
	for _, o := range f.variations {
		if o.ID != arg.ID && o.ResumeID == v.ResumeID && o.Name == arg.Name {
			return store.Variation{}, &pgconn.PgError{Code: "23505"}
		}
	}
	v.Name = arg.Name
	f.variations[arg.ID] = v
	return v, nil
}

func (f *fakeStore) InsertDraft(_ context.Context, arg store.InsertDraftParams) (store.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := store.Draft{ID: uuid.New(), ResumeID: arg.ResumeID, VariationID: arg.VariationID, SourceTex: arg.SourceTex, SourceSha256: arg.SourceSha256, Tree: arg.Tree, LastGoodPdfKey: arg.LastGoodPdfKey, NeedsCompile: arg.NeedsCompile, UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	f.drafts[d.ID] = d
	return d, nil
}

func (f *fakeStore) GetDraftForUser(_ context.Context, arg store.GetDraftForUserParams) (store.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.drafts[arg.ID]
	if !ok || f.owner[d.ResumeID] != arg.UserID {
		return store.Draft{}, errNotFound
	}
	return d, nil
}

func (f *fakeStore) GetDraftByResumeForUser(_ context.Context, arg store.GetDraftByResumeForUserParams) (store.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.owner[arg.ResumeID] != arg.UserID {
		return store.Draft{}, errNotFound
	}
	// Prefer the default variation's draft, matching the SQL join on
	// variations.is_default; fixtures without variations fall back to any
	// draft of the resume.
	var fallback *store.Draft
	for _, d := range f.drafts {
		if d.ResumeID != arg.ResumeID {
			continue
		}
		d := d
		if v, ok := f.variations[d.VariationID]; ok && v.IsDefault {
			return d, nil
		}
		fallback = &d
	}
	if fallback != nil {
		return *fallback, nil
	}
	return store.Draft{}, errNotFound
}

// --- variations ---

func (f *fakeStore) InsertVariation(_ context.Context, arg store.InsertVariationParams) (store.Variation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.variations {
		if v.ResumeID == arg.ResumeID && v.Name == arg.Name {
			return store.Variation{}, &pgconn.PgError{Code: "23505"}
		}
	}
	v := store.Variation{ID: uuid.New(), ResumeID: arg.ResumeID, Name: arg.Name, IsDefault: arg.IsDefault, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	f.variations[v.ID] = v
	return v, nil
}

func (f *fakeStore) ListVariationsForUser(_ context.Context, arg store.ListVariationsForUserParams) ([]store.ListVariationsForUserRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.owner[arg.ResumeID] != arg.UserID {
		return nil, nil
	}
	out := []store.ListVariationsForUserRow{}
	for _, v := range f.variations {
		if v.ResumeID != arg.ResumeID {
			continue
		}
		row := store.ListVariationsForUserRow{ID: v.ID, ResumeID: v.ResumeID, Name: v.Name, IsDefault: v.IsDefault, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
		for _, d := range f.drafts {
			if d.VariationID == v.ID {
				id := d.ID
				row.DraftID = &id
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func (f *fakeStore) GetVariationForUser(_ context.Context, arg store.GetVariationForUserParams) (store.Variation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.variations[arg.ID]
	if !ok || f.owner[v.ResumeID] != arg.UserID {
		return store.Variation{}, errNotFound
	}
	return v, nil
}

func (f *fakeStore) GetDefaultVariationForResume(_ context.Context, arg store.GetDefaultVariationForResumeParams) (store.Variation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.variations {
		if v.ResumeID == arg.ResumeID && v.IsDefault && f.owner[arg.ResumeID] == arg.UserID {
			return v, nil
		}
	}
	return store.Variation{}, errNotFound
}

func (f *fakeStore) DeleteVariation(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.variations[id]; !ok {
		return errNotFound
	}
	delete(f.variations, id)
	// Drafts cascade with their variation.
	for did, d := range f.drafts {
		if d.VariationID == id {
			delete(f.drafts, did)
		}
	}
	return nil
}

func (f *fakeStore) CloneDraftForVariation(_ context.Context, arg store.CloneDraftForVariationParams) (store.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.drafts {
		if d.VariationID == arg.VariationID {
			clone := d
			clone.ID = uuid.New()
			clone.VariationID = arg.VariationID_2
			f.drafts[clone.ID] = clone
			return clone, nil
		}
	}
	return store.Draft{}, errNotFound
}

func (f *fakeStore) GetDraftByVariationForUser(_ context.Context, arg store.GetDraftByVariationForUserParams) (store.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.drafts {
		if d.VariationID == arg.VariationID && f.owner[d.ResumeID] == arg.UserID {
			return d, nil
		}
	}
	return store.Draft{}, errNotFound
}

func (f *fakeStore) InsertFailedUpload(_ context.Context, _ store.InsertFailedUploadParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed++
	return nil
}

func (f *fakeStore) InsertAsset(_ context.Context, arg store.InsertAssetParams) (store.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := store.Asset{ID: uuid.New(), ResumeID: arg.ResumeID, TexPath: arg.TexPath, StorageKey: arg.StorageKey, Sha256: arg.Sha256, Mime: arg.Mime, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	f.assets = append(f.assets, a)
	return a, nil
}

func (f *fakeStore) MergeResumeDraftsAssetMap(_ context.Context, _ store.MergeResumeDraftsAssetMapParams) error {
	return nil
}

func resumeFixture(id uuid.UUID) store.Resume {
	return store.Resume{ID: id, Title: "T", Compiler: "pdflatex", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
}

func draftFixture(id, resumeID uuid.UUID) store.Draft {
	return store.Draft{ID: id, ResumeID: resumeID, VariationID: uuid.New(), SourceTex: `x`, SourceSha256: "h", Tree: []byte(`{"schemaVersion":1,"parserVersion":"1","sourceHash":"h","meta":{"compiler":"pdflatex","documentClass":"article","family":null,"commands":[]},"header":null,"sections":[],"styles":[],"assets":[],"report":{"sections":0,"entries":0,"bullets":0,"readOnlyRegions":0,"coverage":0}}`), UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
}

func (f *fakeStore) UpdateDraftWithRevision(_ context.Context, arg store.UpdateDraftWithRevisionParams) (store.Draft, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.drafts[arg.ID]
	if !ok {
		return store.Draft{}, errNotFound
	}
	if d.Revision != arg.Revision {
		return store.Draft{}, pgx.ErrNoRows
	}
	d.SourceTex = arg.SourceTex
	d.SourceSha256 = arg.SourceSha256
	d.Tree = arg.Tree
	d.LastGoodPdfKey = arg.LastGoodPdfKey
	d.CompileError = arg.CompileError
	d.NeedsCompile = true
	d.Revision++
	f.drafts[arg.ID] = d
	return d, nil
}

func (f *fakeStore) InsertJob(_ context.Context, arg store.InsertJobParams) (store.Job, error) {
	return store.Job{ID: uuid.New(), UserID: arg.UserID, JdText: arg.JdText, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}, nil
}

func (f *fakeStore) InsertTailorRun(_ context.Context, arg store.InsertTailorRunParams) (store.TailorRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	run := store.TailorRun{ID: uuid.New(), DraftID: arg.DraftID, JobID: arg.JobID, BaseSha256: arg.BaseSha256, Model: arg.Model, Status: "running", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	if f.runs == nil {
		f.runs = map[uuid.UUID]*store.TailorRun{}
	}
	f.runs[run.ID] = &run
	return run, nil
}

func (f *fakeStore) FinishTailorRun(_ context.Context, arg store.FinishTailorRunParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.runs[arg.ID]; ok {
		r.Status = arg.Status
		r.Changes = arg.Changes
	}
	return nil
}

func (f *fakeStore) UpdateTailorRunChanges(_ context.Context, arg store.UpdateTailorRunChangesParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.runs[arg.ID]; ok {
		r.Changes = arg.Changes
	}
	return nil
}

func (f *fakeStore) GetTailorRunForUser(_ context.Context, arg store.GetTailorRunForUserParams) (store.TailorRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[arg.ID]
	if !ok {
		return store.TailorRun{}, errNotFound
	}
	// ownership: run → draft → resume → user
	d, ok := f.drafts[r.DraftID]
	if !ok || f.owner[d.ResumeID] != arg.UserID {
		return store.TailorRun{}, errNotFound
	}
	return *r, nil
}

func (f *fakeStore) SetDraftCompileResult(_ context.Context, arg store.SetDraftCompileResultParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.drafts[arg.ID]
	if !ok || f.owner[d.ResumeID] != arg.UserID {
		return 0, nil
	}
	if arg.LastGoodPdfKey.Valid {
		d.LastGoodPdfKey = arg.LastGoodPdfKey
	}
	d.CompileError = arg.CompileError
	if d.Revision == arg.Revision {
		d.NeedsCompile = arg.NeedsCompile
	} else {
		d.NeedsCompile = true
	}
	f.drafts[arg.ID] = d
	return 1, nil
}
