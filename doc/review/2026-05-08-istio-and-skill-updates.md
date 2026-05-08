# Multi-perspective review — 2026-05-08 Istio Ambient + skill/tooling updates

| | |
|---|---|
| **日付** | 2026-05-08 |
| **対象ブランチ / PR** | `master` 直近 5 commit (`767f885^..b3ebc17`) |
| **発火元** | `/multi-perspective-review` (user-invoked) |
| **変更概要** | Istio Ambient (PERMISSIVE mTLS in dev) を導入。`utilgrpc.WithTLS` を削除しトランスポート暗号化を mesh に委譲。auth サービスに `Gateway` + `HTTPRoute` を追加。`refactoring-reviewer` agent を新設し `/multi-perspective-review` skill を `doc/review/` に永続化対応。Makefile のカバレッジを `.coverage/<mod>-coverage.{txt,html}` に集約。 |
| **総合判定** | **needs-changes** — block-merge は無いが、信頼性 (`make istio-up` のロールアウト失敗が抑制される) とコンプライアンス (devcontainer の `~/.claude` 資格情報露出) でそれぞれ実質的指摘あり。doc-side の stale comment が 5 reviewer から重複検出されており、まとめて潰す価値が高い。 |

---

## 1. 対応アイテム一覧 (チェックリスト)

### 1.1 doc-only / scope-included — この変更内で対応する

| # | 対応内容 | 検出元 | 重要度 |
|---|---|---|---|
| A | `modules/auth/deploy/k8s/base/gateway.yaml:5-7` の NodePort/extraPortMapping コメントを削除 (`make istio-port-forward` を指す 1 行に置換) | security / coding-conventions / maintainability / reliability / compliance (5 reviewer) | 高 |
| B | `.claude/skills/multi-perspective-review/SKILL.md` §4 テンプレ冒頭の「8 sub-agent」→「9 sub-agent」 (refactoring-reviewer 追加分が漏れ) | coding-conventions | 中 |
| C | `modules/shared/src/utilgrpc/client.go:1-9` package doc コメントの `shared/contract/<svc>/v1/` を実在パス (`modules/<svc>/src/route/grpc/`) に修正、または当該文を削除 | maintainability | 中 |
| D | `deploy/k8s/istio.md` §4 step 5 「NodePort 経由のホスト到達が継続するか確認」を `make istio-port-forward` ベースの smoke-test に書き換え | maintainability / coding-conventions | 中 |
| E | `CLAUDE.md` Repository Layout セクションに `doc/review/` への 1 行リファレンスを追加 (新規 top-level dir のためレイアウト記載が望ましい) | coding-conventions | 低 |
| F | `Makefile` 内 `GATEWAY_API_VERSION` の脇に「バンプ時はここを書き換える」コメントを追加 (`ISTIO_VERSION` の Dockerfile 注釈と対称にする) | maintainability | 低 |

### 1.2 code-side / 別 PR でフォローアップ

| # | 対応内容 | 検出元 | 重要度 |
|---|---|---|---|
| G | `Makefile` `istio-up` の `kubectl rollout status` 三行から `-@` を外す (現状ロールアウト失敗を握りつぶし `k8s-apply` まで進んでしまう。ユーザは clean exit と勘違いし mesh 半完成状態でゲートウェイ未プロビジョン) | reliability | 高 |
| H | `.devcontainer/compose.yaml` の `${HOME}/.claude:/home/vscode/.claude` を read-only (`:ro`) でマウント、または `.credentials.json` だけ別系統で渡す。Codespaces / 共有 dev 環境で API key 露出するため | security / compliance | 高 |
| I | `otel/k8s/base/namespace.yaml` (observability namespace) に `istio.io/dataplane-mode: ambient` を付与する、または `deploy/k8s/istio.md` §4 で「STRICT 移行時は observability namespace 用 PA 例外を作成」を明示 | security | 中 |
| J | `deploy/k8s/dev/peerauthentication.yaml` のコメントと `istio.md` §4 STRICT 行に「staging/prod overlay は STRICT 必須」を bind 表現で追加 (現状アドバイザリ止まり) | security / compliance | 中 |
| K | `modules/shared/src/utilotel/http_test.go:127-157` の `TestHTTPMiddleware_spanNameFallsBackToServerNameWithoutPattern` 偽陽性を修正 (`r.Pattern == ""` を実際に発生させて assertion を意味あるものに)。前回 review (`2026-05-08-doc-refresh-...`) item I と同一 | testability | 中 |
| L | `modules/audit/src/infra/queueclient/client.go` 用テストを追加 (現状 `infra/` consumer wrapper でテストゼロは testing.md §1 違反) | testability | 中 |
| M | `otel/prometheus/prometheus.yml` istiod scrape job に bearer-token auth を staging/prod 昇格時に有効化する旨のコメントを追加 | security | 低 |
| N | `.claude/skills/multi-perspective-review/SKILL.md` に「永続化前に PII / credential を redact する」ガイドラインを追加。`doc/review/` 永続物のリテンションポリシーも明記 | compliance | 低 |
| O | `Makefile` `k8s-up` の `kubectl wait` ブロックも `-@` で失敗を抑制している。少なくともコメントで「advisory wait」と明示する | reliability | 低 |
| P | `Makefile` `kubectl apply -f $(GATEWAY_API_URL)` がインストール時に GitHub からフェッチ。チェックサム検証なし。fragility note として `istio-up` コメントに追記、または CRD YAML を vendor 化 | reliability | 低 |

