-- +goose Up
-- Application schema for the Resume Editor. Identity lives in auth.users
-- (Supabase-managed, read-only here). RLS is enabled on every app table with
-- zero permissive policies: the auto-exposed Data API denies anon and
-- authenticated roles. Go connects as the postgres role, which bypasses RLS.

create table resumes (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references auth.users(id) on delete cascade,
  title       text not null,
  compiler    text not null,
  family      text,
  created_at  timestamptz not null default now(),
  deleted_at  timestamptz
);
create index resumes_user_id_idx on resumes (user_id);

create table assets (
  id           uuid primary key default gen_random_uuid(),
  resume_id    uuid not null references resumes(id) on delete cascade,
  tex_path     text not null,
  storage_key  text not null,
  sha256       text not null,
  mime         text not null,
  created_at   timestamptz not null default now()
);
create index assets_resume_id_idx on assets (resume_id);

create table jobs (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references auth.users(id) on delete cascade,
  title       text,
  company     text,
  jd_text     text not null,
  source_url  text,
  created_at  timestamptz not null default now()
);
create index jobs_user_id_idx on jobs (user_id);

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

create table drafts (
  id                 uuid primary key default gen_random_uuid(),
  resume_id          uuid not null unique references resumes(id) on delete cascade,
  base_version_id    uuid not null references versions(id),
  source_tex         text not null,
  source_sha256      text not null,
  tree               jsonb not null,
  asset_map          jsonb not null default '{}',
  last_good_pdf_key  text,
  compile_error      text,
  revision           int not null default 0,
  updated_at         timestamptz not null default now()
);

create table tailor_runs (
  id                uuid primary key default gen_random_uuid(),
  draft_id          uuid not null references drafts(id) on delete cascade,
  job_id            uuid not null references jobs(id),
  base_sha256       text not null,
  model             text not null,
  changes           jsonb not null default '[]',
  status            text not null,
  result_version_id uuid references versions(id),
  latency_ms        int,
  input_tokens      int,
  output_tokens     int,
  created_at        timestamptz not null default now()
);
create index tailor_runs_draft_created_idx on tailor_runs (draft_id, created_at desc);

create table pdf_cache (
  cache_key   text primary key,
  pdf_key     text not null,
  page_count  int not null,
  created_at  timestamptz not null default now()
);

create table failed_uploads (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null references auth.users(id) on delete cascade,
  source_tex  text not null,
  error_log   text not null,
  created_at  timestamptz not null default now()
);

-- RLS hardening: deny everything through the Data API. No policies are
-- created, so with RLS enabled the default is deny-all.
alter table resumes        enable row level security;
alter table assets         enable row level security;
alter table jobs           enable row level security;
alter table versions       enable row level security;
alter table drafts         enable row level security;
alter table tailor_runs    enable row level security;
alter table pdf_cache      enable row level security;
alter table failed_uploads enable row level security;

revoke all on all tables in schema public from anon, authenticated;
alter default privileges in schema public revoke all on tables from anon, authenticated;

-- +goose Down
drop table if exists failed_uploads;
drop table if exists pdf_cache;
drop table if exists tailor_runs;
drop table if exists drafts;
drop table if exists versions;
drop table if exists jobs;
drop table if exists assets;
drop table if exists resumes;
