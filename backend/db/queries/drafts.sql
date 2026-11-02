-- name: InsertDraft :one
insert into drafts (resume_id, variation_id, source_tex, source_sha256, tree, asset_map, last_good_pdf_key, needs_compile)
values ($1, $2, $3, $4, $5, $6, $7, $8)
returning id, resume_id, source_tex, source_sha256, tree, asset_map, last_good_pdf_key, compile_error, revision, updated_at, variation_id, needs_compile;

-- name: GetDraftForUser :one
select d.id, d.resume_id, d.source_tex, d.source_sha256, d.tree, d.asset_map, d.last_good_pdf_key, d.compile_error, d.revision, d.updated_at, d.variation_id, d.needs_compile
from drafts d
join resumes r on r.id = d.resume_id
where d.id = $1 and r.user_id = $2 and r.deleted_at is null;

-- name: GetDraftByResumeForUser :one
-- Resolves the default ("Original") variation's draft for legacy callers.
select d.id, d.resume_id, d.source_tex, d.source_sha256, d.tree, d.asset_map, d.last_good_pdf_key, d.compile_error, d.revision, d.updated_at, d.variation_id, d.needs_compile
from drafts d
join variations v on v.id = d.variation_id and v.is_default
join resumes r on r.id = d.resume_id
where d.resume_id = $1 and r.user_id = $2 and r.deleted_at is null;

-- name: UpdateDraftWithRevision :one
update drafts
set source_tex = $3, source_sha256 = $4, tree = $5, asset_map = $6,
    last_good_pdf_key = $7, compile_error = $8, needs_compile = true,
    revision = revision + 1, updated_at = now()
where id = $1 and revision = $2
returning id, resume_id, source_tex, source_sha256, tree, asset_map, last_good_pdf_key, compile_error, revision, updated_at, variation_id, needs_compile;


-- name: MergeResumeDraftsAssetMap :exec
-- Images belong to the resume, so every variation's draft gets the new file.
update drafts
set asset_map = asset_map || $3::jsonb, needs_compile = true
where resume_id = $1
  and exists (select 1 from resumes r where r.id = $1 and r.user_id = $2 and r.deleted_at is null);

-- name: SetDraftCompileResult :execrows
-- Records a compile outcome without bumping the revision. The "needs compile"
-- flag is cleared only when the draft is still at the revision that was
-- compiled (a save that landed mid-compile keeps it set). A failed compile
-- passes a null pdf key, which keeps the previous good PDF.
update drafts d
set last_good_pdf_key = coalesce($4, d.last_good_pdf_key),
    compile_error = $5,
    needs_compile = case when d.revision = $3 then $6 else true end
from resumes r
where d.id = $1 and r.id = d.resume_id and r.user_id = $2 and r.deleted_at is null;
