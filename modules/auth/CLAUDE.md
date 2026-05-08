# CLAUDE.md (modules/auth)

This file extends the repo-root [`CLAUDE.md`](../../CLAUDE.md) with guidance specific to the `auth` module. The repo-root file remains the source of truth for cross-cutting conventions; only auth-local facts live here.

## What this module is

A first-party OAuth 2.0 / OIDC Authorization Server. **HTTP API only** — this is the one service in the monorepo that does not use gRPC. chi router, JSON over HTTP, request validation via `ozzo-validation/v4` + the `utilhttp.Validator` seam.

Primary spec: [`docs/system-design.md`](docs/system-design.md). §1 (scope), §2 (RFC list), §3 (endpoints), §6 (data model) before non-trivial work.

## Where to start reading

1. `docs/system-design.md` §1–§3 — scope, primary RFCs, endpoint catalog.
2. `src/cmd/api/main.go` — the most complete `main.go` in the repo and the canonical reference for env → DI → server → graceful shutdown wiring.
3. `src/route/handler.go` — chi router shape (always carries `/health` and `r.Use(r.Middlewares()...)` even when no middleware is registered). Adds `utilotel.HTTPMiddleware("auth-api")` after the seam — that's all the OTel wiring needed; chi v5 populates `http.Request.Pattern` and `utilotel`'s span-name formatter picks it up automatically (no per-router retag middleware required).
4. `src/route/login.go` — handler shape (`Decode → Service → Encode`); copy this when adding a new endpoint.
5. `src/service/login.go` + `src/service/login_test.go` — service layer + the `stubQuerier` pattern used to mock `db.Querier` (see `testing.md` §3).
6. `src/domain/token.go` — domain types (`UserID`, `Scope`, `Role`, `Expired` are domain primitives, not bare strings).

## Layout (auth-specific)

This module is **the canonical example** of the layered shape (`coding-standards.md` §1). When in doubt about where something belongs in audit or queue, check how auth structured the equivalent.

Distinctive bits:

- **No `route/grpc/`** and no proto. HTTP only.
- **Has `route/middleware/`** — the chi seam for cross-cutting middleware (Bearer token, logging, etc.). The `r.Use(r.Middlewares()...)` line in `handler.go` keeps the registration point in place even before middleware is wired.
- **Has `domain/` and `service/`** with real content (audit and queue do not yet).
- **Has tests across every layer** — `domain`, `service`, `route`, and `route/request` all have `*_test.go` companions. Use these as the reference when writing tests for audit/queue.

## Module-local conventions

- **One handler per file** under `route/`. Don't co-locate two unrelated handlers in `route/login.go`. The naming pattern is `route/<feature>.go` plus `route/request/<feature>.go`.
- **`response` struct + `newResponse(...)` helper per handler** (see `route/login.go:10-16`). Don't return `domain.*` types directly to the client — the JSON shape is a route concern.
- **Service depends on `db.Querier` (the sqlc interface), never on `*sql.DB`**. This is the seam unit tests rely on. Constructor takes the dependency positionally: `NewLoginService(repo db.Querier) LoginService`.
- **Cache (Redis)** is consumed from `shared/utilcache` directly in `cmd/api/main.go`. Do NOT introduce `auth/src/infra/cache/` — the convention is "cache lives in shared, prefixed at construction". Multiple `Cache` values per service (different prefixes/TTLs) are fine.
- **JSON quirk**: `shared/utilhttp/response.go` uses `goccy/go-json`; `request.go` uses stdlib `encoding/json`. Match the file you're editing — don't migrate one to the other inside an unrelated change (`coding-standards.md` §12).
- **Known stub**: `LoginService.Post` returns `&domain.Token{}` (an empty token) after credential check passes. This is Phase 0 scaffolding — the real token construction lands when `domain.NewAccessToken` / `NewRefreshToken` get hashing + JWT signing wired up. If you fix this, drop the empty-token assertion in `service/login_test.go` and assert on a populated token instead.

## Make targets specific to this module

| Target | What it does |
|---|---|
| `make auth` | build + up + migrate (`auth-api`, `auth-db`, `migrator`) |
| `make auth-down` | stop the auth compose stack |
| `make auth-build` / `auth-up` / `auth-migrate` | granular |
| `make auth-new-migrate` | shell into `migrator` and `atlas migrate new` |
| `make auth-sqlc-gen` | regenerate `infra/database/db/*.go` |

**Gotcha**: `make auth-up` does NOT start `auth_cache` (Redis), but `auth-api` Pings it on startup via `utilcache.NewClient` and exits if unreachable. Bring the cache up explicitly: `docker compose up -d auth_cache`. The `run-service` skill encapsulates this.

## When schema / queries change

- **Migration**: `make auth-new-migrate` → write SQL with `COMMENT ON TABLE` / `COMMENT ON COLUMN` annotations (mandated for every `CREATE TABLE` — `coding-standards.md` §10) → `make auth-migrate`.
- **Query**: edit `queries/login.sql` (or new file) → `make auth-sqlc-gen` → `cd modules/auth/src && go vet ./...`. `regen-sqlc` skill automates this.
- **Request type**: every new struct under `route/request/` MUST implement `Validate() error` — the `utilhttp.RequestBody[T]` generic constraint requires it. Copy `route/request/login.go` as the template (`ozzo-validation` `ValidateStruct` form).
