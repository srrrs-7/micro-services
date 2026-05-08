# CLAUDE.md (modules/shared)

This file extends the repo-root [`CLAUDE.md`](../../CLAUDE.md) with guidance specific to the `shared` module. The repo-root file remains the source of truth for cross-cutting conventions; only shared-local facts live here.

## What this module is

A library-only module — no `cmd/`, no `deploy/`, no migrations, no proto. Holds primitives every service would otherwise re-derive: HTTP error catalog + JSON helpers, slog setup, Redis client + prefixed wrapper, gRPC client plumbing, and the OpenTelemetry SDK wiring.

## Where to start reading

1. `src/utilhttp/error.go` — the typed `AppError` + `ErrorType` enum. Adding a new category requires synchronized edits in `error.go` and `response.go`; use the `add-error-type` skill.
2. `src/utilhttp/response.go` — `ResponseOk` / `ResponseError` type-switch. Note: this file uses `goccy/go-json`; `request.go` uses stdlib `encoding/json` (known inconsistency — `coding-standards.md` §12).
3. `src/utilhttp/request.go` — `RequestBody[T]` / `RequestUrlParam[T]` generics. The `Validator` interface is the constraint that lets `T` validate itself.
4. `src/utilcache/cache.go` — `Cache` wraps `*redis.Client` with a prefix and a uniform TTL. Multiple `Cache` values per service (different prefixes/TTLs) are fine.
5. `src/utilgrpc/client.go` — `Dial` + functional `Option`s. `Option` is **re-exported** by per-service consumer wrappers (e.g. `audit/src/infra/queueclient/`) as `type Option = utilgrpc.Option` so callers don't import `shared/utilgrpc` directly.
6. `src/utilotel/{init,http,grpc}.go` — `Init` configures the global TracerProvider + MeterProvider from `OTEL_*` env vars (noop providers when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset; near-zero overhead, the W3C TraceContext + Baggage propagator still runs unconditionally). `HTTPMiddleware` wraps `otelhttp.NewMiddleware` with a `SpanNameFormatter` that reads `http.Request.Pattern` so spans land as `"<METHOD> <pattern>"` without any per-router retag middleware. `GRPCServerOption` returns the StatsHandler; `GRPCClientOption` returns a `utilgrpc.Option`. See [`otel/README.md`](../../../otel/README.md) for the dev stack, supported router versions, and PII / sampling notes.

## Layout (shared-specific)

Differs from every other module:

- **No binary.** No `cmd/`, no `infra/database/`, no `route/`, no `deploy/`. Just packages directly under `src/`.
- **No service-specific code.** If a util is only useful to one service, it does NOT belong here. The canonical example of "almost added to shared, then moved out" is the `<svc>client/` gRPC consumer wrapper — it lives in **the consumer's `infra/<svc>client/`** (see `audit/src/infra/queueclient/`), not in shared. shared owns the generic `utilgrpc` plumbing only.
- **`testutil/` is reserved for cross-module test helpers** (`testing.md` §2.2). Currently empty — add it on demand. Per-service helpers go under `modules/<svc>/src/testutil/`, not here.

## Module-local conventions

- **Dependency direction terminus.** `shared` does not import `auth`, `audit`, or `queue`. If you find yourself reaching for a service type, the code probably belongs in that service's `infra/` instead.
- **No service-specific defaults baked in.** `utilcache.NewClient` doesn't know about a "session" prefix; `utilgrpc.Dial` doesn't know about a "queue-api" address. Services configure these at call sites.
- **All public API has tests next to it.** `utilhttp/*_test.go`, `utillog/log_test.go`, `utilcache/cache_test.go` (+ `_integration_test.go` build-tagged), `utilgrpc/{client,interceptor}_test.go`. New shared code without a test will not pass review.
- **Adding an error type spans two files** — `error.go` (the `iota` block + `String()` + `Type` field on the wrapper struct) and `response.go` (the type-switch in `ResponseError`). Use the `add-error-type` skill so neither half is missed.
- **JSON inconsistency is a known fact, not a bug to fix mid-PR.** `response.go` uses `goccy/go-json` (faster encode); `request.go` uses stdlib `encoding/json`. Match the file you're editing. Reconciliation, when it happens, is its own change with explicit justification.
- **gRPC option pattern**: `utilgrpc.Option` is `func(*config)` over a deliberately unexported `config` struct. Don't expose the struct — that's how we keep the API stable. New optional knobs become `WithXxx(...) Option` constructors.
- **`grpc.NewClient`, not `grpc.Dial`.** `Dial` is deprecated upstream; this package picked the non-deprecated path. If you wrap another gRPC primitive here, check the upstream deprecation list first.

## Make targets specific to this module

There are **no** shared-specific Make targets. shared is exercised by the workspace-wide targets:

| Target | What it does |
|---|---|
| `make test` | runs `go test ./...` per module — shared is one of them |
| `make lint` / `make vet` / `make fmt` / `make tidy` | iterate all modules including shared |

shared has no Docker image, no compose service, and no k8s manifests.

## When something here changes

- **Public API change** ripples to every importer. Search:
  ```bash
  grep -rn "shared/utilhttp\|shared/utillog\|shared/utilcache\|shared/utilgrpc" modules/{auth,audit,queue}/src
  ```
  Update call sites in the same change. The pre-push hook (`make test`) catches breakage.
- **New direct dep**: `cd modules/shared/src && go get <pkg> && go mod tidy`, then `make tidy` from the repo root to settle every module's `go.sum`.
- **Removing a util**: confirm zero call sites (the grep above) before deletion. Don't leave deprecation shims — the codebase prefers clean cuts (`coding-standards.md` general principle).