### 1.3 Refactoring proposals (advisory — open follow-up PR)

`refactoring-reviewer` の提案。各項目は単一 commit サイズの transformation。

| # | Refactoring | path:line | Cost | Test impact |
|---|---|---|---|---|
| R1 | Makefile に `ISTIO_VERSION := 1.29.2` を追加し `istio-up` で実行 PATH の istioctl とのバージョン guard をかける | `Makefile` (新規 var + istio-up 内) | S | none |
| R2 | `insecure.NewCredentials()` を package-level の `var plaintext = grpc.WithTransportCredentials(...)` に括り出し「常に plaintext」不変条件を grep で見つけられるようにする | `modules/shared/src/utilgrpc/client.go:60-62` | S | none |
| R3 | `k8s-up` の wait ブロックを `k8s-wait-ready` サブターゲットに extract して recipe を `k8s-cluster k8s-build k8s-load istio-up k8s-apply k8s-wait-ready` に短縮 | `Makefile` k8s-up (`Makefile:266-275`) | S | none |
| R4 | 3 ファイルで重複している namespace.yaml の ambient enrollment 5 行コメントを `# Ambient enrollment — see deploy/k8s/istio.md §1` に短縮 | `modules/{audit,auth,queue}/deploy/k8s/base/namespace.yaml` | S | none |

---

## 2. 横断パターン (cross-reviewer)

- **Stale NodePort references** — `gateway.yaml` 冒頭コメント / `istio.md` §4 step 5 / `peerauthentication.yaml` 周辺の表現に「NodePort + extraPortMapping」が残存。security / coding-conventions / maintainability / reliability / compliance が独立に検出。実装は `make istio-port-forward` に統一済みなので**ドキュメント側だけが過去の選択肢を引きずっている**。1 commit で全箇所を `port-forward` 表現に揃えるのが良い。
- **STRICT 移行が「アドバイザリ」止まり** — `peerauthentication.yaml` (PERMISSIVE) と `istio.md` §4 が staging/prod の STRICT 移行を「checklist」「how-to」の形でしか記述していない。security と compliance が「機械的・社会的に enforce される拘束義務になっていない」と独立に指摘。1 文「staging/prod overlay は STRICT 必須」を `peerauthentication.yaml` コメントと `istio.md` §4 に追加するのが最低限。
- **前回 review の未完了アイテムが本 PR に持ち越されている** — testability が `http_test.go` の偽陽性 (`r.Pattern == ""` を発生させていない) を再指摘。`doc/review/2026-05-08-doc-refresh-multi-perspective.md` の item I と同一。`/multi-perspective-review` で永続化した checklist が follow-up に流れていない兆候。**プロセス改善**: 直近 review の未完了アイテムを「review 開始時に再確認する」ステップを skill に追加する余地あり。
- **`doc/review/` 永続化が compliance / 規約面でカバーされていない** — `CLAUDE.md` レイアウトに記載なし (coding-conventions)、PII redact / retention のガイダンスなし (compliance)。ドキュメント永続化を skill に組み込んだ commit が、永続化先のガバナンスを定義し切れていない。

---

## 3. Top findings (severity-ordered)

