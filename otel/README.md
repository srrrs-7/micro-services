# Observability stack (`otel/`)

Local OpenTelemetry-native observability for the audit / auth / queue services. Brings up an OTel Collector + Prometheus + Grafana + Tempo + Loki stack as a Compose `include` under `profiles: [obs]` — opt-in, isolated from the per-service `make audit` / `make auth` flows. The same shape lifts cleanly into Kubernetes (Phase 4).

For the **why** behind these choices (push-via-Collector, `shared/utilotel` placement, opt-in profile, etc.) see the design discussion that produced this stack — the short version is captured below in [Design rationale](#design-rationale).

## Quick start

```bash
make obs-up                                                      # collector + prometheus + grafana + tempo + loki
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 make audit  # recreate audit stack with OTel enabled
# generate some traffic (gRPC client / curl / etc.)
open http://localhost:3000                                       # Grafana, anonymous Admin
make obs-down                                                    # stop the obs stack (volumes preserved)
```

`make obs-up` does **not** restart your service stack — it only starts the obs containers and prints the `OTEL_EXPORTER_OTLP_ENDPOINT=...` line for you to re-run a service stack with. This keeps `make audit` zero-overhead when you don't care about telemetry.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Service binaries  (auth-api / audit-api / audit-worker / queue-api)
│  └── shared/utilotel.Init configures TracerProvider + MeterProvider
│  └── HTTP middleware (chi) / gRPC StatsHandler emit OTel data
│                              │ OTLP/gRPC :4317
│                              ▼                                 │
└─────────────────────────────────────────────────────────────────┘
                               │
              ┌────────────────┴────────────────┐
              │      otel-collector (contrib)   │
              │  - memory_limiter / batch       │
              │  - filter/healthcheck           │
              │  - resource (deployment.env=…)  │
              └─┬──────────────┬──────────────┬─┘
        traces  │      metrics │         logs │  (skeleton — no producer in Phase 1)
                ▼              ▼              ▼
            ┌───────┐  ┌──────────────┐  ┌──────┐
            │ tempo │  │ collector    │  │ loki │
            │ :3200 │  │ :8889 prom   │  │ :3100│
            └───┬───┘  │  exporter    │  └──┬───┘
                │      └──────┬───────┘     │
                │      scrape │             │
                │             ▼             │
                │      ┌────────────┐       │
                │      │ prometheus │       │
                │      │ :9090      │       │
                │      └─────┬──────┘       │
                │            │              │
                └────────────┴──────────────┘
                             │
                       ┌──────────────┐
                       │   grafana    │
                       │  :3000       │
                       └──────────────┘
```

| Signal | SDK in services | Wire path | Backend | Grafana datasource |
|---|---|---|---|---|
| Traces | `otelhttp` (auth) / `otelgrpc` StatsHandler (audit/queue) | OTLP gRPC → Collector → OTLP gRPC → Tempo | Tempo (`:3200`) | `Tempo` (uid=`tempo`) |
| Metrics | Same StatsHandler / middleware + `contrib/instrumentation/runtime` | OTLP gRPC → Collector → Prometheus exporter `:8889` ← scrape | Prometheus (`:9090`) | `Prometheus` (uid=`prometheus`) |
| Logs | _(skeleton)_ — slog stdout today | _Phase 3:_ filelog receiver OR `otelslog` bridge → Collector → Loki | Loki (`:3100`) | `Loki` (uid=`loki`) |

Cross-signal correlation is provisioned in Grafana: **Tempo→Loki** (find logs for a trace ID), **Tempo→Metrics** (service map / RED for a span), **Loki→Tempo** (open trace from a log line via `trace_id` derived field).

## Ports (host-published)

| Port | Service | Purpose |
|---|---|---|
| `3000` | grafana | UI |
| `3100` | loki | HTTP API |
| `3200` | tempo | HTTP query API |
| `4317` | otel-collector | OTLP gRPC (services push here) |
| `4318` | otel-collector | OTLP HTTP (alt protocol) |
| `8889` | otel-collector | Prometheus exporter (scraped by Prometheus) |
| `9090` | prometheus | UI / API |

Inside the `internal` Docker bridge, services resolve each other by container name (`otel-collector`, `prometheus`, `tempo`, `loki`, `grafana`).

## Telemetry flow — what each commit touched

| Code | Responsibility |
|---|---|
| `modules/shared/src/utilotel/init.go` | `Init` configures the global TracerProvider + MeterProvider from `OTEL_*` env vars. Falls back to **noop providers** when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, so dev loops without `make obs-up` stay zero-overhead. Always installs the W3C TraceContext + Baggage propagator. Starts Go runtime metrics. |
| `modules/shared/src/utilotel/http.go` | `HTTPMiddleware(serverName, opts...)` wraps `otelhttp.NewMiddleware`. Default request filter skips `GET /health`. Override via `WithRequestFilter`. |
| `modules/shared/src/utilotel/grpc.go` | `GRPCServerOption()` returns `grpc.StatsHandler(otelgrpc.NewServerHandler())` — covers both spans and `rpc.server.*` metrics. `GRPCClientOption()` returns a `utilgrpc.Option` so it composes with the existing dial-option set. |
| `modules/auth/src/route/middleware/otel.go` | chi-specific `RouteTag()` middleware that retags spans with `RoutePattern()` after the inner handler runs. Lives in `auth` (not `shared`) so `shared/utilotel` stays chi-free. |

Per-binary wiring lives in each `cmd/<binary>/main.go`. The OTel shutdown is invoked **before** DB / cache close so in-flight spans + metrics reach the Collector while the network is still up.

## Adding a custom metric or span

In any service code:

```go
import (
    "go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("auth/service/login")
var meter  = otel.Meter("auth/service/login")

ctx, span := tracer.Start(ctx, "Login.verify")
defer span.End()

requests, _ := meter.Int64Counter("auth.login.attempts")
requests.Add(ctx, 1, /* attribute.String("result","ok") */)
```

There is no need to plumb a `TracerProvider` through DI — `Init` sets the global, and `otel.Tracer(name)` reads from it.

When the noop fallback is active (no obs stack), these calls are zero-allocation no-ops.

## Adding a new service

1. In `cmd/<binary>/main.go`, call `utilotel.Init(ctx, "<service-name>")` early in `run()` and defer the returned shutdown alongside other resource teardowns.
2. **HTTP server:** wire `utilotel.HTTPMiddleware("<service-name>")` via `r.Use(...)` after the chi seam line. If using chi, also wire a `RouteTag()` middleware (copy from `modules/auth/src/route/middleware/otel.go`).
3. **gRPC server:** add `utilotel.GRPCServerOption()` to `grpc.NewServer(...)` *before* the `ChainUnaryInterceptor` call so the StatsHandler sits outside the existing logging/recovery interceptors.
4. **Outbound gRPC client:** when calling `utilgrpc.Dial`, prepend `utilotel.GRPCClientOption()` to the option list (see `audit/infra/queueclient/client.go`).
5. In `compose.yml`, add the service's env block with `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, the `OTEL_EXPORTER_OTLP_ENDPOINT: ${OTEL_EXPORTER_OTLP_ENDPOINT:-}` interpolation, `OTEL_EXPORTER_OTLP_PROTOCOL: grpc`, and `OTEL_TRACES_SAMPLER`.

The dashboard's `$service` template variable will pick up the new `service.name` automatically (it queries Prometheus for `label_values(service_name)`).

## Troubleshooting

**"I see no traces in Grafana."** Did you re-run your service stack with `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317`? `make obs-up` only starts the obs containers — services started without that env var are running with `utilotel` in noop mode. Check via `docker compose exec audit-api env | grep OTEL`.

**"`make obs-up` succeeds but Grafana shows datasource errors."** Run `make obs-status`; if any container is in `Restarting`, check `docker compose logs <svc>`. Tempo and Loki sometimes fail on first start with permission errors on the named volume — `docker compose down --volumes` then `make obs-up` resets them.

**"Health check spans dominate Tempo."** They shouldn't — the Collector's `filter/healthcheck` processor drops `GET /health` (HTTP) and `grpc.health.v1.Health/Check`/`Watch` (gRPC). If you're seeing them, your code path attaches a different `http.route` attribute or names the gRPC method differently — adjust `otel/collector/config.yaml` accordingly.

**"The OTLP exporter logs `connection refused` on stderr without obs running."** Means `OTEL_EXPORTER_OTLP_ENDPOINT` is set but the collector is unreachable. Either bring obs up (`make obs-up`) or recreate your stack without the env var (compose interpolation defaults the value to empty when unset, which trips the noop fallback).

**"Prometheus has no data even though traces work."** OTel metrics use a periodic exporter (default 60s interval). Wait, generate some traffic, check Prometheus targets at <http://localhost:9090/targets> — `otel-collector:8889` should be UP.

## Configuration reference

Every binary reads the standard OTEL_* environment variables:

| Variable | Effect |
|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector address (`http://otel-collector:4317`). **Empty / unset → utilotel noop.** |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` (recommended) or `http/protobuf` |
| `OTEL_SERVICE_NAME` | Overrides the explicit `serviceName` arg passed to `Init` |
| `OTEL_RESOURCE_ATTRIBUTES` | `key=value,key=value` — merged into `resource` (e.g. `service.namespace=audit`) |
| `OTEL_TRACES_SAMPLER` | `always_on` / `parentbased_traceidratio` / etc. Default: parent-based |
| `OTEL_TRACES_SAMPLER_ARG` | Sampler argument (e.g. ratio for `traceidratio`) |
| `OTEL_LOG_LEVEL` | Internal SDK log verbosity |

Compose env in `compose.yml` sets these per service; `make obs-up` documents the endpoint to export from the host shell when re-running a service stack.

## Phase roadmap

- **Phase 1 (this PR):** stack up; traces + metrics flow end-to-end; logs pipeline configured but no producer (slog stays on stdout).
- **Phase 2:** richer dashboards (split HTTP / gRPC / per-service views), alert rules under `otel/prometheus/rules/`, sampling tuning.
- **Phase 3:** logs producer — choose between `filelog` receiver (compose-local, no code change) and `otelslog` bridge in `shared/utillog` (trace ID auto-injection, code change). Wire chosen path into the `logs` pipeline.
- **Phase 4:** Kubernetes overlays under `deploy/k8s/observability/{base,overlays/dev}/` mirroring the per-service module pattern, in a new `observability` namespace. Same Collector / Prometheus / Tempo / Loki / Grafana shape; image tags `:dev`, configmap-mounted configs, Downward API for `k8s.pod.name` resource attributes.

## Design rationale

Recorded here so future "why is this not done differently?" questions land on the answer.

- **Push (services → Collector → Prometheus scrape) vs. pull (services expose `/metrics`):** push, because gRPC services would otherwise need a second HTTP listener purely for `/metrics`, and we want to keep `prometheus/client_golang` out of the dependency tree (single SDK = OTel only). The cost is a HA-Collector requirement in prod (Phase 4).
- **`shared/utilotel` vs. duplicate per-service init:** shared, mirroring the existing `utilXxx` family. Single SDK version across the monorepo; no per-service drift.
- **chi route-pattern retag in `auth/route/middleware/` not `shared/utilotel`:** keeps `shared` chi-free. Only auth uses chi today.
- **`filter/healthcheck` at the Collector, not the SDK:** single point of filtering for both traces and metrics. SDK-level filters would have to be configured per binary.
- **Opt-in profile (`obs`) instead of always-on:** `make audit` should not pay the cost of starting Grafana for every iteration. The two-step dance (`make obs-up` → re-run service stack with env) is a small ergonomics tax for a big resource win.
- **Logs deferred:** filelog vs. otelslog is a real fork — one is host-side and zero-code, the other is in-process and gives trace ID correlation. Choosing too early would foreclose the better answer; the pipeline shape stays the same regardless.
