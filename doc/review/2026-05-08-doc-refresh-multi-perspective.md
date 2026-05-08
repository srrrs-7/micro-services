# Multi-perspective review — 2026-05-08 doc refresh

| | |
|---|---|
| **日付** | 2026-05-08 |
| **対象ブランチ** | `master` (uncommitted: README.md, modules/shared/CLAUDE.md, modules/shared/README.md, otel/README.md) |
| **発火元** | `/refresh-docs` → `/multi-perspective-review` |
| **変更概要** | chi v5 アップグレード (commit `c79c46a`) で削除された `modules/auth/src/route/middleware/otel.go` の参照除去 + `shared/utilotel` パッケージの追加反映 (4 docs, doc-only) |
| **総合判定** | `minor-cleanup` — block-merge / dangerous / blocking はゼロ。指摘は doc 仕上げと、新文書が anchor している既存コード債務 |

---

## 1. 対応アイテム一覧 (チェックリスト)

### 1.1 doc-only — この変更内で対応する

| # | 対応内容 | 検出元 | 重要度 |
|---|---|---|---|
| A | `otel/README.md:249-252` の壊れた重複 fenced-code block を削除 | coding-conventions, maintainability | 高 (実際のレンダリングバグ) |
| B | `chi v5 / stdlib ServeMux 1.22+` のバージョン記述を `otel/README.md` に集約。`shared/CLAUDE.md:6` と `shared/README.md:15` は参照に置換 | maintainability | 中 (3 箇所重複) |
| C | `shared/README.md:39` の `http.go` 行 tree コメントを `HTTP OTel middleware` に短縮 (バージョン記述は table から消す) | maintainability | 中 (B の派生) |
| D | `otel/README.md` "Adding a new service" §2 に 1 文追加: `r.Pattern == ""` (404 / 非 chi/ServeMux) フォールバックは `serverName` になる | reliability | 低 (debuggability) |
| E | `otel/README.md` `OTEL_TRACES_SAMPLER` 行に PII 注記: `always_on` は Tempo 100% 保持、`otelhttp` の `net.peer.addr` / `http.user_agent` は `audit_events` §5.1 で PII 分類済み | compliance | 中 (本番運用前に必須) |
| F | `otel/README.md` Troubleshooting に注記: `OTEL_EXPORTER_OTLP_ENDPOINT` の ConfigMap 切り替えは明示的な `kubectl rollout restart` が必要 (mixed-fleet hazard) | scalability | 低 |
| G | `shared/CLAUDE.md:6` の "noop fallback" 表現を緩和: W3C propagator は無条件で走るため "near-zero overhead" が正確 | performance | 低 (誤解防止) |

### 1.2 code-side — 別 PR でフォローアップ

| # | 対応内容 | 検出元 | 重要度 |
|---|---|---|---|
| H | `modules/shared/src/utilotel/http_test.go` に `TestHTTPMiddleware_spanNamePrependMethodForChiPattern` を追加 (chi v5 ブランチが現状未テスト) | testability | 中 (本ドキュメント変更で promise した挙動の coverage gap) |
| I | `TestHTTPMiddleware_spanNameFallsBackToServerNameWithoutPattern` の振る舞い検証なしアサーションを修正 (実際の span name を assert する形に) | testability | 中 |
| J | `modules/auth/src/cmd/api/main.go:142` の `<-shutdownCtx.Done()` 削除 (graceful shutdown が常に 30 秒待つバグ) | reliability | 中 |
| K | `modules/shared/src/utilotel/init.go:104-108` の `tp.Shutdown` + `mp.Shutdown` で同じ 5s context を共有している件のレビュー | reliability | 低 |
| L | `otel/k8s/base/collector.yaml:10` に "Phase 4 prod requires HA" のコメント注記 (もしくは prod overlay で replicas patch を予約) | scalability | 低 |

---

## 2. 横断パターン (cross-reviewer)

- **新文が anchor している既存コード債務** — testability + reliability が `utilotel/` のテスト不足と `auth/cmd/api/main.go` の shutdown blocking を独立に flag。本変更で導入したバグではないが、リフレッシュした doc が「これがリファレンス」として endorsement する形になるため、code-side の追従 PR で潰すのが筋。
- **バージョン結合表現が 3 ファイルに重複** — maintainability が直接指摘、coding-conventions が文言一致を独立に確認。2 reviewer が agree しているので今回まとめて整理する価値あり。
- **`otel/README.md` の壊れた fenced-code block** — coding-conventions と maintainability が独立に検出。本変更で混入したものではないが、doc を触っている今こそが直し時。

---

## 3. 集約結果トップ findings (severity 順)

