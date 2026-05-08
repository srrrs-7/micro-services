# shared

リポジトリ内の全 Go モジュール (`auth`, `audit`, `queue`) から横断的に利用される **ライブラリ専用モジュール**。バイナリは無く、`cmd/` ディレクトリも `deploy/` も持たない。

`go.work` がワークスペース解決を担うため、各モジュールは `import "shared/utilxxx"` のように **プレフィックス無し** で参照する (`coding-standards.md` §2)。

## パッケージ一覧

| パッケージ | 役割 | 主な公開 API |
|---|---|---|
| [`utilhttp`](src/utilhttp/) | HTTP レイヤの共通基盤 | `AppError` + 8 種の `New*Error` factory, `RequestBody[T]` / `RequestUrlParam[T]` (Validator 制約付きジェネリクス), `ResponseOk` / `ResponseError`, `SuccessResponse[T]` / `ErrorResponse`, `Validator` インターフェース |
| [`utillog`](src/utillog/) | 構造化ログ | `NewLogger()` — `slog.Default` を JSON ハンドラ + LevelDebug で差し替え (各 binary の `init()` で 1 度だけ呼ぶ) |
| [`utilcache`](src/utilcache/) | Redis アクセス | `NewClient(addr, pass)` (Ping 付き), `Cache` (prefix + 共通 TTL ラッパ), `(Cache).Set/Get/Del/MakeKey` |
| [`utilgrpc`](src/utilgrpc/) | gRPC クライアント基盤 | `Dial(addr, opts...)` (functional options), `WithTLS` / `WithUnaryInterceptors` / `WithStreamInterceptors` / `WithDialOption`, `LoggingInterceptor` (アウトバウンド構造化アクセスログ) |
| [`utilotel`](src/utilotel/) | OpenTelemetry SDK 配線 | `Init(ctx, serviceName)` (`OTEL_*` 環境変数から TracerProvider + MeterProvider を構築。endpoint 未設定時は noop fallback), `HTTPMiddleware(serverName, opts...)` (chi v5 / stdlib ServeMux 1.22+ の `r.Pattern` を読んで `<METHOD> <pattern>` 形式の span 名にする — 別途 retag middleware は不要), `WithRequestFilter`, `GRPCServerOption()` / `GRPCClientOption()` (otelgrpc StatsHandler) |

各サービスは上記をそのまま import するか、**自モジュールの `infra/<svc>client/` 配下に薄いラッパ** を置いてからその先で利用する (gRPC consumer pattern — 詳細は `coding-standards.md` §2 と [`CLAUDE.md`](../../CLAUDE.md) のリファレンス例)。

## ディレクトリ

```
modules/shared/
└── src/
    ├── go.mod                      module name = shared
    ├── utilhttp/
    │   ├── error.go / response.go / request.go
    │   └── *_test.go
    ├── utillog/log.go (+ test)
    ├── utilcache/
    │   ├── client.go               *redis.Client constructor (Ping 付き)
    │   ├── cache.go                Prefixed Set/Get/Del wrapper
    │   └── *_test.go (+ integration)
    ├── utilgrpc/
    │   ├── client.go               Dial + functional Options
    │   ├── interceptor.go          LoggingInterceptor
    │   └── *_test.go
    └── utilotel/
        ├── init.go                 TracerProvider + MeterProvider 構築 (noop fallback あり)
        ├── http.go                 chi v5 / stdlib ServeMux 1.22+ 互換 HTTP middleware
        ├── grpc.go                 GRPCServerOption / GRPCClientOption
        └── *_test.go
```

`testutil/` は将来追加予定 (`testing.md` §2.2): サービス横断で再利用したいテストヘルパーが現れた時にここへ。

## 開発

リポジトリルートから:

```bash
make fmt                  # go fmt (shared を含む全モジュール)
make vet                  # go vet
make lint                 # golangci-lint
make test                 # go test + per-module coverage
make tidy                 # go mod tidy
```

shared 単体のビルドターゲットや compose ターゲットは無い (バイナリが無いため)。

統合テスト (`utilcache/cache_integration_test.go`) は `//go:build integration` タグ付き — `go test -tags=integration` で走る。Redis を `localhost:6379` に立ち上げてから:

```bash
docker compose up -d auth_cache
cd modules/shared/src
go test -tags=integration -run TestCache ./utilcache/...
```

## 関連ドキュメント

- リポジトリ規約 — [`.claude/rules/coding-standards.md`](../../.claude/rules/coding-standards.md), [`testing.md`](../../.claude/rules/testing.md)
- ルートの開発手順 / Makefile 一覧 — [`CLAUDE.md`](../../CLAUDE.md)
- gRPC consumer pattern — `coding-standards.md` §2、リファレンス実装は [`modules/audit/src/infra/queueclient/`](../audit/src/infra/queueclient/)

## このモジュールに依存物を追加するとき

shared は **依存方向の終端** であり、`auth` / `audit` / `queue` のいずれにも依存しない。新しい util を追加する場合:

1. **本当にここに置く必要があるか確認する** — サービス固有の薄いラッパは各サービスの `infra/` に置く方が望ましい (例: `audit/src/infra/queueclient/` は「shared に置こうとして外した」結果)。
2. テストをこのモジュール内 (`shared/src/<pkg>/*_test.go`) に置く — 他モジュールから `shared/testutil/...` を引きたい場合のみ `testutil/` を新設する。
3. 第三者依存を追加するなら `cd modules/shared/src && go get <dep> && go mod tidy` の上、ルートで `make tidy` で全モジュールの go.sum を整える。
