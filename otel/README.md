# Observability stack (`otel/`)

Local OpenTelemetry-native observability for the audit / auth / queue services. Brings up an OTel Collector + Prometheus + Grafana + Tempo + Loki stack as a separate Compose file (`otel/compose.yml`) loaded only by the `make obs-*` targets — opt-in, isolated from the per-service `make audit` / `make auth` flows. The same shape lifts cleanly into Kubernetes (Phase 4).

For the **why** behind these choices (push-via-Collector, `shared/utilotel` placement, opt-in profile, etc.) see the design discussion that produced this stack — the short version is captured below in [Design rationale](#design-rationale).

## Quick start

```bash
make obs-up                                                      # collector + prometheus + grafana + tempo + loki
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 make audit  # recreate audit stack with OTel enabled
# generate some traffic (gRPC client / curl / etc.)
open http://localhost:3001                                       # Grafana, anonymous Admin (3001 to dodge dev-server :3000 collisions)
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
                       │ host :3001   │
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
| `3001` | grafana | UI (host 3001 → container 3000; remap to dodge :3000 dev-server collisions) |
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
2. **HTTP server:** wire `utilotel.HTTPMiddleware("<service-name>")` via `r.Use(...)` after the chi seam line. The middleware's `SpanNameFormatter` reads `http.Request.Pattern` (populated by chi v5 and stdlib ServeMux 1.22+) so span names land as `"<METHOD> <pattern>"` automatically — no per-router retag middleware needed.
3. **gRPC server:** add `utilotel.GRPCServerOption()` to `grpc.NewServer(...)` *before* the `ChainUnaryInterceptor` call so the StatsHandler sits outside the existing logging/recovery interceptors.
4. **Outbound gRPC client:** when calling `utilgrpc.Dial`, prepend `utilotel.GRPCClientOption()` to the option list (see `audit/infra/queueclient/client.go`).
5. In `compose.yml`, add the service's env block with `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, the `OTEL_EXPORTER_OTLP_ENDPOINT: ${OTEL_EXPORTER_OTLP_ENDPOINT:-}` interpolation, `OTEL_EXPORTER_OTLP_PROTOCOL: grpc`, and `OTEL_TRACES_SAMPLER`.

The dashboard's `$service` template variable will pick up the new `service.name` automatically (it queries Prometheus for `label_values(service_name)`).

## Troubleshooting

**"I see no traces in Grafana."** Did you re-run your service stack with `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317`? `make obs-up` only starts the obs containers — services started without that env var are running with `utilotel` in noop mode. Check via `docker exec audit-api env | grep OTEL`.

**"`make obs-up` succeeds but Grafana shows datasource errors."** Run `make obs-status`; if any container is in `Restarting`, check `docker-compose -f compose.yml -f otel/compose.yml logs <svc>`. Tempo and Loki sometimes fail on first start with permission errors on the named volume — `docker-compose -f compose.yml -f otel/compose.yml down --volumes` then `make obs-up` resets them.

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

- **Phase 1:** stack up; traces + metrics flow end-to-end; logs pipeline configured but no producer (slog stays on stdout).
- **Phase 2 (this PR):** dashboards split into three focused views (`go-runtime`, `http-red`, `grpc-red`) using OTel-semconv-current metric names (`go_*`, `rpc_server_call_duration_seconds_*`, `http_server_request_duration_seconds_*`); recording rules for RED quantities under `otel/prometheus/rules/recording.yml`; alert rules under `otel/prometheus/rules/alerts.yml` covering ServiceDown / High{HTTP,gRPC}{ErrorRate,LatencyP95}. Prometheus runs with `--web.enable-lifecycle` so reloading rules is `curl -X POST http://localhost:9090/-/reload`.
- **Phase 3 (deferred):** the otelslog bridge path was attempted but every audit-api crashed with a SIGSEGV inside `go.opentelemetry.io/otel/sdk/log` v0.19's `sync.Pool.getSlow` on the first attribute-bearing log record (Go 1.26.2). The bridge wiring + scaffold (`shared/utilotel/slog.go`) was reverted; logs continue to flow only to stdout via `shared/utillog`. The Collector / Loki side remains ready (Loki container, datasource, and Collector logs pipeline are still provisioned). The `slog.InfoContext` switch in the gRPC interceptors stayed — ctx-aware logging is net-positive on its own. Re-attempt path: wait for an upstream sdk/log release with the Pool fix, re-add a `LoggerProvider` + small slog bridge inside `Init`, and restore `lp.Shutdown` to the cleanup chain. Alternative: pivot to a `filelog` receiver tailing `/var/lib/docker/containers/*/*.log` on the Collector — code-change-free but loses inline trace_id correlation unless the JSON body carries it.
- **Phase 4 (this PR):** Kubernetes manifests for the obs stack under `otel/k8s/{base,overlays/dev}/` (chose `otel/k8s/` over `deploy/k8s/observability/` so kustomize's load-restriction sandbox doesn't block the `configMapGenerator` references back into `otel/<component>/*`). New `observability` namespace; one Deployment + Service per component (collector / prometheus / tempo / loki / grafana). The 8 ConfigMaps are generated from the existing `otel/` config tree — single source of truth across `make obs-up` (Compose) and `make k8s-up` (kind). Service-side `OTEL_*` env vars are now set in each module's k8s ConfigMap pointing at `otel-collector.observability.svc.cluster.local:4317`; remove `OTEL_EXPORTER_OTLP_ENDPOINT` from the ConfigMap to disable telemetry from a given service if obs is intentionally down in that env.

## Phase 2 — recording + alert rules + split dashboards

After pulling Phase 2:

```bash
# Apply rule + Prometheus config changes — rules live in otel/prometheus/rules/
docker rm -f prometheus
docker-compose -f compose.yml -f otel/compose.yml up -d prometheus

# (Subsequent rule edits) reload without restart
curl -X POST http://localhost:9090/-/reload

# Verify rules loaded
curl -fsS http://localhost:9090/api/v1/rules | jq '.data.groups[].name'
# expect: rpc.server.recordings, http.server.recordings,
#         service.availability, rpc.server.health, http.server.health

# Verify alerts visible
curl -fsS http://localhost:9090/api/v1/alerts | jq '.data.alerts | length'
```

Grafana auto-reloads dashboards every 30s via the dashboards provisioning provider — no Grafana restart needed when dashboard JSON changes. Three dashboards now appear under "Dashboards":

| UID | Title | Audience |
|---|---|---|
| `go-runtime` | Go Runtime | Per-service goroutines / memory / GC behaviour |
| `http-red` | HTTP RED (auth) | auth-api request rate / 4xx-5xx errors / p50-p95-p99 / top routes |
| `grpc-red` | gRPC RED (audit / queue) | RPC rate by method / errors by status code (excluding OK) / latency quantiles / top slow methods |

**Alerts and dev stubs.** `HighGRPCErrorRate` deliberately excludes `rpc_grpc_status_code="12"` (UNIMPLEMENTED) so audit's Phase 1 stub handlers don't page during dev. Drop the exclusion once real handlers land — see `otel/prometheus/rules/alerts.yml` for the comment that calls this out.

## Phase 3 — paused

The otelslog bridge crashed every audit-api on the first attribute-bearing log record (`sdk/log v0.19` Pool segfault). Producer code was deleted; the receiving side stays ready:

- Loki + Grafana Loki datasource + Collector logs pipeline: provisioned, idle.
- `shared/utillog` keeps writing JSON to stdout — visible via `docker logs <svc>`.
- Interceptors continue to use `slog.InfoContext(ctx, ...)` so a future bridge picks up trace context with no further service edits.

When sdk/log ships a fix (or we pivot to a `filelog` receiver), the re-enable is small: construct a `LoggerProvider` + `BatchProcessor` over `otlploggrpc.New(ctx)` inside `Init`, install a tee that wraps the existing slog default with `otelslog.NewHandler(serviceName)`, and append `lp.Shutdown(sctx)` to the cleanup. The previous attempt's deletion lives in git (commit removing `utilotel/slog.go`) for reference.

The Phase 1 / 2 / 4 stack is unaffected.

```bash
# Phase 1 / 2 service rebuild — unchanged
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 make audit-build
```

## Phase 4 — Kubernetes (kind)

The same Collector + Prometheus + Tempo + Loki + Grafana stack runs on the local kind cluster as part of `make k8s-up`. Layout:

```
otel/k8s/
├── base/
│   ├── kustomization.yaml          # configMapGenerator pulls otel/<comp>/*
│   ├── namespace.yaml              # `observability`
│   ├── collector.yaml              # Deployment + Service
│   ├── prometheus.yaml             # Deployment + Service (emptyDir)
│   ├── tempo.yaml                  # ditto
│   ├── loki.yaml                   # ditto
│   └── grafana.yaml                # ditto
└── overlays/
    └── dev/kustomization.yaml      # passthrough; staging/prod overlays go here
```

`deploy/k8s/dev/kustomization.yaml` references `../../../otel/k8s/overlays/dev`, so `kubectl apply -k deploy/k8s/dev` brings up the obs stack alongside audit / auth / queue. `make k8s-apply` invokes `kubectl kustomize --load-restrictor=LoadRestrictionsNone` so the configMapGenerator can reach back into `otel/<component>/`.

```bash
make k8s-up                    # cluster + build + load + apply + wait + status
make k8s-status                # pods/services/jobs across all 4 namespaces
make k8s-down                  # tear down the kustomization (cluster stays)
```

In-cluster DNS ports:

| URL | Use |
|---|---|
| `otel-collector.observability.svc.cluster.local:4317` | OTLP gRPC ingest (services dial here) |
| `prometheus.observability.svc.cluster.local:9090` | Prometheus UI / API |
| `grafana.observability.svc.cluster.local:3000` | Grafana UI (port-forward to view: `kubectl -n observability port-forward svc/grafana 3001:3000`) |
| `tempo.observability.svc.cluster.local:3200` | Tempo HTTP API |
| `loki.observability.svc.cluster.local:3100` | Loki HTTP API |

**Storage.** All five backends use `emptyDir` for dev (data lost on pod restart). For staging / prod, layer in a `PersistentVolumeClaim` per backend in `otel/k8s/overlays/<env>/`.

**Service-side OTEL_* env.** `modules/<svc>/deploy/k8s/base/configmap.yaml` ships with `OTEL_EXPORTER_OTLP_ENDPOINT` set to the in-cluster Collector. With `make k8s-up`, the obs stack and the services come up together so the dial succeeds. To disable telemetry from a specific service (e.g. for an env without the obs stack), remove that ConfigMap key — `utilotel.Init` falls back to noop providers when the endpoint is empty.

**ConfigMap edits.** Because `disableNameSuffixHash: true` keeps ConfigMap names deterministic, editing `otel/<component>/*` and re-running `make k8s-apply` updates the ConfigMaps but does **not** auto-restart pods. Roll explicitly: `kubectl -n observability rollout restart deploy/<component>`.

```bash
# Phase 1 / 2 service rebuild — unchanged
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 make audit-build

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 make audit-build
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317 make audit-up
```

(Same shape for `make auth-*`.) `make audit-down && make audit` works too — only the binaries need to be rebuilt because the bridge wiring is in `shared/utilotel`.

Verify logs are landing in Loki:

```bash
# Loki ingestion stats
curl -fsSG http://localhost:3100/loki/api/v1/labels | jq '.data'
# expect: ["service_name"] plus any other labels Loki extracted from
# OTLP resource attributes / structured metadata

# Recent records for a service
curl -fsSG http://localhost:3100/loki/api/v1/query \
  --data-urlencode 'query={service_name="audit-api"}' \
  --data-urlencode 'limit=5' | jq '.data.result[].values[][1]' | head
```

Or in Grafana → Explore → Loki: `{service_name="audit-api"}` returns recent log lines. Open a trace in Tempo, click the "Logs for this span" button — the tracesToLogs link (provisioned in `otel/grafana/provisioning/datasources/default.yaml`) jumps to Loki filtered by service.name.

**Where the trace IDs come from.** When code uses `slog.InfoContext(ctx, "...")` and ctx carries an active span, the otelslog bridge reads the trace context and writes `trace_id` / `span_id` as record attributes. With OTLP-to-Loki ingest, those become structured metadata on the log entry — Loki query `{service_name="audit-api"} | trace_id=~".+"` filters to traced log lines.

Existing handler code that uses bare `slog.Info("...")` (no ctx) still flows through, but without trace correlation. To get correlation, switch to `slog.InfoContext(ctx, "...")` in handler code paths that have a request context.

## Design rationale

Recorded here so future "why is this not done differently?" questions land on the answer.

- **Push (services → Collector → Prometheus scrape) vs. pull (services expose `/metrics`):** push, because gRPC services would otherwise need a second HTTP listener purely for `/metrics`, and we want to keep `prometheus/client_golang` out of the dependency tree (single SDK = OTel only). The cost is a HA-Collector requirement in prod (Phase 4).
- **`shared/utilotel` vs. duplicate per-service init:** shared, mirroring the existing `utilXxx` family. Single SDK version across the monorepo; no per-service drift.
- **Span-name formatting via `http.Request.Pattern` in `shared/utilotel`, not a chi-specific middleware:** chi v5 and stdlib ServeMux 1.22+ both populate `r.Pattern` during routing, so a single `SpanNameFormatter` in `utilotel.HTTPMiddleware` handles both routers. The earlier `auth/route/middleware/otel.go` retag middleware was deleted alongside the chi v5 upgrade — `shared/utilotel` stays chi-free without giving up route-pattern span names.
- **`filter/healthcheck` at the Collector, not the SDK:** single point of filtering for both traces and metrics. SDK-level filters would have to be configured per binary.
- **Opt-in profile (`obs`) instead of always-on:** `make audit` should not pay the cost of starting Grafana for every iteration. The two-step dance (`make obs-up` → re-run service stack with env) is a small ergonomics tax for a big resource win.
- **Logs deferred:** filelog vs. otelslog is a real fork — one is host-side and zero-code, the other is in-process and gives trace ID correlation. Choosing too early would foreclose the better answer; the pipeline shape stays the same regardless.
