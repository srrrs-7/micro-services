# CLAUDE.md (modules/queue)

This file extends the repo-root [`CLAUDE.md`](../../CLAUDE.md) with guidance specific to the `queue` module. The repo-root file remains the source of truth for cross-cutting conventions; only queue-local facts live here.

## What this module is

A priority-aware, Kafka-flavored message broker (topic + consumer-group abstraction with **per-message priority** and **per-message ack/nack + visibility timeout**) exposed as a single gRPC service. The first-party consumer is `audit-worker`, which pulls the `audit.events` topic into `audit-db`.

Primary spec: [`docs/system-design.md`](docs/system-design.md). §1.2 (what we trade vs Kafka), §3.4 (the audit contract), §6 (proto + RPC catalog), §9 (ordering / priority interaction), §13 (Phase plan).

## Where to start reading

1. `docs/system-design.md` §1–§3 — scope, RFCs/best-practices, audit-side contract.
2. `src/route/grpc/queue.proto` — the wire contract; every Phase change starts here.
3. `src/cmd/api/main.go` — gRPC server lifecycle (env → DI → graceful shutdown). Mirror this when wiring is added.
4. `src/route/server.go` — interceptor chain + reflection + `grpc.health.v1` registration.
5. `src/route/handler.go` — `*handler` with `UnimplementedQueueServer` embed; this is where real RPCs land.
6. `src/route/interceptor/{logging,recovery}.go` — the only hand-written interceptors.

## Layout (queue-specific)

Differs from a "standard" service module:

- **No `domain/`, no `service/`, no `infra/database/` yet.** Phase 1.0 (storage) is still pending — see design §13. When DB lands, follow auth's layered shape (`coding-standards.md` §1).
- **Single binary**: `cmd/api`. There is no worker — queue is a server, not a consumer.
- **No `infra/<svc>client/`** — queue is a producer of types, not a consumer of others. The consumer-side wrapper for talking *to* queue lives in `audit/src/infra/queueclient/` (see coding-standards §2).

## Module-local conventions

- **`UnimplementedQueueServer` embed** is mandatory on `*handler` (see `route/handler.go`). protoc-gen-go-grpc's forward-compat contract relies on it: every RPC added to `queue.proto` automatically returns `codes.Unimplemented` until a real method lands on `*handler`. Do not change the embed to a pointer or remove it.
- **gRPC error codes**: handlers return `status.Error(codes.X, msg)` from `google.golang.org/grpc/status`. The recovery interceptor converts panics to `codes.Internal`. `utilhttp.AppError` is HTTP-only and does not apply here.
- **`route/grpc/` is the contract surface** and is the only path inside this module that may be imported by another module (per coding-standards §2). The cross-import is confined to `<consumer>/src/infra/queueclient/`. If you add a new top-level proto message that consumers must name, add a matching `type X = queuegrpc.X` alias in **the consumer's wrapper** — not anywhere here.
- **Priority + ordering interaction**: `docs/system-design.md` §9 is the authoritative description. Same `(partition_key, priority)` is FIFO; cross-priority delivery is non-FIFO by design. Do not "tighten" ordering without updating the design first.
- **at-least-once**: MVP is at-least-once delivery; consumers must be idempotent. The audit-side `event_id` UUID is the canonical example (`ON CONFLICT (event_id) DO NOTHING`).

## Make targets specific to this module

| Target | What it does |
|---|---|
| `make queue-proto-gen` | regenerate `route/grpc/{queue.pb.go,queue_grpc.pb.go}` from `queue.proto` |

There is **no** `make queue` / `queue-up` / `queue-migrate` (`SERVICES := auth audit` only). `queue-api` ships inside `audit_compose_up`, so `make audit` brings it up alongside the audit stack. Phase 1.0 will likely add per-service queue targets when `queue-db` lands.

## When proto changes

1. Edit `src/route/grpc/queue.proto`.
2. `make queue-proto-gen`.
3. Commit the proto AND the generated `*.pb.go` together.
4. Add the handler method on `*handler` only when you have a real implementation; otherwise let the `UnimplementedQueueServer` embed handle it.

`devcontainer` ships `protoc 28.3` + `protoc-gen-go v1.34.2` + `protoc-gen-go-grpc v1.5.1`. Bumping versions also requires editing `.devcontainer/Dockerfile` and `Makefile`'s `PROTOC_INCLUDE`.
