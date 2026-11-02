-- +goose Up
-- Saving a draft no longer compiles it. needs_compile marks drafts whose
-- stored PDF does not reflect the saved source (edits, source saves, image
-- uploads) or that have never compiled successfully. Cleared only by a
-- successful POST /api/drafts/:id/compile.
alter table drafts add column needs_compile boolean not null default true;
update drafts set needs_compile = false
where last_good_pdf_key is not null and compile_error is null;

-- +goose Down
alter table drafts drop column if exists needs_compile;