```
[production-impact] [reliability]   Makefile istio-up:323-325 — kubectl rollout status に `-@` プレフィクス。istiod / ztunnel / istio-cni-node のロールアウト失敗が握りつぶされ k8s-apply が続行 → mesh 半完成 + gateway 未プロビジョンで clean exit。 — Fix: `-` を外して fail-fast。
[production-impact] [compliance]    .devcontainer/compose.yaml:13   — ${HOME}/.claude マウントで .credentials.json (Anthropic API key) を全 sub-agent 実行コンテキストへ露出。Codespaces 等で port 8081 と組み合わさると外部到達リスク。 — Fix: `:ro` 化、もしくは credentials.json を別経路で渡す。
[quality]           [security]      otel/k8s/base/namespace.yaml   — observability namespace に ambient ラベルなし。サービス pod → Collector 通信が PERMISSIVE / STRICT 双方で plaintext。STRICT 切替時に静かに壊れる。 — Fix: ambient ラベル追加 or 例外を文書化。
[quality]           [security/compliance] deploy/k8s/dev/peerauthentication.yaml — STRICT 移行が「アドバイザリ」止まり。staging/prod 必須化の記述が無い。 — Fix: コメント + istio.md §4 に bind 文を追加。
[quality]           [coding/maintain/security/reliability/compliance] modules/auth/deploy/k8s/base/gateway.yaml:5-7 — 廃止された NodePort 方式を参照する stale コメント。5 reviewer が独立検出。 — Fix: `make istio-port-forward` を指す 1 行に置換。
[quality]           [maintainability] modules/shared/src/utilgrpc/client.go:1-9 — package doc が存在しないパス `shared/contract/<svc>/v1/` を参照。 — Fix: 実在パス `modules/<svc>/src/route/grpc/` に修正、または該当文削除。
[quality]           [maintainability/coding] deploy/k8s/istio.md:100 — STRICT 移行 checklist step 5 が NodePort 経由 smoke-test を指示 (廃止済み)。 — Fix: `curl -fsS http://localhost:8081/health` (port-forward) に書き換え。
[quality]           [coding-conventions] .claude/skills/multi-perspective-review/SKILL.md — §4 テンプレ説明文「8 sub-agent」が更新漏れ (本 commit で 9 reviewer に拡張済み)。 — Fix: 「9 sub-agent」に更新し、refactoring を列挙に追加。
[quality]           [testability]   modules/shared/src/utilotel/http_test.go:127-157 — `TestHTTPMiddleware_spanNameFallsBackToServerNameWithoutPattern` が常に pass する偽陽性。前回 review item I の繰越。 — Fix: `r.Pattern == ""` の経路を実際にトリガーする入力を組む。
[quality]           [testability]   modules/audit/src/infra/queueclient/client.go — テストファイル未配置。`infra/` consumer wrapper でテストゼロは testing.md §1 違反。 — Fix: `localhost:0` への lazy dial smoke-test を追加。
[drift]             [security]      Makefile:344                   — `kubectl port-forward --address 0.0.0.0` が devcontainer 内で全 IF bind。Codespaces public visibility と組み合わさると外部到達。 — Fix: ローカルは `127.0.0.1`、Codespaces 利用時の risk note を `istio.md` §3 に追記。
[drift]             [security]      otel/prometheus/prometheus.yml:54-57 — istiod:15014 を auth/TLS 無しで scrape。dev 範囲では許容されるが staging 昇格時に bearer-token を有効化する明示が無い。 — Fix: scrape job コメントに「staging 移行時に SA token を projected で渡す」を追記。
[drift]             [coding]        CLAUDE.md Repository Layout    — 新設の `doc/review/` ディレクトリが Layout 記述に不在。 — Fix: 1 行追加。
[drift]             [maintainability] Makefile GATEWAY_API_VERSION  — Dockerfile の `ISTIO_VERSION` には bump コメントがあるが Makefile の Gateway API には無い。 — Fix: 同等の 1 行コメント追加。
```

`refactoring-reviewer` の R1–R4 は別 PR として §1.3 の表を参照。

---

## 4. Per-reviewer reports (verbatim)

9 sub-agent (security / performance / coding-conventions / maintainability / testability / reliability / scalability / compliance / refactoring) を並列 dispatch した結果。各 agent の verbatim 報告を保存する。

### 4.1 Security — `needs-fix`

> **Blocking findings (must fix before merge):** None.
>
> **Non-blocking findings:**
>
> 1. `.devcontainer/compose.yaml:13` — Claude Code credentials directory mounted into dev container. The volume `${HOME}/.claude:/home/vscode/.claude` mounts the host user's full `~/.claude` directory (including session JSONL, credentials, settings.local.json) into the devcontainer. With `/var/run/docker.sock` already mounted, the credential exposure is the new finding. Suggested improvement: scope the mount to subdirectories that need to survive (e.g. `~/.claude/todos`, `~/.claude/history`) or document the credential persistence behavior.
> 2. `modules/auth/deploy/k8s/base/gateway.yaml:1-7` — Stale comment references a NodePort Service that does not exist. The header says "the dev overlay adds a NodePort Service alongside so kind's extraPortMapping (host :8081 → node :30080) reaches it." No such NodePort Service exists; the abandonment is documented in `deploy/k8s/istio.md` §3. Delete lines 5-7 of the comment block.
> 3. `deploy/k8s/dev/peerauthentication.yaml` — Mesh-wide PERMISSIVE mode with no namespace-scoped fallback; `observability` namespace carries no `istio.io/dataplane-mode: ambient` label, so service-to-Collector traffic is plaintext in both PERMISSIVE and STRICT modes. STRICT flip would silently break this path. Suggested: add ambient label to `otel/k8s/base/namespace.yaml`, or document the exception in `deploy/k8s/istio.md` §4.
> 4. `Makefile:344` — `kubectl port-forward --address 0.0.0.0` binds the auth gateway to all interfaces inside the devcontainer. Combined with the `8081:8081` publish in `.devcontainer/compose.yaml`, the unauthenticated `/auth/v1/token/login` endpoint may be reachable from the host. In Codespaces this risks external reach if visibility is set to public. Document the risk or use `--address 127.0.0.1` outside devcontainers.
> 5. `otel/prometheus/prometheus.yml:54-57` — Unauthenticated Prometheus scrape of `istiod:15014` with no TLS. Acceptable for dev; note that bearer-token auth (via projected ServiceAccount token) should be added when promoting to staging.
>
> **Confirmed-good:** `.claude/settings.local.json` contains only a Bash permission grant (no secrets). `utilgrpc.WithTLS` removal is deliberate and correct: `grpc.WithTransportCredentials(insecure.NewCredentials())` is hardcoded; `WithDialOption` cannot override it. `PeerAuthentication` scope (`namespace: istio-system`, no selector) is correct mesh-wide default. `Gateway` `allowedRoutes.namespaces.from: Same` correctly restricts cross-namespace route injection. `istio.io/dataplane-mode: ambient` labels present on all three service namespaces. No high-entropy strings, AWS key prefixes, JWT-shaped strings, or private-key headers in the diff. `insecure.NewCredentials()` now unconditional — no zero-init footgun.

### 4.2 Performance — `ship`

> **Hot-path:**
>
> - `modules/shared/src/utilotel/grpc.go:26,38` — `otelgrpc.NewClientHandler()` / `NewServerHandler()` are called once at `NewServer` / `Dial` time (startup), not per-request. Per-RPC cost is the StatsHandler dispatch — one function call per RPC event. No allocation concern per request.
> - `modules/audit/src/infra/queueclient/client.go:53` — `opts = append([]Option{utilotel.GRPCClientOption()}, opts...)` allocates a new slice every `New()` call. `New()` runs once per process at startup. Cold-path.
>
> **Cold-path:**
>
> - `modules/shared/src/utilgrpc/client.go:54,71` — `Dial` wraps `grpc.NewClient` (lazy). Behavior identical post-`WithTLS` removal: `insecure.NewCredentials()` constructed once per `Dial`, not per request. No regression.
> - No `WithKeepaliveParams` / `WithConnectParams` after the refactor — pre-existing gap, not new. Under mesh, ztunnel handles TCP keepalive. For Compose path long-idle connections may be dropped at Linux default `tcp_keepalive_time` (~2h). Not a new regression.
> - `otel/prometheus/prometheus.yml:25-29` — istiod scrape adds ~200 series at 15s. Bounded by istiod's `/metrics` (not user-proportional). No issue.
>
> **Worth measuring:** `otelgrpc` defaults emit spans for high-frequency health probes; filtering is done at Collector. Verify filter-processor config before scaling N>1.
>
> **Confirmed-good:** `WithTLS` removal is clean and behavior-preserving. `grpc.NewClient` is the correct upstream call (not deprecated `grpc.Dial`). istiod scrape uses static_configs (bounded cardinality). Coverage aggregation under `.coverage/` is startup/CI-only — no request-path concern.

### 4.3 Coding conventions — `minor-drift`

> **Convention violations (must fix):**
>
> - `modules/auth/deploy/k8s/base/gateway.yaml:6-7` — Comment describes a NodePort Service that does not exist and was explicitly abandoned. Replace with a one-liner: `# dev host access: make istio-port-forward (see deploy/k8s/istio.md §3).`
>
> **Drift / inconsistency:**
>
> - `CLAUDE.md` Repository Layout — new top-level `doc/` directory not described. Add: `- doc/review/ — dated multi-perspective review logs (generated by /multi-perspective-review skill; do not hand-edit).`
> - `.claude/skills/multi-perspective-review/SKILL.md` — `doc/review/` §4 template header still reads "8 sub-agent (security / performance / coding-conventions / maintainability / testability / reliability / scalability / compliance) を並列 dispatch". Now nine — refactoring-reviewer was added in the same commit. Update the count and add refactoring to the list.
>
> **Confirmed-good:** `WithTLS` removal preserves the functional-options idiom (`Option = func(*config)`, unexported `config`); plaintext is now unconditional rather than configurable default; no call sites missed (grep confirms zero remaining `WithTLS`). Tests for the deleted option are removed cleanly. `Gateway` named `auth-gateway`, `HTTPRoute` named `auth-route` — matches `<svc>-gateway` / `<svc>-route` convention. No `app.kubernetes.io/part-of` label. Required `name`/`component`/`managed-by` labels present. `istio.io/dataplane-mode: ambient` on all three service namespaces. `peerauthentication.yaml` placed at `deploy/k8s/dev/` per §13. `deploy/k8s/istio.md` at the env-agnostic doc root. `refactoring-reviewer.md` frontmatter consistent with sibling agents. Coverage aggregation `.gitignore` correctly scoped (`/.coverage/`).
>
> **Out of scope:** `deploy/k8s/istio.md` §4 STRICT migration step 5 says "NodePort 経由のホスト到達が継続するか確認" — design-doc accuracy issue, flagged for `maintainability-reviewer`.

