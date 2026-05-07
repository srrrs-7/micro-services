# CLAUDE.md (modules/audit)

This file extends the repo-root [`CLAUDE.md`](../../CLAUDE.md) with guidance specific to the `audit` module. The repo-root file remains the source of truth for cross-cutting conventions (errors, gRPC consumer pattern, Kubernetes layout, etc.); only audit-local facts live here.

## What this module is

The 5W1H audit-trail service. `audit-api` exposes a gRPC `Audit` service (Ingest / GetEvent / ListEvents); `audit-worker` (Phase 1.1) consumes the `audit.events` topic from `queue-api` for asynchronous ingest. Append-only by application contract; future hash-chain integrity verification is Phase 1.5.

Primary spec: [`docs/system-design.md`](docs/system-design.md). Read §1 (scope), §4 (architecture), §5 (storage), §6 (proto/JSON shape), §13 (Phase plan) before non-trivial work.

## Where to start reading

1. `docs/system-design.md` — what the service must guarantee.
2. `src/route/grpc/audit.proto` — the wire-level contract.
3. `src/cmd/api/main.go` — env → DI → gRPC server lifecycle (canonical for any new wiring).
4. `src/infra/database/migrations/20260507075754_audit_events.sql` — the schema.
5. `src/infra/database/queries/audit.sql` — currently the only sqlc input.
6. `src/infra/queueclient/client.go` — the **only** place inside this module that imports `queue/route/grpc` cross-service. See coding-standards §2 for the contract-surface exemption.

## Layout (audit-specific)

Differs from a "standard" service module in three ways:

- **Two binaries**: `cmd/api` (gRPC server) and `cmd/worker` (queue consumer, currently a stub).
- **No `domain/` or `service/` packages yet** — handlers in `route/handler.go` are still `UnimplementedAuditServer`. When real handlers land, follow the auth-side pattern (`route/handler.go` → `service/<feature>.go` → `domain/`).
- **Has `infra/queueclient/`** — the gRPC consumer wrapper (canonical example in the repo). Every audit package that needs a queue type references it through this wrapper's type aliases (`queueclient.PublishRequest`, `queueclient.LeasedMessage`, …) — never `queue/route/grpc` directly.

## Module-local conventions

- **Append-only**. `audit_events` rows are written once and never updated. Migrations that would `UPDATE` or `DELETE` existing audit rows are rejected by review.
- **`event_id` (UUID) is the natural idempotency key**. `InsertEvent` uses `ON CONFLICT (event_id) DO NOTHING RETURNING ...`; duplicate ingests return the **original** `recorded_at`, not the retry's. Mirror this in any new ingest path.
- **5W1H column shape is locked** for Phase 1.0. Adding a new structured field (Who/What/Where/Why/How) requires both a schema migration and a proto change; freeform context goes in `details JSONB`. See design §5.1 for what may NOT live in `details` (no PII, no secrets).
- **gRPC error codes**: handlers return `status.Error(codes.X, msg)` from `google.golang.org/grpc/status`. The recovery interceptor (`route/interceptor/recovery.go`) converts panics to `codes.Internal` so the access log line gets a meaningful code. `utilhttp.AppError` is HTTP-only and does not apply here.
- **DB role separation** that revokes `UPDATE`/`DELETE` (design §5.2) is configured **outside** the migration file so the devcontainer Postgres stays writable for tests. Don't bake the revoke into a migration.

## Make targets specific to this module

| Target | What it does |
|---|---|
| `make audit` | build + up + migrate (brings up `audit-api`, `audit-worker`, `audit-db`, `queue-api`, `migrator`) |
| `make audit-down` | stop the audit compose stack |
| `make audit-build` / `audit-up` / `audit-migrate` | granular |
| `make audit-new-migrate` | shell into `migrator` and `atlas migrate new` |
| `make audit-sqlc-gen` | regenerate `infra/database/db/*.go` from queries + migrations |
| `make audit-proto-gen` | regenerate `route/grpc/{audit.pb.go,audit_grpc.pb.go}` |

`audit-up` brings up `queue-api` too because the worker consumes it. `audit-worker` is currently a stub — the compose up succeeds even though it just prints and exits.

## When proto / schema changes

- **`audit.proto` change** → `make audit-proto-gen` → commit `*.pb.go` alongside the proto. Add handler method on `*handler` only when ready; otherwise the `UnimplementedAuditServer` embed keeps the build green and returns `codes.Unimplemented` for the new RPC.
- **Schema change** → `make audit-new-migrate` (creates a timestamped file) → write SQL with full `COMMENT ON ...` annotations (design §5.1) → `make audit-migrate` (`migrate hash` + `apply`).
- **Query change** → edit `queries/audit.sql` → `make audit-sqlc-gen` → `cd modules/audit/src && go vet ./...`. The `regen-sqlc` skill automates this.
