-- name: InsertResume :one
insert into resumes (user_id, title, compiler, family)
values ($1, $2, $3, $4)
returning id, user_id, title, compiler, family, created_at, deleted_at;

-- name: ListResumesByUser :many
select id, user_id, title, compiler, family, created_at, deleted_at
from resumes
where user_id = $1 and deleted_at is null
order by created_at desc;

-- name: GetResumeForUser :one
select id, user_id, title, compiler, family, created_at, deleted_at
from resumes
where id = $1 and user_id = $2 and deleted_at is null;

-- name: SoftDeleteResume :exec
update resumes set deleted_at = now()
where id = $1 and user_id = $2 and deleted_at is null;

-- name: RenameResume :one
update resumes set title = $3
where id = $1 and user_id = $2 and deleted_at is null
returning id, user_id, title, compiler, family, created_at, deleted_at;
