# AGENTS.md

## Quick start

- **Go workspace**: `modules/go.work` — 4 modules (`auth`, `audit`, `queue`, `shared`). Always `cd modules/<svc>/src` before `go test`, `go vet`, `go run`.
- **Full instructions**: Read `CLAUDE.md` first. Per-module: `modules/<svc>/CLAUDE.md`.
- **Rules**: `.claude/rules/coding-standards.md`, `.claude/rules/testing.md`, `.claude/rules/kubernetes-conventions.md`.

## Commands

```bash
make test          # workspace-wide: go test -v -coverprofile per module
make lint          # golangci-lint v2 per module
make fmt           # go fmt per module
make vet           # go vet per module
make tidy          # go mod tidy per module

cd modules/<svc>/src && go test -run TestName ./path/to/pkg   # single test

make auth          # Docker Compose: build + up + migrate
make audit         # Docker Compose: build + up + migrate (includes queue-api)
make auth-down / make audit-down

make k8s-up        # kind cluster + build + load + istio + apply
make k8s-down      # remove kustomization only
make k8s-cluster-delete

make audit-proto-gen / make queue-proto-gen   # regenerate gRPC stubs
make audit-sqlc-gen / make auth-sqlc-gen      # regenerate sqlc code
```

## Critical gotchas

### Error handling — NEVER use `%w`

- `utilhttp.ResponseError` uses a **type switch** on concrete wrappers, not `errors.Is`/`errors.As`.
- Typed wrappers (`NotFoundError`, etc.) embed `AppError` by value and do NOT implement `Unwrap()`.
- Always use `fmt.Errorf("...: %v", err)`.
- Adding a new error type requires edits in **both** `shared/src/utilhttp/error.go` and `shared/src/utilhttp/response.go`. Use the `add-error-type` skill.

### gRPC cross-service imports

- Only `<consumer>/src/infra/<svc>client/` may import `<producer>/route/grpc`.
- All other packages reference producer types via `type X = <svc>grpc.X` aliases in that wrapper.
- gRPC errors use `status.Error(codes.X, msg)` — `utilhttp.AppError` is HTTP-only.

### Generated code — never hand-edit

- `infra/database/db/*.go` → sqlc. Edit `queries/*.sql` then `make <svc>-sqlc-gen`.
- `route/grpc/*.pb.go` + `*_grpc.pb.go` → protoc. Edit `*.proto` then `make <svc>-proto-gen`.

### auth Redis gotcha

- `make auth-up` does NOT start `auth_cache`. Run `docker-compose up -d auth_cache` explicitly.

### JSON library inconsistency

- `shared/utilhttp/response.go` uses `goccy/go-json`; `request.go` uses stdlib `encoding/json`.
- Match the file you're editing — do not reconcile mid-PR.

### Kubernetes

- **dev**: `kubectl apply -k deploy/k8s/dev` — PERMISSIVE mTLS, in-cluster DB/Cache.
- **prd**: `kubectl apply -k deploy/k8s/prd` — STRICT mTLS, HTTPS Gateway, AuthorizationPolicy, ExternalName DB/Cache, PVC obs storage.
- Always `--load-restrictor=LoadRestrictionsNone` (otel kustomization reaches outside its base dir). `make k8s-apply` includes it.
- Auth is the only service with a Gateway (HTTPRoute → auth-api).

## Linting

- `.golangci.yml` v2. `errcheck` with `check-type-assertions` + `check-blank`.
- `exhaustive` checks both `switch` and `map`.
- `misspell` uses US locale.
- `*_test.go` and `testutil/` have relaxed rules for `errcheck`, `errchkjson`, `dupl`, `noctx`.

## Git hooks

- `make hooks` installs pre-commit (fmt + vet + lint) and pre-push (test) at `.githooks/`.

## Observability

- Each binary calls `utilotel.Init(ctx, "<service-name>")` early in `run()`.
- Falls back to **noop providers** when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset.
- Phase 3 (logs bridge) is paused — logs flow to stdout via `utillog` only.
- Use `slog.InfoContext(ctx, ...)` when a request context is available.