```
[quality]   [coding-conventions, maintainability] otel/README.md:249-255 — Malformed fenced-code block.
[quality]   [maintainability]                     shared/README.md:15 + shared/CLAUDE.md:6 + otel/README.md:116 — version phrase 3x duplication.
[quality]   [maintainability]                     shared/README.md:39 — tree comment duplicates table-row content.
[quality]   [testability]                         shared/src/utilotel/http_test.go — chi-v5 branch coverage gap.
[quality]   [testability]                         shared/src/utilotel/http_test.go:127-157 — behavior-free assertion.
[drift]     [compliance]                          otel/README.md:145 — always_on + otelhttp PII attributes not documented.
[drift]     [reliability]                         otel/README.md "Adding a new service" §2 — empty-Pattern fallback not documented.
[drift]     [performance]                         shared/CLAUDE.md:6 — "noop fallback" implies zero-overhead but propagator runs unconditionally.
[drift]     [scalability]                         otel/k8s/base/collector.yaml:10 — replicas:1 SPOF; HA-Collector noted in docs but not in manifest.
[drift]     [scalability]                         otel/README.md (mixed-fleet rollout) — endpoint-toggle ConfigMap change requires explicit rollout restart.
```

---

## 4. Per-reviewer reports (verbatim)

8 sub-agent (security / performance / coding-conventions / maintainability / testability / reliability / scalability / compliance) を並列 dispatch した結果。各 agent の verbatim 報告を保存する。

### 4.1 Security — `pass`

> No security findings. This is a doc-only diff (four markdown files) and all prose accurately reflects the implementation:
>
> - The `utilotel.HTTPMiddleware` description correctly characterizes `SpanNameFormatter` reading `r.Pattern` — confirmed against `/workspace/main/modules/shared/src/utilotel/http.go:62-70`. No security-relevant behavior is misrepresented.
> - The `OTEL_*` env-var documentation describes no secrets — `OTEL_EXPORTER_OTLP_ENDPOINT` is a collector endpoint address, not a credential. The noop-fallback behavior when the var is unset is correctly documented.
> - The deleted `auth/route/middleware/otel.go` file is correctly noted as removed. The one surviving reference in `/workspace/main/otel/README.md:285` is a historical design-rationale sentence, not a live instruction.
> - No hardcoded secrets, key prefixes, or JWT strings appear anywhere in the diff.
> - The trace-context propagation description (W3C TraceContext + Baggage) matches the implementation in `/workspace/main/modules/shared/src/utilotel/init.go:48-51`. Nothing in the docs overstates what is propagated — no PII fields are described as traversing span context.
> - `/health` probe exclusion from tracing (`defaultShouldTrace`) is correctly reflected in the `WithRequestFilter` description.

### 4.2 Performance — `ship`

> Hot-path findings: None. This is a doc-only diff; no source changed.
>
> Confirmed-good:
>
> **Noop fallback is not literally zero-overhead — the propagator is always installed.** The docs say "zero-overhead" when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset. The code at `init.go:48-56` installs the W3C TraceContext + Baggage propagator unconditionally before the early-return noop branch. That propagator runs on every inbound HTTP/gRPC request to extract context from headers. The noop TracerProvider and MeterProvider mean no spans are created and no metrics are emitted, but header extraction still happens per request. The claim is directionally accurate but "near-zero overhead" would be more honest.
>
> **`SpanNameFormatter` two-invocations-per-request claim is accurate.** `spanNameFromPattern` at `http.go:62-70` is six lines: empty-string check, `strings.HasPrefix`, conditional concat. One small allocation per request on chi paths, zero on ServeMux paths.
>
> **otelhttp version claim is accurate.** Pinned at v0.68.0 in go.mod; pattern-rewrite behavior is present.
>
> **Dropped retag middleware is confirmed gone.** `auth/route/middleware/` contains only `auth.go`.
>
> **Filter interaction is correct.** `otelhttp.WithFilter` short-circuits before invoking the formatter, so `GET /health` pays zero formatter cost.

### 4.3 Coding conventions — `matches-repo`

> Drift / inconsistency:
>
> - `otel/README.md:249-254` — duplicate unclosed code fence. Block at line 249 opens with a bare `bash` comment line and a closing fence at 251, then immediately opens another `\`\`\`bash` block at 252. Renders as malformed Markdown. Lines 249-252 should be deleted.
>
> Confirmed-good:
>
> - `README.md` table-row update consistent in column count, separator style, Japanese register with surrounding rows.
> - `modules/shared/CLAUDE.md` item 6 follows the same prose structure as items 1-5.
> - `modules/shared/README.md` package table row matches column count and Japanese register.
> - `modules/shared/README.md` directory tree `utilotel/` entry matches `utilcache/` and `utilgrpc/` style.
> - `otel/README.md` "Adding a new service" §2 prose consistent imperative voice; matches wording in `shared/CLAUDE.md` item 6 — no drift between sites.
> - Design-rationale bullet correctly records the deleted file by name and attributes the deletion to the chi v5 upgrade.
> - Bilingual status unchanged.

