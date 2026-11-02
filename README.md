# Multi Resume

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
cd apps/web && bun install && bun run dev        # Next.js on :3000
cd backend && go run ./cmd/server              # Go on :8080
cd backend && make build                       # binaries -> backend/out/ (server, parse)
goose -dir backend/db/migrations postgres "$DATABASE_URL" up
cd backend && sqlc generate && tygo generate   # after schema/type changes
```

> **Supabase note:** migrations must use the **session-mode pooler** URL
> (pooler host, port `5432`), not transaction mode (`:6543`) — goose's
> prepared statements fail under transaction mode. The direct
> `db.<ref>.supabase.co` host is IPv6-only. If `DATABASE_URL` points at
> `:6543`, run:
>
> ```sh
> goose -dir backend/db/migrations postgres "$(grep -m1 '^DATABASE_URL=' backend/.env | cut -d= -f2- | sed 's/:6543\//:5432\//')" up
> ```

or the whole thing in Docker (TeX Live included in the backend image):

```sh
docker compose up --build
```

## Tests

```sh
cd backend && go test ./...        # parser goldens, engine fuzz, auth matrix, API tests
cd apps/web && bun run typecheck && bun run lint
```

The parser/engine metric tables:

```sh
go test ./internal/checks/ -run TestAllFixturesRoundTrip -v   # 52-file round-trip
go test ./internal/engine/ -run TestEditabilityMetric -v -args -engine.metric
```

## Deploy

- **Backend API**: `backend/Dockerfile.server` (static Go binary, no TeX). Set secrets per `backend/.env.example`, plus `COMPILER_URL`/`COMPILER_AUTH`.
- **Compiler service**: `backend/Dockerfile.compiler` (TeX Live + `cmd/compiler`), deployed separately and scaled on its own — see `docs/compiler-service.md` (Cloud Run steps and the security checklist).
- `backend/Dockerfile` is the all-in-one image for local Docker use (in-process bubblewrap compiles).
- **Frontend**: Vercel project rooted at `apps/web`; set `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `GO_BACKEND_URL` (the Fly app URL). Supabase Auth → Site URL = prod domain.

See `tasks/FINAL-BLOCKERS.md` for the one-time external setup checklist and the live-verification steps deferred from the build.
