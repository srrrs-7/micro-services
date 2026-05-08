# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Layout

A Go workspace (`modules/go.work`, Go 1.26.0) containing four modules under `modules/`:

- `audit/src` — audit service exposing a gRPC API (proto: `modules/audit/src/route/grpc/audit.proto`), split into `cmd/api` and `cmd/worker` entry points
- `auth/src` — auth service (HTTP API only), backed by Postgres + Redis cache
- `queue/src` — queue service exposing a gRPC API (proto: `modules/queue/src/route/grpc/queue.proto`)
- `shared/src` — cross-service helpers: `utilhttp` (typed `AppError` + JSON request/response helpers), `utillog` (slog JSON logger), `utilcache` (Redis client + prefixed `Cache` wrapper), `utilgrpc` (gRPC `Dial` with functional options for TLS / interceptors + an outbound logging interceptor), and `utilotel` (`Init` for the global TracerProvider + MeterProvider, `HTTPMiddleware` for chi, `GRPCServerOption` / `GRPCClientOption` for the otelgrpc StatsHandler). Service-specific gRPC client wrappers do NOT live here — see the gRPC consumer pattern below.

Each service module follows the same internal layout: `cmd/<binary>/main.go` wires env → infra → service → route, while the layers live under `domain/` (entities, inputs), `service/` (business logic over `db.Querier`), `infra/database/` (sqlc + migrations), and `route/` (chi HTTP router for `auth`; gRPC `server.go` + `handler.go` + `interceptor/` for `audit` / `queue`). Cache access (Redis) is provided by `shared/utilcache` and consumed directly from `cmd/<binary>/main.go` — no per-service `infra/cache/` directory. Every binary calls `utilotel.Init(ctx, "<service-name>")` early in `run()` and flushes the returned shutdown ahead of DB / cache close so in-flight spans + metrics reach the Collector while the network is still up.

Service-to-service calls go over an internal Docker bridge network (`compose.yml`). When working on `audit`, `make audit` also brings up `queue-api` because the audit worker is expected to consume from it.

## Toolchain

The following tools are required and are pre-installed in the devcontainer (`.devcontainer/Dockerfile`):

- **sqlc** generates the `infra/database/db/` package from `migrations/*.sql` (schema) + `queries/*.sql` (queries). Regenerate with `make audit-sqlc-gen` / `make auth-sqlc-gen`. Do not edit files in `infra/database/db/` by hand — they are regenerated.
- **Atlas** runs migrations via the `migrator` container (see `.images/migrator/Dockerfile`). The `*-migrate` targets first run `migrate hash` (updates `atlas.sum`) and then `migrate apply` against the service DB.
- **golangci-lint** v2 config in `.golangci.yml`. Linters are grouped by intent: error handling (`errcheck` with `check-type-assertions` + `check-blank`, `errchkjson`, `nilerr`), resource handling (`bodyclose`, `rowserrcheck`, `sqlclosecheck`, `noctx`), exhaustiveness (`exhaustive` over `switch` + `map`), static analysis (`govet`, `staticcheck`, `unused`, `ineffassign`), and quality (`misspell` US, `gocritic`, `dupl`, `predeclared`, `nolintlint`, `gocheckcompilerdirectives`). Formatters: `gofmt` + `goimports`. `errcheck` / `errchkjson` / `dupl` are relaxed in `*_test.go` and `testutil/`; `noctx` is relaxed in `*_test.go`; `w.Write` is excluded globally. CI runs `make lint` and `make test` (`.github/workflows/ci-cd.yml`).
- **kubectl + kind** for the local Kubernetes setup (see "Kubernetes deployment" below). The dev container shares the host Docker socket via `.devcontainer/compose.yaml`, so `kind` runs its node containers on the host's daemon.
- **protoc 28.3 + protoc-gen-go v1.34.2 + protoc-gen-go-grpc v1.5.1** generate the gRPC stubs (`*.pb.go`, `*_grpc.pb.go`) under each service's `route/grpc/`. Regenerate with `make queue-proto-gen` / `make audit-proto-gen`. Bumping versions also requires editing `.devcontainer/Dockerfile` and `Makefile`'s `PROTOC_INCLUDE`.

## Common Commands

All commands run from the repo root unless noted. The `MODS` variable in the `Makefile` is `auth audit queue shared` — module-wide targets iterate that list.

### Workspace-wide
- `make test` — runs `go test -v -coverprofile=coverage.txt ./...` per module and prints the `total:` coverage line; HTML report written to `coverage.html`
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
- `make auth` ≡ `auth-build` + `auth-up` + `auth-migrate`. Brings up `auth-api`, `auth-db`, `migrator` (note: the auth API depends on the `auth_cache` Redis service in `compose.yml`, but `auth-up` does not start it — start it explicitly with `docker-compose up -d auth_cache` if running auth-api locally; v1 hyphenated `docker-compose` is what the Makefile uses everywhere).
- Postgres host ports: audit `5434` (avoids the very common host-:5432 collision), auth `5433`. Container internal port stays 5432 so service-to-service DNS is unaffected. Redis: `6379`. API containers expose `8080` internally.