### 4.4 Maintainability — `small-debt`

> Findings:
>
> - `otel/README.md:252-255` — Duplicate code block. Two consecutive, unclosed/re-opened fenced code blocks. Rendering bug; will confuse the next reader.
> - `modules/shared/README.md:39` — Directory tree comment embeds `chi v5 / stdlib ServeMux 1.22+` in a code-comment column. Package table row already says this; tree-line should just say `HTTP OTel middleware`.
>
> Trends:
>
> - **Version-coupled phrasing across three files** — `modules/shared/README.md:15`, `modules/shared/CLAUDE.md:16`, `otel/README.md:116` all contain `chi v5 and stdlib ServeMux 1.22+`. Authoritative statement is in `otel/README.md`. The other two should reference it instead of duplicating.
>
> Future-debt watchlist:
>
> - `modules/shared/README.md:21-41` — directory tree is fragile; goes stale silently. Now that `utilotel/` lists four files individually, future additions won't appear here unless someone remembers.
> - `otel/README.md:285` — historical design-rationale bullet is useful for 3-6 months; becomes clutter once normalized. Flag for deletion when section is next touched.
> - `otel/README.md:156` — Phase 4 roadmap bullet in first-person commit-message voice ("chose `otel/k8s/` over …") will read oddly in 12 months.
>
> Confirmed-good:
>
> - Dead reference to `auth/route/middleware/otel.go` is fully excised.
> - `CLAUDE.md` item 6 is a readable single-sentence summary.
> - Root `README.md` change minimal and accurate.

### 4.5 Testability — `well-tested`

> Coverage gaps:
>
> `spanNameFromPattern` (`http.go:62-70`) contains a chi-v5-specific branch (path-only pattern, prepend `r.Method`). Doc claim in `shared/README.md:15` and `shared/CLAUDE.md:6` is that the formatter handles "chi v5 / stdlib ServeMux 1.22+". `TestHTTPMiddleware_spanNameUsesRequestPattern` (`http_test.go:100-125`) exercises only the stdlib ServeMux 1.22+ path. The chi-v5 branch (`r.Pattern = "/users/{id}"`, no method prefix) has no test. If the `strings.HasPrefix(r.Pattern, r.Method+" ")` check were inverted, the stdlib path would still pass; chi would silently produce wrong span names.
>
> Missing test: `TestHTTPMiddleware_spanNamePrependMethodForChiPattern` — register a handler where `r.Pattern` is path-only, assert span name is `"GET /users/{id}"`.
>
> Test smells:
>
> `TestHTTPMiddleware_spanNameFallsBackToServerNameWithoutPattern` (`http_test.go:127-157`) has a confused assertion. The comment acknowledges stdlib ServeMux still sets `r.Pattern = "/foo"`, so the formatter produces `"GET /foo"`. The assertion only checks the name is NOT `"test-server"`, which always passes regardless of what the formatter returns. Fix: assert `spans[0].Name() == "GET /foo"` directly, or rename to match what it actually verifies.
>
> Confirmed-good:
>
> - `TestInit_unsetEndpoint_returnsNoopShutdown` and `TestInit_unsetEndpoint_installsNoopProviders` confirm the noop-fallback claim.
> - `TestInit_alwaysSetsW3CPropagator` pins the propagator installation in the unset-endpoint path.
> - `TestHTTPMiddleware_skipsGETHealthByDefault` and `TestHTTPMiddleware_postHealthIsTraced` cover both branches of `defaultShouldTrace`.
> - `TestHTTPMiddleware_customFilterDisablesTracing` covers `WithRequestFilter`.
> - `TestGRPCClientOption_composesWithDial` uses lazy `grpc.NewClient` (no real server).
> - No `testify`, no `gomock`, no globals leaked.

### 4.6 Reliability — `resilient`

> Page-worthy: None.
>
> Degradation issues:
>
> - `auth/cmd/api/main.go:142` — `<-shutdownCtx.Done()` blocks until the 30-second shutdown context expires before `shutdown()` returns. Pre-existing bug, not introduced by this diff. The new docs ("OTel shutdown invoked before DB / cache close" — `otel/README.md:88`) implicitly endorse `shutdown()` as the reference pattern, so this is worth fixing.
> - `init.go:72-74` — When `OTEL_EXPORTER_OTLP_ENDPOINT` is set but the collector is unreachable, `otlptracegrpc.New(ctx)` succeeds (lazy connection) and the error only surfaces on the first export. The docs correctly describe this as non-fatal.
>
> Recovery story gaps:
>
> - `http.go:63-70` — Code is correct: empty `r.Pattern` returns `operation` (the `serverName`). However chi 404 path also produces empty `r.Pattern`, so 404 spans all bucket under the service name. Future implementer may think the formatter is broken. One-sentence note in the doc would prevent confusion.
> - `init.go:104-108` — Shutdown calls `tp.Shutdown` and `mp.Shutdown` sequentially with the same 5-second context. If `tp.Shutdown` takes the full 5s, `mp.Shutdown` inherits an expired context and fails silently. Pre-existing.
>
> Confirmed-good:
>
> - Noop-fallback behavior in `init.go:53-57` is exactly as documented.
> - Two-invocation `SpanNameFormatter` behavior accurately described.
> - Shutdown ordering matches `otel/README.md:88` claim — HTTP server stops first, then `otelShutdown`, then cache, then DB.
> - `shared/CLAUDE.md` "chi-free" claim accurately reflects the code (`http.go` imports only `net/http` and `otelhttp`).
> - W3C propagator always installed even in noop mode.

