-- +goose Up
-- Versions (frozen snapshots) are folded into variations: a version IS a
-- variation (a named, editable copy with its own draft). The snapshot
-- machinery is removed. THIS DESTROYS every row in `versions`, and with it
-- the metadata pointing at stored snapshot PDFs; the objects themselves stay
-- in the bucket and must be cleaned up separately if desired.

alter table drafts      drop column if exists base_version_id;
alter table tailor_runs drop column if exists result_version_id;
drop table if exists versions;

-- +goose Down
-- Recreates the empty structure only; snapshot data is not recoverable.
-- base_version_id comes back nullable because there is nothing to point at.
create table versions (
  id             uuid primary key default gen_random_uuid(),
  resume_id      uuid not null references resumes(id) on delete cascade,
  parent_id      uuid references versions(id),
  name           text not null,
  kind           text not null,
  job_id         uuid references jobs(id),
  source_tex     text not null,
  source_sha256  text not null,
  asset_map      jsonb not null default '{}',
  pdf_key        text not null,
  page_count     int not null,
  created_at     timestamptz not null default now()
);
create index versions_resume_created_idx on versions (resume_id, created_at desc);
alter table versions enable row level security;

alter table drafts      add column base_version_id uuid references versions(id);
alter table tailor_runs add column result_version_id uuid references versions(id);
