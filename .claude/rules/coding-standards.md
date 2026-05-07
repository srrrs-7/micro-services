# Coding Standards

Project-specific Go conventions for this microservices monorepo. Every rule below is grounded in existing code — citations point to the canonical example. Generic Go style (effective-go, Uber style guide) is assumed; only deviations and project-specific patterns are listed.

## 1. Module & package layout

Every service module follows this fixed layout. Do not invent new top-level directories without a clear reason.

```
modules/<service>/src/
├── go.mod                       # module name == <service>
├── cmd/<binary>/main.go         # env, DI, server, graceful shutdown only
├── domain/                      # value objects + input structs (no I/O)
├── service/                     # business logic; depends on db.Querier interface
├── route/                       # HTTP/gRPC adapter
│   ├── handler.go               # router setup
│   ├── service.go               # interfaces the route depends on
│   ├── <feature>.go             # one handler per feature
│   ├── request/<feature>.go     # request structs with Validate()
│   └── middleware/              # chi middleware
└── infra/
    └── database/
        ├── database.go          # *sql.DB constructor
        ├── migrations/*.sql     # Atlas-managed
        ├── queries/*.sql        # sqlc input
        ├── sqlc.yaml
        └── db/                  # GENERATED — never hand-edit
```

Redis is consumed from `shared/utilcache` directly in `cmd/<binary>/main.go` — there is intentionally no per-service `infra/cache/` directory.

Reference: `modules/auth/src/` is the most complete example. `audit/` is structurally similar but mostly stub.

## 2. Imports

- Group order (gofmt + goimports auto-handles this): stdlib, then a single blank line, then third-party + local. See `modules/auth/src/cmd/api/main.go:3-21`.
- Local imports use the module name as root: `auth/domain`, `auth/route/request`, `shared/utilhttp`. Never use full repo paths.
- Cross-service imports (`audit` → `auth`, etc.) are forbidden. Use `shared/...` for common code; service-to-service contracts go via gRPC.

## 3. Error handling

This codebase has a **specific, non-standard** error idiom. Read this section carefully.

### 3.1 No error wrapping

The codebase uses `fmt.Errorf("context: %v", err)` — value formatting, NOT `%w`. Therefore:
- Do **NOT** use `errors.Is` / `errors.As` for unwrapping internal errors.
- `utilhttp.ResponseError` discriminates on the concrete typed wrapper via a type switch (`case BadRequestError:`, `case DBError:`, …), not via `errors.As`. The wrappers embed `AppError` by value and do not implement `Unwrap`, so `errors.As` would not extract them.
- The only sanctioned `errors.As` in the codebase is in tests, asserting that an error is exactly the concrete wrapper type (e.g. `errors.As(err, &utilhttp.DBError{})`). It works there because the target and dynamic types are identical, not because of unwrapping.
- Comparing sentinel errors uses `==`: `if err != http.ErrServerClosed` (`auth/cmd/api/main.go:94`), not `errors.Is`.

If you introduce `%w` wrapping, you change the contract — discuss before doing so.

### 3.2 Crossing layer boundaries

| Layer returning the error | What it returns |
|---|---|
| `infra/` (DB, cache) | Raw error (`*sql.DB` errors, `redis` errors) |
| `service/` | `utilhttp.New*Error(fmt.Errorf("...: %v", err))` — typed wrapper |
| `route/` | Calls `utilhttp.ResponseError(w, err)` which type-switches to HTTP status |
| `cmd/.../main.go` | Logs via `slog.Error` and `os.Exit(1)` |

Reference: `modules/auth/src/service/login.go:21-27`.

### 3.3 The error catalog

`shared/utilhttp/error.go` is the single source of truth. Available factories:

`NewNotFoundError`, `NewBadRequestError`, `NewInternalServerError`, `NewUnauthorizedError`, `NewForbiddenError`, `NewConflictError`, `NewTooManyRequestsError`, `NewDBError`.

Adding a new category requires synchronized edits in `error.go` and `response.go` — use the `add-error-type` skill.

### 3.4 `init()` and `main()`

Errors during startup print via `slog.Error` then `os.Exit(1)`. The pattern is `main` calls `run() error`; `run` does the work and returns; `main` is the only function that calls `os.Exit`. See `auth/cmd/api/main.go:60-65`.

## 4. Naming