### gRPC code generation
- `make queue-proto-gen` / `make audit-proto-gen` regenerate one service's `*.pb.go` + `*_grpc.pb.go` from the `.proto` next to them.

### Migrations
- New migration: `make audit-new-migrate` or `make auth-new-migrate` (NOT `make new-migrate MODULE=...`). These shell into the `migrator` container and call `atlas migrate new`.
- Apply: `make audit-migrate` / `make auth-migrate`.
- Migration files live at `modules/<service>/src/infra/database/migrations/`.

### Cleanup
- `make down` — stop & remove **every** container in this Compose project (services + obs stack). Named volumes preserved.
- `make nuke` — `make down` + drop named volumes (DB / Prometheus / Tempo / Loki / Grafana data). Use when host disk is tight and per-project state can go.
- `make rmi` — remove dangling Docker images
- `make rmv` — `docker volume prune`

### Observability stack (Docker Compose)
- `make obs-up` — start the OTel Collector + Prometheus + Tempo + Loki + Grafana stack (Compose `-f compose.yml -f otel/compose.yml`, opt-in — does NOT touch service stacks). Prints the `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317` line to export when re-running a service stack so telemetry actually flows. Grafana publishes to host `:3001` to avoid collisions with the typical dev-server `:3000`.
- `make obs-down` — stop & remove the obs containers (data volumes preserved).
- `make obs-logs` — tail otel-collector logs.
- `make obs-status` — `ps` of the five obs containers.

### Kubernetes (local kind)
- `make k8s-up` — full lifecycle: create kind cluster `dev` if missing → build all `:dev` images → `kind load docker-image` each → `kubectl kustomize --load-restrictor=LoadRestrictionsNone deploy/k8s/dev | kubectl apply -f -` → wait for DBs/cache/migrate Jobs → print status. Brings up the obs stack (collector / prom / tempo / loki / grafana) alongside the services. The `--load-restrictor=LoadRestrictionsNone` flag is required because the obs base kustomization references files outside its directory (`otel/k8s/base/` → `otel/<component>/*` via `configMapGenerator`).
- `make k8s-status` — pods/services/jobs across the `audit`, `auth`, `queue`, `observability` namespaces.
- `make k8s-down` — delete the kustomization (cluster stays up). `make k8s-cluster-delete` removes the kind cluster entirely.
- Granular: `make k8s-cluster` / `k8s-build` / `k8s-load` / `k8s-apply`.

## Conventions Worth Knowing

- **Errors across HTTP**: services return typed errors from `shared/utilhttp` (e.g. `NewDBError`, `NewUnauthorizedError`). The route layer calls `utilhttp.ResponseError(w, err)` which type-switches on `AppError.Type` to the right HTTP status. Add new error categories by extending `ErrorType` and the switch in both `error.go` and `response.go`.
- **Request decoding**: `utilhttp.RequestBody[T]` and `RequestUrlParam[T]` require `T` to implement `Validate() error` (the `Validator` interface). New request types under `route/request/` must satisfy this.
- **Logging**: call `utillog.NewLogger()` once in `init()` to install a JSON `slog` handler at debug level, then use `slog` directly. Inside handlers and interceptors that have a request `context.Context` (HTTP / gRPC), prefer `slog.InfoContext(ctx, ...)` over `slog.Info(...)` — the call is one keystroke longer but lets a future log-export bridge auto-attach `trace_id` / `span_id` from the active span.
- **gRPC consumer pattern**: each consumer module that needs to call another service over gRPC owns a `<consumer>/src/infra/<svc>client/` wrapper (canonical example: `audit/src/infra/queueclient/`). The wrapper is the **only** place inside that consumer that may import `<producer>/route/grpc` cross-service — every other package in the consumer references producer types via the wrapper's type aliases. The wrapper also re-exports `utilgrpc.Option` so callers don't reach for `shared/utilgrpc` or `google.golang.org/grpc`. When proto adds a new message that the consumer must name, add a matching `type X = <svc>grpc.X` alias in the wrapper. See `coding-standards.md` §2 for the contract-surface exemption that makes the cross-service import legal.
- **gRPC error handling**: handlers return `status.Error(codes.X, msg)` from `google.golang.org/grpc/status` — `utilhttp.AppError` is HTTP-only and does not apply. The recovery interceptor (`route/interceptor/recovery.go`) converts panics to `codes.Internal` so the outer logging interceptor sees a meaningful code on the access log line.
- **Generated code**: `infra/database/db/*.go` (sqlc output) and gRPC stubs under `modules/{audit,queue}/src/route/grpc/` (`*.pb.go`, `*_grpc.pb.go`) are generated — modify the source (`.sql` queries, `.proto`) and regenerate. The `shared/<svc>client/` wrappers are hand-written but follow a strict pattern: connection lifecycle + type aliases re-exporting proto types.

## Kubernetes deployment

