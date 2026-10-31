# Resume Editor

Upload your LaTeX resume, edit it through fields instead of code, tailor it to a job description with AI, and keep one version per job. The `.tex` is the truth; the JSON tree is an index over it; every edit is applied to both and re-verified by parsing.

## Stack

- **Frontend** — Next.js 15 (App Router) + Tailwind + shadcn/ui, Zustand editor state, dnd-kit, pdfjs-dist, CodeMirror 6, Supabase auth (`@supabase/ssr`), Vercel AI SDK (`useObject`) for the tailor stream.
- **Backend** — Go + chi; LaTeX tokenizer/parser/engine (double-apply verification), sandboxed `latexmk` compile (bubblewrap, `-no-shell-escape`, no network), Anthropic streaming, sqlc → Supabase Postgres, R2 for PDFs/images.
- **Infra** — Supabase (Postgres + Auth), Cloudflare R2, Anthropic API. Deploys: Fly.io (backend Docker) + Vercel (frontend).

## Layout

```
apps/web/     Next.js app (pages, editor components, lib)
backend/      Go server (cmd/server, internal/{latex,parser,textconv,engine,checks,compile,tailor,auth,store,storage,api,config}, db/{migrations,queries}, testdata)
tasks/        build loop, phase files, FINAL-BLOCKERS.md, ACCEPTANCE.md
```

## Local development

First time? Follow **[SETUP.md](SETUP.md)** — step-by-step from Supabase project to running app.

The stack needs a Supabase project (Google provider enabled) and an R2 bucket. Copy the env templates and fill them:

```sh
cp apps/web/.env.example apps/web/.env.local
cp backend/.env.example backend/.env
```

Then either run the pieces directly:

```sh
cd apps/web && pnpm install && pnpm dev        # Next.js on :3000
cd backend && go run ./cmd/server              # Go on :8080
goose -dir backend/db/migrations postgres "$DATABASE_URL" up
cd backend && sqlc generate && tygo generate   # after schema/type changes
```

or the whole thing in Docker (TeX Live included in the backend image):

```sh
docker compose up --build
```

## Tests

```sh
cd backend && go test ./...        # parser goldens, engine fuzz, auth matrix, API tests
cd apps/web && pnpm typecheck && pnpm lint
```

The parser/engine metric tables:

```sh
go test ./internal/checks/ -run TestAllFixturesRoundTrip -v   # 52-file round-trip
go test ./internal/engine/ -run TestEditabilityMetric -v -args -engine.metric
```

## Deploy

- **Backend**: `fly deploy` from `backend/` (Dockerfile includes TeX Live, poppler, bubblewrap; ≥1GB RAM). Set secrets per `backend/.env.example` via `fly secrets set`.
- **Frontend**: Vercel project rooted at `apps/web`; set `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `GO_BACKEND_URL` (the Fly app URL). Supabase Auth → Site URL = prod domain.

See `tasks/FINAL-BLOCKERS.md` for the one-time external setup checklist and the live-verification steps deferred from the build.