- **Module name == service name**: `module auth` in `auth/src/go.mod`.
- **Constructor functions**: `New<Type>(...)` returning the value (not pointer) for small/immutable types (`NewUser`, `NewLoginInput`, `NewAccessToken`); pointer for resource-holders (`NewDB` returns `*sql.DB`, `NewCache` returns `*redis.Client`). Match the existing pattern for the type you're adding.
- **Receivers**: 1–2 letter, lowercase. Value receivers for stateless services (`s LoginService`); pointer receivers for handlers and resource-bearing structs (`h *handler`).
- **Interfaces**: lowercase if package-private (`loginService` in `auth/route/service.go`); exported when crossing packages (`utilhttp.Validator`, `db.Querier`).
- **Domain types over primitives**: `type UserID string`, `type Scope string`, `type Role string`, `type Expired time.Time`. Use these instead of bare strings/timestamps in domain code (`auth/domain/token.go`).
- **Env-var constants**: `EnvDbUrl = "DB_URL"`, declared in a `const (...)` block at the top of `main.go` (`auth/cmd/api/main.go:23-27`).

## 5. Configuration

All config is from environment variables, parsed once in `main.go`:

```go
type env struct { dbUrl, cacheAddr, cachePrefix string }

func newEnv() env { return env{ dbUrl: os.Getenv(EnvDbUrl), ... } }

func (e env) validate() error {
    if e.dbUrl == "" { return fmt.Errorf("empty env %s", EnvDbUrl) }
    ...
}
```

When adding a required env var, you MUST add it to BOTH `newEnv` and `validate`. No defaults — missing config is a startup error.

## 6. Logging