### 4.7 Scalability — `scales-cleanly`

> Replica-coherence: NONE. Removing the per-router retag middleware in favor of a single SpanNameFormatter eliminates a code path. Formatter is a pure stateless function with zero mutable state per request.
>
> Capacity findings:
>
> - `otel/k8s/base/collector.yaml:10` — Collector at `replicas: 1` is a single-writer / SPOF for all telemetry. Docs acknowledge HA-Collector as a Phase 4 prod requirement but the manifest carries no comment. Pre-existing.
> - `init.go:86` — PeriodicReader uses default 60s export interval. With N replicas synchronized to process start, all export simultaneously to the Collector at 60s wall-clock periods, creating bursty Prometheus write load at high N.
>
> Backpressure / rate-limit gaps:
>
> - `otel/README.md` (mixed-mode fleet) — noop-fallback story is correct per-replica, but partial rollouts (some pods with endpoint set, some not) silently produce incomplete traces with no observable signal. Note in Troubleshooting that endpoint-toggling ConfigMap changes require an explicit rolling restart.
>
> Confirmed-good:
>
> - Global TracerProvider/MeterProvider as process-level singletons is correct OTel design. N replicas each call `Init` once. Safe at any N.
> - W3C propagator always installed, so context headers forward cross-service even when receiving replica is in noop mode.
> - `spanNameFromPattern` is pure. Removing the old retag middleware strictly reduces per-request overhead.
> - Shutdown ordering documented and implemented correctly.
> - `defaultShouldTrace` skipping `GET /health` prevents probe spans from inflating RPS metrics or Tempo storage.

### 4.8 Compliance — `compliant (two doc gaps)`

> PII handling concerns:
>
> **`always_on` sampler + `actor_ip` / `actor_user_agent` in span attributes — not documented as a risk.** Every service in `compose.yml` and the k8s base ConfigMaps sets `OTEL_TRACES_SAMPLER: always_on`. `otelhttp` and `otelgrpc` StatsHandlers emit standard semantic-convention attributes including `net.peer.addr` and `http.user_agent`. The `audit_events` schema (§5.1) classifies these as PII. Same data lands in Tempo with no documented retention. Tempo uses emptyDir in dev (acceptable), but only mentioned in the k8s section, not in the general configuration reference or "Adding a new service" instructions.
>
> Fix (doc-level): in `otel/README.md` configuration table, note `always_on` means 100% retention in Tempo; in "Adding a new service" §2, note IP and user-agent attributes land in traces and must be considered for prod data-residency policy.
>
> **Span-name caveat for non-chi/non-ServeMux routers.** Span names use `r.Pattern` (correct, PII-safe) and fall back to `serverName` when `r.Pattern == ""`. Any handler outside chi/ServeMux 1.22+ produces span name = service name. Loses observability but does not leak PII. Doc could note the Go 1.22+ requirement for pattern population.
>
> Documentation alignment:
>
> - `otel/README.md` sampling section does not bridge audit §10.1 PII classification to the otel configuration choice.
> - `shared/CLAUDE.md` `WithRequestFilter` described only by its default; doesn't mention overriding implications.
>
> Confirmed-good:
>
> - Span names use `r.Pattern` (templated), not `r.URL.Path` (live URL with substituted values). PII in URL path params does not propagate into span names.
> - Removing the chi-specific `RouteTag()` middleware preserves the data-handling contract: old middleware called `chi.RouteContext(r.Context()).RoutePattern()`; new `SpanNameFormatter` reads `r.Pattern`. Effective output is identical.
> - W3C TraceContext + Baggage propagator always installed; trace context propagates across service boundaries even in noop mode.
> - Baggage propagation is currently un-populated by any code in the diff. Future code-review gate if a caller adds user data to Baggage.
> - Noop fallback correctly skips entire OTLP export chain.
> - Health endpoint spans filtered at both SDK and Collector — double-filtering is correct.
> - No secrets / credentials in the changed doc files.
> - No new dependencies; license/supply-chain unaffected.