### 4.4 Maintainability — `small-debt`

> **Findings (worth fixing in this change):**
>
> 1. `modules/auth/deploy/k8s/base/gateway.yaml:5-8` — stale comment describes the NodePort pattern that was abandoned. Replace with: "Dev host access is via `make istio-port-forward` — see `deploy/k8s/istio.md` §3."
> 2. `modules/shared/src/utilgrpc/client.go:1-9` — package doc references a non-existent path `shared/contract/<svc>/v1/`. Per-service gRPC stubs live under `modules/<svc>/src/route/grpc/`. Will appear in generated docs. Fix the path or remove the sentence.
> 3. `deploy/k8s/istio.md:100` — STRICT migration checklist step 5 references NodePort for smoke-test verification. Should reference `make istio-port-forward` / `curl http://localhost:8081/health`.
>
> **Trends:**
>
> - **Doc duplication across 4 files for the same Istio rationale.** `CLAUDE.md` (§ Service mesh), `kubernetes-conventions.md` §13, `deploy/k8s/istio.md`, `deploy/k8s/README.md` §5.1. Currently consistent, but a future change requires four-place edits. Collapse the narrative into one source (`deploy/k8s/istio.md`) and have the others cross-reference. Not urgent today.
> - **`K8S_NAMESPACES` includes `istio-system`** — `make k8s-status` now partially duplicates `make istio-status` without the extra context (gatewayclass, httproute). Either remove `istio-system` from `K8S_NAMESPACES` and have `k8s-status` invoke `istio-status`, or accept the duplication.
>
> **Future-debt watchlist:**
>
> - `utilgrpc/client.go` — after `WithTLS` removal the wrapper is a thin pass-through. Interceptor accumulation is the genuine value. Re-evaluate if no third `Withxxx` materialises in two features.
> - `Makefile:308-310` — `GATEWAY_API_VERSION` / `GATEWAY_API_URL` have no doc pointer like `ISTIO_VERSION` does. Add a comment.
> - `deploy/k8s/dev/peerauthentication.yaml:16` — PERMISSIVE has no machine-readable marker for the STRICT migration. Social convention only.
> - `doc/review/2026-05-08-doc-refresh-multi-perspective.md` — its provenance anchor refers to "会話履歴". The Istio adoption rationale at 210 lines warrants a `decisions/` ADR rather than chat log.
>
> **Confirmed-good:** `WithTLS` removal clean (field gone, credentials import dropped, tests deleted, both surviving doc references frame it as anti-pattern). Surviving `utilgrpc` tests cover all three remaining options. `istio-up` extracts `ISTIO_PROFILE` / `GATEWAY_API_VERSION` to named variables; comment explains Gateway API CRDs before istiod. `peerauthentication.yaml` placement justified in `istio.md`. `refactoring-reviewer` agent definition tightly scoped. `clean-coverage` sweeps stale per-module artifacts. `istio-down` uses `-` (ignore make errors) on uninstall and namespace delete. All three service namespaces consistently labeled.

