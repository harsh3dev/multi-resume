-- +goose Up
-- Variations: a resume has many named variations; each variation owns exactly
-- one draft. Uploads create a default "Original" variation. The default
-- variation cannot be deleted, so a resume always keeps at least one
-- variation. The old drafts.resume_id UNIQUE (1:1) is replaced by a UNIQUE
-- on drafts.variation_id.

create table variations (
  id          uuid primary key default gen_random_uuid(),
  resume_id   uuid not null references resumes(id) on delete cascade,
  name        text not null,
  is_default  boolean not null default false,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  unique (resume_id, name)
);
create index variations_resume_idx on variations (resume_id);
-- At most one default variation per resume.
create unique index variations_one_default_idx on variations (resume_id) where is_default;

-- Backfill: every resume gets a default "Original" variation, including
-- soft-deleted ones (their drafts still exist and must not stay NULL;
-- all queries filter resumes.deleted_at anyway).
insert into variations (resume_id, name, is_default)
select id, 'Original', true from resumes;

alter table drafts add column variation_id uuid references variations(id) on delete cascade;
update drafts d set variation_id = v.id
  from variations v
  where v.resume_id = d.resume_id and v.is_default;
alter table drafts alter column variation_id set not null;
alter table drafts drop constraint if exists drafts_resume_id_key;
create unique index drafts_variation_id_idx on drafts (variation_id);

alter table variations enable row level security;

-- +goose Down
-- Restoring the 1:1 constraint fails if a resume has more than one draft;
-- that is expected when rolling back after variations were used.
alter table drafts drop column if exists variation_id;
alter table drafts add constraint drafts_resume_id_key unique (resume_id);
drop table if exists variations;
