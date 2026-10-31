-- name: InsertDraft :one
insert into drafts (resume_id, variation_id, source_tex, source_sha256, tree, asset_map, last_good_pdf_key)
values ($1, $2, $3, $4, $5, $6, $7)
returning id, resume_id, source_tex, source_sha256, tree, asset_map, last_good_pdf_key, compile_error, revision, updated_at, variation_id;

-- name: GetDraftForUser :one
select d.id, d.resume_id, d.source_tex, d.source_sha256, d.tree, d.asset_map, d.last_good_pdf_key, d.compile_error, d.revision, d.updated_at, d.variation_id
from drafts d
join resumes r on r.id = d.resume_id
where d.id = $1 and r.user_id = $2 and r.deleted_at is null;

-- name: GetDraftByResumeForUser :one
-- Resolves the default ("Original") variation's draft for legacy callers.
select d.id, d.resume_id, d.source_tex, d.source_sha256, d.tree, d.asset_map, d.last_good_pdf_key, d.compile_error, d.revision, d.updated_at, d.variation_id
from drafts d
join variations v on v.id = d.variation_id and v.is_default
join resumes r on r.id = d.resume_id
where d.resume_id = $1 and r.user_id = $2 and r.deleted_at is null;

-- name: UpdateDraftWithRevision :one
update drafts
set source_tex = $3, source_sha256 = $4, tree = $5, asset_map = $6,
    last_good_pdf_key = $7, compile_error = $8, revision = revision + 1, updated_at = now()
where id = $1 and revision = $2
returning id, resume_id, source_tex, source_sha256, tree, asset_map, last_good_pdf_key, compile_error, revision, updated_at, variation_id;