### 4.5 Testability — `well-tested`

> **Coverage gaps:**
>
> 1. `modules/shared/src/utilotel/http_test.go:127-157` — `TestHTTPMiddleware_spanNameFallsBackToServerNameWithoutPattern` asserts the wrong thing. Test comment admits stdlib ServeMux 1.22+ still populates `r.Pattern`. The branch at `http.go:63` (`r.Pattern == ""`) has no test that distinguishes correct vs broken implementation. Same finding as previous review item I; persists in this diff.
> 2. `modules/shared/src/utilotel/http.go:66` — chi-v5 branch where `r.Pattern` is path-only (no method prefix) has no test. Existing test exercises only the stdlib ServeMux form. If the `HasPrefix` branch were dropped accidentally, the stdlib test would still pass.
> 3. `modules/audit/src/infra/queueclient/client.go` — no test file exists. The OTel-prepend behavior arrived in a prior commit; coverage gap is not this diff's fault but persists. Per testing.md §1, `infra/` consumer wrappers are always expected to have tests.
>
> **TDD compliance:** `WithTLS` removal is a deletion of a public API; no new behavior to drive. Surviving tests still exercise the remaining public API. Acceptable per development-workflow.md §1.
>
> **Seam issues:** `utilgrpc.Dial` hardcodes `insecure.NewCredentials()` post-removal — intentional, mesh provides mTLS. Not a regression. `queueclient.New()` references global OTel provider — manageable seam (swappable via `otel.SetTracerProvider`).
>
> **Test smells:** `http_test.go:154` assertion always passes; test name promises behavior the test cannot falsify.
>
> **Coverage Makefile change is a behavioral improvement:** test failure now propagates non-zero exit (previously could print stale `total:` line on failure). `total:` output preserved identically. `.gitignore` consistent (`/.coverage/`).
>
> **Confirmed-good:** `WithTLS` test deletion clean (no orphaned scaffolding). Three surviving option tests cover the live API. `TestDial_acceptsAllOptionTypesTogether` correctly removes TLS arm. `queueclient` doc-comment update is doc-only. Istio k8s manifests outside testability scope per §1. `clean-coverage` sweeps stale paths.