Each service owns its k8s manifests under `modules/<svc>/deploy/k8s/{base,overlays/dev}/`. The cross-service entry point is `deploy/k8s/dev/kustomization.yaml`, which references each service's `overlays/dev` plus `otel/k8s/overlays/dev` (the obs stack). Four namespaces (`audit`, `auth`, `queue`, `observability`) are created — three per service module + one for the obs stack — and cross-service traffic uses `<svc-resource>.<ns>.svc.cluster.local` DNS (e.g. `queue-api.queue.svc.cluster.local:8080`, `otel-collector.observability.svc.cluster.local:4317`). Image names have no project prefix: `audit-api`, `audit-worker`, `auth-api`, `queue-api`, `migrator`, all tagged `:dev` for local kind. Upstream obs images (collector / prometheus / tempo / loki / grafana) are pinned to their published tags in `otel/k8s/base/`. `compose.yml` is preserved for Docker-Compose-based local dev — k8s is additive.

The obs stack is the one exception to the per-module manifest rule: it lives at `otel/k8s/{base,overlays/dev}/` (not `deploy/k8s/observability/`) so kustomize's `configMapGenerator` can pull `otel/<component>/*` config files into ConfigMaps without tripping the load-restriction sandbox. See `.claude/rules/kubernetes-conventions.md` for the binding manifest conventions and `deploy/k8s/README.md` for the full layout walkthrough.

## Observability

Telemetry is OpenTelemetry-native. Each binary calls `utilotel.Init` in `run()`, which sets up the global TracerProvider + MeterProvider from `OTEL_*` env vars and falls back to **noop providers** when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset — `make audit` / `make auth` stay zero-overhead in the default dev loop. HTTP services install `utilotel.HTTPMiddleware(...)` (its `SpanNameFormatter` reads `r.Pattern` so chi v5 / stdlib ServeMux 1.22+ both produce `"<METHOD> <pattern>"` span names automatically — no per-router retag middleware needed); gRPC servers add `utilotel.GRPCServerOption()` ahead of the existing `ChainUnaryInterceptor`; outbound clients (canonical example `audit/infra/queueclient`) prepend `utilotel.GRPCClientOption()` so trace context propagates cross-service.

The local dev stack at `otel/` ships across two paths: `make obs-up` (Compose include of `otel/compose.yml`) and `make k8s-up` (`otel/k8s/overlays/dev` brought in by the root kustomization). Both serve the same backends — `otel-collector` (contrib) → Prometheus (`:9090`) + Tempo (`:3200`) + Loki (`:3100`) → Grafana (Compose host `:3001` to dodge `:3000` dev-server collisions; in-cluster `:3000` Service) — with cross-signal correlation pre-provisioned and ConfigMaps generated from the same `otel/<component>/*` source files in both deployment paths.

Status of the four planned observability phases:

- **Phase 1 (traces + metrics):** done. Spans + RPC duration histograms reach Tempo and Prometheus; verified via `kubectl kustomize` and Compose smoke tests.
- **Phase 2 (rules + dashboards):** done. Recording / alert rules under `otel/prometheus/rules/`; three split dashboards (`go-runtime`, `http-red`, `grpc-red`).
- **Phase 3 (logs producer):** **paused.** The `otelslog` bridge crashed `audit-api` with a SIGSEGV inside `go.opentelemetry.io/otel/sdk/log` v0.19's `sync.Pool` path under Go 1.26.2. The bridge is reverted; logs flow only to stdout via `utillog`. Loki container, Collector logs pipeline, and Grafana datasource remain provisioned and idle so re-enabling is one Init function edit away when sdk/log ships a fix (or we pivot to a `filelog` receiver). The `slog.InfoContext` switch in interceptors stays — ctx-aware logging is good practice on its own and is exactly what a future bridge will use.
- **Phase 4 (k8s overlay):** done. Five Deployments + Services in `observability` namespace; service-side `OTEL_*` env in each module's k8s ConfigMap points at `otel-collector.observability.svc.cluster.local:4317`.

See `otel/README.md` for the architecture diagram, port table, troubleshooting, and how to add a custom metric / span.

## Detailed Rules

The following files contain the full development workflow, coding standards, testing policy, k8s conventions, and AI agent implementation rules for this repo. Treat them as binding for any change you author here.

@.claude/rules/development-workflow.md
@.claude/rules/coding-standards.md
@.claude/rules/testing.md
@.claude/rules/kubernetes-conventions.md
@.claude/rules/ai-agent.md

## Per-module guidance

Each service module also ships its own `modules/<svc>/CLAUDE.md` with module-local invariants (canonical reading order, what's stub vs. real, module-specific Make targets, gotchas). Read it first when working inside a single service — it picks up where this file leaves off.

## Per-service design docs

Each service has an MVP-level system design doc that future changes should align with. Read the relevant one before non-trivial work in that module:

- `modules/auth/docs/system-design.md` — OAuth 2.0 / OIDC AS
- `modules/audit/docs/system-design.md` — 5W1H audit trail
- `modules/queue/docs/system-design.md` — priority queue + audit contract
