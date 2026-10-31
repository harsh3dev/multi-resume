-- name: InsertAsset :one
insert into assets (resume_id, tex_path, storage_key, sha256, mime)
values ($1, $2, $3, $4, $5)
returning id, resume_id, tex_path, storage_key, sha256, mime, created_at;

-- name: ListAssetsByResume :many
select a.id, a.resume_id, a.tex_path, a.storage_key, a.sha256, a.mime, a.created_at
from assets a
join resumes r on r.id = a.resume_id
where a.resume_id = $1 and r.user_id = $2 and r.deleted_at is null;

-- name: GetPDFCache :one
select cache_key, pdf_key, page_count, created_at
from pdf_cache
where cache_key = $1;

-- name: UpsertPDFCache :exec
insert into pdf_cache (cache_key, pdf_key, page_count)
values ($1, $2, $3)
on conflict (cache_key) do update set pdf_key = $2, page_count = $3;

-- name: InsertFailedUpload :exec
insert into failed_uploads (user_id, source_tex, error_log)
values ($1, $2, $3);

-- name: InsertJob :one
insert into jobs (user_id, title, company, jd_text, source_url)
values ($1, $2, $3, $4, $5)
returning id, user_id, title, company, jd_text, source_url, created_at;