### 4.6 Reliability — `needs-fix`

> **Page-worthy:**
>
> - `modules/auth/src/cmd/api/main.go:142` — Shutdown always burns the full 30-second window. After cleanup, blocks on `<-shutdownCtx.Done()` which only fires at the deadline. Every graceful shutdown takes 30 seconds, delaying rolling deployments. *(Pre-existing — file not in this diff's change set, but flagged.)*
> - `Makefile:323-325` (istio-up rollout waits) — Three `rollout status` lines prefixed with `-@`. If istiod / ztunnel / istio-cni-node fails to roll out within 180s, `make k8s-up` continues to `k8s-apply` and exits 0 with a broken mesh. Fix: drop the `-` so rollout failures abort.
>
> **Degradation issues:**
>
> - `modules/audit/src/cmd/api/main.go:84-86` — OTel shutdown shares the 30-second `shutdownCtx`; if `GracefulStop()` exhausts most of it, OTel flush is silently truncated. Give OTel its own sub-context (e.g., 5s).
> - `modules/auth/deploy/k8s/base/gateway.yaml:1-7` — Stale comment will mislead a future contributor into the abandoned NodePort path.
> - `Makefile:265` — `k8s-up` `kubectl wait` lines are also `-@`. Failed waits don't abort. Document as advisory.
>
> **Recovery story gaps:**
>
> - `peerauthentication.yaml` + namespace.yaml — what if `istio-up` is skipped and `k8s-apply` runs directly? Gateway API CRDs and Istio CRDs would be missing; `kubectl apply` partial-failure undocumented. Add a CRD-presence check in `k8s-apply`.
> - `queueclient/client.go:52-58` — `grpc.NewClient` is lazy; first RPC surfaces dial failures. Acceptable for stub; when worker becomes real, add `WithBlock + WithTimeout` or startup health check.
> - `Makefile:320` — `kubectl apply -f $(GATEWAY_API_URL)` fetches from internet at install time, no checksum. Not blocking; consider vendoring CRD YAML.
>
> **Confirmed-good:** `WithTLS` removal complete and clean — `config.transportCreds` gone, `Dial` hardcodes `insecure.NewCredentials()`, no orphaned callers. `utilgrpc.Dial` contract preserved post-removal. OTel trace context propagation unaffected (`GRPCClientOption()` first in chain). All three service namespaces consistently labeled. `PeerAuthentication` scope correct. `make istio-up` ordered correctly in `k8s-up` chain. `audit-api` uses `GracefulStop()` with `Stop()` fallback. `HTTPRoute` `PathPrefix: /` keeps `/health` reachable. No new retry loops or goroutines.

### 4.7 Scalability — `scales-cleanly`

> **Replica-coherence findings:** None introduced by this diff. The three items below are flagged because the diff touches surrounding code/manifests, but none is a new regression.
>
> **Capacity findings:**
>
> 1. `modules/auth/src/infra/database/database.go:11` — Unbounded DB connection pool (no `SetMaxOpenConns`). At 2 replicas fine; past ~10 replicas exhausts auth-db's default `max_connections=100`. Pre-existing; the Istio gateway makes auth-api the first unbounded fan-out point. Fix: `SetMaxOpenConns(n)` where `n = max_connections / max_expected_replicas`.
> 2. `modules/auth/deploy/k8s/base/gateway.yaml` — Auto-provisioned Envoy gateway has one replica, no HPA. Fine for dev. Production path: `infrastructure.parametersRef` (Gateway API v1.1+) or post-hoc HPA on the auto-provisioned Deployment. Stale NodePort comment is misleading; flagged.
> 3. `otel/k8s/base/collector.yaml:10` — OTel Collector at `replicas: 1` is a SPOF for telemetry. Pre-existing. Now subscribing both app telemetry and istiod scrape. Acknowledged in prior review.
>
> **Backpressure / rate-limit:**
>
> - istiod scrape at 15s: bounded by Istio-managed services and proxy certificates, not by app replica count. No N-multiplier.
> - audit-worker `replicas: 1` intentional and documented.
>
> **Confirmed-good:** `utilgrpc/client.go` `WithTLS` removal eliminates a code path where `WithTLS(nil)` could nil-deref. `grpc.ClientConn` multiplexes; per-process connection sharing correct. `queueclient.New()` constructs one ClientConn per process. `PeerAuthentication` PERMISSIVE applied uniformly across enrolled namespaces — replica-coherent. Namespace ambient labels per-namespace, atomic. Liveness probes (auth `/health`, audit/queue native gRPC) decoupled from shared state. `make test` sequential is correct for monorepo with separate `go.mod` per module. istiod scrape down-target acknowledged in Compose path comment.

### 4.8 Compliance — `gap`

> **Compliance gaps (must close before merge):**
>
> 1. `.devcontainer/compose.yaml:13` — Credential mount into shared container context. `~/.claude/.credentials.json` is confirmed present (470 bytes) and exposed at this path. With `/var/run/docker.sock` mount and port 8081 publish, prompt injection in a sub-agent could exfiltrate the API key. Fix: scope the mount to subdirs that need persistence, or use `:ro` and verify CLI session resumption still works, or move credentials out-of-band. Ref: `ai-agent.md` §6, `kubernetes-conventions.md` §8.
> 2. `modules/auth/deploy/k8s/base/gateway.yaml:6-7` — Stale comment contradicts implemented design. No NodePort Service exists in `modules/auth/deploy/k8s/overlays/dev/`. Auditor reading the manifest will have a false picture of the exposure path.
>
> **Audit-trail gaps:** No new gaps introduced. Login-attempt auditing is a pre-existing gap in `auth/service/login.go` (no `audit.IngestEvent` call) but out of scope for this diff.
>
> **PII handling:**
>
> - `.claude/skills/multi-perspective-review/SKILL.md:70,89` — `doc/review/` persistence has no PII redaction gate. Skill passes full `git diff` to nine sub-agents and persists "verbatim" reports. If a future diff has fixtures with real emails / phones / tokens, those values land in git permanently. No redaction step in the skill. `doc/review/` is not gitignored. Fix: add a pre-persistence redaction step.
> - `otel/README.md:150-151` — PII retention in Tempo now documented (added in `f9a5cef`). `always_on` + `net.peer.addr` / `http.user_agent` note. Confirmed-good for current HEAD; warrants a staging/prod gate (Tempo retention TTL). Currently advisory prose. Consider a `TODO(STRICT-BEFORE-PROD)` comment in the base k8s ConfigMap for `OTEL_TRACES_SAMPLER`.
>
> **Documentation alignment:**
>
> - `deploy/k8s/istio.md` §4 / `peerauthentication.yaml` — PERMISSIVE → STRICT framed as advisory, not as a blocking pre-prod gate. No document states "staging and prod overlays MUST set `mode: STRICT`". Fix: add binding sentence.
> - `SKILL.md` — no retention policy for `doc/review/` files. Fix: add "What not to persist" block.
>
> **Confirmed-good:** `.claude/settings.local.json` only has a Bash permission allow — acceptable scope. `WithTLS` removal eliminates accidental credential leakage path through the `transportCreds` field. `peerauthentication.yaml` correctly scopes PERMISSIVE to dev with rationale. All three service namespaces correctly labeled `ambient`. `kustomization.yaml` still lists `networkpolicy.yaml` after `gateway.yaml` — defense-in-depth preserved. `gateway.yaml` `allowedRoutes.namespaces.from: Same` correctly restricts route injection. Persisted review file contains no PII / credentials / user data. `otel/README.md` PII/retention note (commit `f9a5cef`) is a meaningful improvement.

### 4.9 Refactoring — `small-improvements`

> **Proposed refactorings (commit-sized, behavior-preserving):**
>
> 1. **Replace magic literal with named constant**: `Makefile` (no line — inline in `istio-up`). `1.29.2` is mentioned in `.devcontainer/Dockerfile` ARG and `CLAUDE.md`; Makefile relies on PATH istioctl silently. Add `ISTIO_VERSION := 1.29.2` and a guard in `istio-up`. Cost S, no test impact.
> 2. **Inline / simplify**: `modules/shared/src/utilgrpc/client.go:60-62`. After `WithTLS` removal the `insecure.NewCredentials()` call is buried in `Dial`. Lift to package-level `var plaintext = grpc.WithTransportCredentials(insecure.NewCredentials())` so the "always-plaintext" invariant is grep-visible. Cost S, no test impact. Judgment call.
> 3. **Extract function**: `Makefile / k8s-up` lines 266-275. Pull the wait block into `k8s-wait-ready` so `k8s-up` reads as `k8s-cluster k8s-build k8s-load istio-up k8s-apply k8s-wait-ready`. Cost S, no test impact.
> 4. **Deduplicate YAML comment**: `modules/{audit,auth,queue}/deploy/k8s/base/namespace.yaml`. Identical 5-line comment repeated three times. Note: `commonLabels` is forbidden on workload selectors but namespaces have no selector — however a kustomize transformer would also touch Deployments/Services and collide. Correct fix is to shorten the comment to a `deploy/k8s/istio.md` §1 pointer. Cost S, no test impact.
> 5. **Withdrawn**: `GATEWAY_API_VERSION` / `GATEWAY_API_URL` are already correctly named.
>
> **Design-discussion-needed (out of scope for single commit):**
>
> - Namespace `istio.io/dataplane-mode: ambient` via kustomize `transformers:` — viable but needs scoping audit so the transformer doesn't add the label to Deployments/Services. Not commit-sized without that.
> - `utilgrpc.Dial` wrapper vs. call-site inlining — wrapper is now 20 lines, 1 functional option type, 0 TLS logic. Architectural discussion (changes import graph and per-service isolation of grpc-go API). Surfaced for future consideration.
>
> **Confirmed-good:** `WithTLS` removal atomic and clean — field, option constructor, tests, and call-site comment all updated together. `GATEWAY_API_VERSION` / `GATEWAY_API_URL` / `ISTIO_PROFILE` are correct named-constant patterns. `queueclient.New` correctly prepends `utilotel.GRPCClientOption()` (OTel stats handler must be outermost). `utilotel.GRPCClientOption()` delegates to `utilgrpc.WithDialOption` — `utilotel` does not import `google.golang.org/grpc` directly, maintaining the layering rule from coding-standards §2. Three namespace files use per-resource labels (avoiding selector-collision trap). `doc/review/` directory: not a code smell — template (in SKILL.md) and dated artifact have different purposes.
