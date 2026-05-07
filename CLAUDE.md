# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Layout

A Go workspace (`modules/go.work`, Go 1.26.0) containing four modules under `modules/`:

- `audit/src` — audit service, split into `cmd/api` and `cmd/worker` entry points
- `auth/src` — auth service (HTTP API only), backed by Postgres + Redis cache
- `queue/src` — queue service exposing a gRPC API (proto: `modules/queue/src/route/grpc/queue.proto`)
- `shared/src` — cross-service helpers: `utilhttp` (typed `AppError` + JSON request/response helpers), `utillog` (slog JSON logger), and `utilcache` (Redis client + prefixed Cache wrapper)

Each service module follows the same internal layout: `cmd/<binary>/main.go` wires env → infra → service → route, while the layers live under `domain/` (entities, inputs), `service/` (business logic over `db.Querier`), `infra/database/` (sqlc + migrations), and `route/` (chi HTTP router; gRPC for queue). Cache access (Redis) is provided by `shared/utilcache` and consumed directly from `cmd/<binary>/main.go` — no per-service `infra/cache/` directory.

Service-to-service calls go over an internal Docker bridge network (`compose.yml`). When working on `audit`, `make audit` also brings up `queue-api` because the audit worker is expected to consume from it.

## Toolchain

Three code-gen / migration tools are required and are pre-installed in the devcontainer (`.devcontainer/Dockerfile`):

- **sqlc** generates the `infra/database/db/` package from `migrations/*.sql` (schema) + `queries/*.sql` (queries). Regenerate with `make audit-sqlc-gen` / `make auth-sqlc-gen`. Do not edit files in `infra/database/db/` by hand — they are regenerated.
- **Atlas** runs migrations via the `migrator` container (see `.images/migrator/Dockerfile`). The `*-migrate` targets first run `migrate hash` (updates `atlas.sum`) and then `migrate apply` against the service DB.
- **golangci-lint** v2 config in `.golangci.yml`. Linters are grouped by intent: error handling (`errcheck` with `check-type-assertions` + `check-blank`, `errchkjson`, `nilerr`), resource handling (`bodyclose`, `rowserrcheck`, `sqlclosecheck`, `noctx`), exhaustiveness (`exhaustive` over `switch` + `map`), static analysis (`govet`, `staticcheck`, `unused`, `ineffassign`), and quality (`misspell` US, `gocritic`, `dupl`, `predeclared`, `nolintlint`, `gocheckcompilerdirectives`). Formatters: `gofmt` + `goimports`. `errcheck` / `errchkjson` / `dupl` are relaxed in `*_test.go` and `testutil/`; `noctx` is relaxed in `*_test.go`; `w.Write` is excluded globally. CI runs `make lint` and `make test` (`.github/workflows/ci-cd.yml`).

## Common Commands

All commands run from the repo root unless noted. The `MODS` variable in the `Makefile` is `auth audit queue shared` — module-wide targets iterate that list.

### Workspace-wide
- `make test` — runs `go test -v -coverprofile=coverage.out ./...` per module and prints the `total:` coverage line; HTML report written to `coverage.html`
- `make fmt` / `make vet` / `make lint` — per-module `go fmt`, `go vet`, `golangci-lint run`
- `make tidy` / `make update` — `go mod tidy` (and `go get -u ./...` for `update`) per module
- `make hooks` — installs git hooks at `.githooks/` (pre-commit: `fmt + vet + lint`; pre-push: `test`) and points `core.hooksPath` at it. Run after first checkout. `make hooks-uninstall` removes them.

### Running a single test
There is no make target for a single test. Run `go test` directly inside the module (each module is its own Go module):
```
cd modules/<service>/src && go test -run TestName ./path/to/pkg
```

### Per-service stack (Docker Compose)
- `make audit` ≡ `audit-build` + `audit-up` + `audit-migrate`. Brings up `audit-api`, `audit-worker`, `audit-db`, `queue-api`, `migrator`. `make audit-down` stops them.
- `make auth` ≡ `auth-build` + `auth-up` + `auth-migrate`. Brings up `auth-api`, `auth-db`, `migrator` (note: the auth API depends on the `auth_cache` Redis service in `compose.yml`, but `auth-up` does not start it — start it explicitly with `docker compose up -d auth_cache` if running auth-api locally).
- Postgres ports: audit `5432`, auth `5433`. Redis: `6379`. API containers expose `8080` internally.

### Migrations
- New migration: `make audit-new-migrate` or `make auth-new-migrate` (NOT `make new-migrate MODULE=...`). These shell into the `migrator` container and call `atlas migrate new`.
- Apply: `make audit-migrate` / `make auth-migrate`.
- Migration files live at `modules/<service>/src/infra/database/migrations/`.

### Cleanup
- `make rmi` — remove dangling Docker images
- `make rmv` — `docker volume prune`

## Conventions Worth Knowing

- **Errors across HTTP**: services return typed errors from `shared/utilhttp` (e.g. `NewDBError`, `NewUnauthorizedError`). The route layer calls `utilhttp.ResponseError(w, err)` which type-switches on `AppError.Type` to the right HTTP status. Add new error categories by extending `ErrorType` and the switch in both `error.go` and `response.go`.
- **Request decoding**: `utilhttp.RequestBody[T]` and `RequestUrlParam[T]` require `T` to implement `Validate() error` (the `Validator` interface). New request types under `route/request/` must satisfy this.
- **Logging**: call `utillog.NewLogger()` once in `init()` to install a JSON `slog` handler at debug level, then use `slog` directly.
- **Generated code**: `infra/database/db/*.go` (sqlc output) and the unimplemented gRPC stubs under `modules/queue/src/route/grpc/` are generated — modify the source (`.sql` queries, `.proto`) and regenerate.

## Detailed Rules

The following files contain the full coding standards and testing policy for this repo. Treat them as binding for any change you author here.

@.claude/rules/coding-standards.md
@.claude/rules/testing.md
