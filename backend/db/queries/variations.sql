-- name: InsertVariation :one
insert into variations (resume_id, name, is_default)
values ($1, $2, $3)
returning id, resume_id, name, is_default, created_at, updated_at;

-- name: ListVariationsForUser :many
select v.id, v.resume_id, v.name, v.is_default, v.created_at, v.updated_at,
       d.id as draft_id
from variations v
join resumes r on r.id = v.resume_id
left join drafts d on d.variation_id = v.id
where v.resume_id = $1 and r.user_id = $2 and r.deleted_at is null
order by v.created_at asc;

-- name: GetVariationForUser :one
select v.id, v.resume_id, v.name, v.is_default, v.created_at, v.updated_at
from variations v
join resumes r on r.id = v.resume_id
where v.id = $1 and r.user_id = $2 and r.deleted_at is null;

-- name: GetDefaultVariationForResume :one
select v.id, v.resume_id, v.name, v.is_default, v.created_at, v.updated_at
from variations v
join resumes r on r.id = v.resume_id
where v.resume_id = $1 and v.is_default and r.user_id = $2 and r.deleted_at is null;

-- name: DeleteVariation :exec
delete from variations where id = $1;

-- name: GetDraftByVariationForUser :one
select d.id, d.resume_id, d.source_tex, d.source_sha256, d.tree, d.asset_map, d.last_good_pdf_key, d.compile_error, d.revision, d.updated_at, d.variation_id, d.needs_compile
from drafts d
join resumes r on r.id = d.resume_id
where d.variation_id = $1 and r.user_id = $2 and r.deleted_at is null;

-- name: CloneDraftForVariation :one
insert into drafts (resume_id, variation_id, source_tex, source_sha256, tree, asset_map, last_good_pdf_key, needs_compile)
select d.resume_id, $2, d.source_tex, d.source_sha256, d.tree, d.asset_map, d.last_good_pdf_key, d.needs_compile
from drafts d
where d.variation_id = $1
returning id, resume_id, source_tex, source_sha256, tree, asset_map, last_good_pdf_key, compile_error, revision, updated_at, variation_id, needs_compile;

-- name: RenameVariation :one
update variations set name = $2, updated_at = now()
where id = $1
returning id, resume_id, name, is_default, created_at, updated_at;
