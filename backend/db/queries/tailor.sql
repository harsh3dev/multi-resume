-- name: InsertTailorRun :one
insert into tailor_runs (draft_id, job_id, base_sha256, model, status)
values ($1, $2, $3, $4, 'running')
returning id, draft_id, job_id, base_sha256, model, changes, status, latency_ms, input_tokens, output_tokens, created_at;

-- name: UpdateTailorRunChanges :exec
update tailor_runs set changes = $2
where id = $1;

-- name: FinishTailorRun :exec
update tailor_runs
set status = $2, changes = $3, latency_ms = $4, input_tokens = $5, output_tokens = $6
where id = $1;

-- name: GetTailorRunForUser :one
select t.id, t.draft_id, t.job_id, t.base_sha256, t.model, t.changes, t.status, t.latency_ms, t.input_tokens, t.output_tokens, t.created_at
from tailor_runs t
join drafts d on d.id = t.draft_id
join resumes r on r.id = d.resume_id
where t.id = $1 and r.user_id = $2 and r.deleted_at is null;