- `utillog.NewLogger()` is called once in `init()` of each binary's `main.go`. It installs a JSON `slog` handler at `slog.LevelDebug`.
- Use `slog` directly throughout: `slog.Info("starting server on port 8080")`, `slog.Error("failed to run application", "error", err)`.
- Key-value pairs: lowercase keys, the error key is `"error"`. See `auth/cmd/api/main.go:62, 95`.
- Never use `fmt.Println`/`log.Println` in production code. (Stub `cmd/api/main.go` files in `audit` and `queue` currently use `fmt.Println` as placeholders — replace, don't propagate.)

## 7. HTTP layer (chi)

- One `chi.Mux` per service, built in `route/handler.go` via `(h *handler).Router()`.
- Always expose `GET /health` returning 200 with empty body (`auth/route/handler.go:22`).
- Versioned routes: `r.Route("/<service>/v1", ...)`.
- Middleware seam: `r.Use(r.Middlewares()...)` — keep this line even if no middleware is registered yet; future cross-cutting middleware plugs in here.
- Handler shape (see `auth/route/login.go:18-32`):

```go
func (h *handler) feature(w http.ResponseWriter, r *http.Request) {
    req, err := utilhttp.RequestBody[request.FeatureRequest](r)
    if err != nil { utilhttp.ResponseError(w, err); return }

    result, err := h.svc.DoThing(r.Context(), domain.NewInput(req.X))
    if err != nil { utilhttp.ResponseError(w, err); return }

    utilhttp.ResponseOk(w, newResponse(result))
}
```

- Per-handler `response` struct + `newResponse` helper for the JSON body (`login.go:10-16`). Don't return `domain.*` types directly to the client.

## 8. Request validation

- Every request struct under `route/request/` MUST implement `Validate() error`. The `utilhttp.RequestBody[T]` generic constraint requires it.
- Use `ozzo-validation/v4` with `validation.ValidateStruct`:

```go
func (r LoginRequest) Validate() error {
    return validation.ValidateStruct(&r,
        validation.Field(&r.Email, validation.Required, validation.Length(5, 100)),
        validation.Field(&r.Password, validation.Required, validation.Length(6, 100)),
    )
}
```

Reference: `auth/route/request/login.go`.

## 9. Service layer

- Depends on the **interface** `db.Querier` (sqlc-generated when `emit_interface: true`), never on `*sql.DB` directly. This is the seam for unit testing.
- Constructor takes the dependencies as positional params: `NewLoginService(repo db.Querier) LoginService` (`auth/service/login.go:15`).
- Returns domain types (`*domain.Token`), wraps errors with `utilhttp.New*Error`.
- Never imports `chi`, `http`, or anything from `route/` — dependency direction is route → service, never the reverse.

## 10. Database access

- Schema in `migrations/*.sql`, queries in `queries/*.sql`, generated Go in `db/*.go`. The `db/` directory is **regenerated** — every file there starts with `// Code generated by sqlc. DO NOT EDIT.`
- SQL style (see existing migrations and queries):
  - Migrations: rich `COMMENT ON TABLE` / `COMMENT ON COLUMN` annotations after every `CREATE TABLE`. New schema must follow.
  - Queries: each clause on its own line, indented; positional params (`$1, $2`); sqlc command comment immediately above (`-- name: GetUser :one`).
- After editing `queries/` or `migrations/`, run `make <service>-sqlc-gen` then `cd modules/<service>/src && go vet ./...`.

## 11. Graceful shutdown

Every long-running binary must:
1. Listen on `os.Interrupt` and `syscall.SIGTERM` via `signal.NotifyContext`.
2. Use a 30-second `context.WithTimeout` for shutdown.
3. Stop the server first, then close resources in reverse-allocation order (cache before DB, since the cache may need DB for its last writes — actually the existing code goes `cache.Close()` then `db.Close()`, but the comment says "順序重要"; keep DB last).

Reference: `auth/cmd/api/main.go:99-132`.

## 12. JSON

- **Known inconsistency** in the codebase: `shared/utilhttp/response.go` uses `github.com/goccy/go-json`, but `shared/utilhttp/request.go` uses stdlib `encoding/json`. Both work. Until reconciled, **match the file you're editing** — don't introduce a third option, and don't unilaterally migrate one to the other without a separate cleanup PR.
- All response structs use `json:"..."` tags, even on unexported helpers (sqlc emits them via `emit_json_tags: true` and the codebase follows suit).

## 13. Comments

- The codebase comments **sparingly**. Most exported types and functions have no doc comment.
- When you do comment:
  - Use Japanese or English freely — both are present and accepted (see `shared/utilhttp/error.go:45` for Japanese; SQL `COMMENT ON ...` for English).
  - Section markers in `main.go`: `// ===== DI =====`, `// ===== start server =====`, `// ===== graceful shutdown =====`. Use this style only in `main.go` files where wiring sections benefit from visual separation.
  - Do not add doc comments just for lint compliance — the lint config does not require them.

## 14. Generated code

Files explicitly off-limits for hand edits:
- `modules/<service>/src/infra/database/db/*.go` — sqlc output
- `modules/<service>/src/infra/database/migrations/atlas.sum` — Atlas hashes
- (Future) `modules/queue/src/route/grpc/*.pb.go` — protoc output

Edit the source (`queries/*.sql`, `migrations/*.sql`, `*.proto`) and regenerate.

## 15. Linting

Pre-commit hook runs `make fmt && make vet && make lint`; pre-push hook runs `make test`. CI re-runs `make lint` and `make test` (`.github/workflows/ci-cd.yml`). The lint config (`.golangci.yml`) groups linters by intent:

- **Error handling (must)**: `errcheck` (with `check-type-assertions: true` and `check-blank: true`), `errchkjson`, `nilerr`.
- **Resource handling (must)**: `bodyclose`, `rowserrcheck`, `sqlclosecheck`, `noctx`.
- **Exhaustiveness (must)**: `exhaustive` over both `switch` and `map` literals; `default-signifies-exhaustive: true`.
- **Static analysis**: `govet`, `staticcheck`, `unused`, `ineffassign`.
- **Quality**: `misspell` (US), `gocritic`, `dupl`, `predeclared`, `nolintlint`, `gocheckcompilerdirectives`.
- **Formatters** (run via `golangci-lint fmt`/`run`): `gofmt`, `goimports`.

Project-specific implications:

- **`exhaustive`**: any `switch` (or map literal) over a defined enum (`ErrorType`, future role enums) must cover every constant or have a `default`.
- **`errcheck check-type-assertions: true`**: `x.(T)` without comma-ok is a lint error. Always write `x, ok := y.(T)`.
- **`errcheck check-blank: true`**: `_ = f()` for an error-returning `f` is a lint error. Either handle the error or document why blanking is correct via a `//nolint:errcheck // <reason>` directive (which `nolintlint` will validate).
- **`noctx`**: HTTP requests and `db.Ping`/`db.Exec` must use the `*Context` variants. Tests using `httptest.NewRequest` are exempted by path.
- **`nolintlint`**: every `//nolint:foo` must name the linter and include `// <explanation>`.

`errcheck`, `errchkjson`, and `dupl` are relaxed in `*_test.go` and `testutil/`. `noctx` is relaxed in `*_test.go`. `(*http.ResponseWriter).Write` is excluded from `errcheck` globally — there is nothing to do if the response write fails.

When a lint exception is genuinely needed, prefer narrowing the rule in `.golangci.yml` (path-based) over `//nolint` directives in code.
