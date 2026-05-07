# queue

優先度付きメッセージブローカ。`audit-worker` をはじめとする社内サービスのための非同期メッセージング基盤として、Apache Kafka 風の topic / consumer-group 抽象に **メッセージ単位の優先度** と **個別 ack/nack + visibility timeout** を加えた gRPC API を提供する。

詳細仕様は **[docs/system-design.md](docs/system-design.md)** を参照。本 README はリポジトリ訪問時の入口にとどめ、設計判断は重複させない。

## 状況

現状は **gRPC routing 下地** まで。`Queue` サービスの全 RPC は `codes.Unimplemented` を返す。各 Phase の進行は `docs/system-design.md` §13 に従う。

| 項目 | 状態 |
|---|---|
| proto (`queue.v1`, `service Queue`) | ✅ CreateTopic / Publish / Consume / Ack を定義 |
| gRPC server + interceptor (logging, recovery) | ✅ |
| gRPC reflection + `grpc.health.v1` | ✅ (dev) |
| graceful shutdown (30s) | ✅ |
| DB / マイグレーション (`queue-db`) | ⏳ Phase 1.0 |
| Bearer JWT interceptor (auth JWKS) | ⏳ Phase 1.1 |
| Producer (`Publish` / `PublishBatch`) | ⏳ Phase 1.1 |
| Consumer (`Consume` long-poll, `Ack`/`Nack`) | ⏳ Phase 1.2 |
| `audit.events` topic seed + queue→audit 監査発行 | ⏳ Phase 1.0–1.1 (`docs/system-design.md` §3.4) |
| Retention bg job, DLQ | ⏳ Phase 1.3–1.4 |

## ディレクトリ

```
modules/queue/
├── docs/system-design.md           設計書 (一次資料)
├── deploy/k8s/{base,overlays/dev}  k8s manifests (.claude/rules/kubernetes-conventions.md)
└── src/
    ├── go.mod                      module name = queue
    ├── cmd/api/main.go             env → DI → gRPC server → graceful shutdown
    └── route/
        ├── server.go               grpc.NewServer + interceptor chain + reflection + health
        ├── handler.go              *handler (UnimplementedQueueServer 埋め込み)
        ├── interceptor/            手書き interceptor (logging / recovery)
        └── grpc/                   生成物のみ (package grpc)
            ├── queue.proto         一次資料 — 設計書 §6 に対応
            ├── queue.pb.go         protoc-gen-go 出力
            └── queue_grpc.pb.go    protoc-gen-go-grpc 出力
```

`route/grpc/*.pb.go` は **生成物**。手で編集せず `make queue-proto-gen` で再生成する (sqlc 出力 / atlas.sum と同じ扱い)。

## 開発

リポジトリルートから実行する:

```bash
make queue-proto-gen     # queue.proto から .pb.go を再生成
make vet                 # go vet (queue を含む全モジュール)
make lint                # golangci-lint (同上)
make test                # go test + per-module coverage (同上)
```

queue 単体の Docker Compose ターゲットは未整備 (`SERVICES := auth audit`)。queue-api は `make audit` のスタックに同梱で立ち上がる (`audit_compose_up` に `queue-api` を含むため)。

単体テストはモジュール内から:

```bash
cd modules/queue/src
go test -run TestName ./path/to/pkg
```

## 関連ドキュメント

- 設計の一次資料 — [docs/system-design.md](docs/system-design.md)
- リポジトリ規約 — [`.claude/rules/coding-standards.md`](../../.claude/rules/coding-standards.md), [`testing.md`](../../.claude/rules/testing.md), [`kubernetes-conventions.md`](../../.claude/rules/kubernetes-conventions.md)
- ルートの開発手順 / Makefile 一覧 — [`CLAUDE.md`](../../CLAUDE.md)
- audit との cross-service 契約 — `docs/system-design.md` §3.4 と [`modules/audit/docs/system-design.md`](../audit/docs/system-design.md)

## proto を変更したとき

1. `src/route/grpc/queue.proto` を編集
2. `make queue-proto-gen`
3. 生成物 (`queue.pb.go`, `queue_grpc.pb.go`) ごとコミット
4. ハンドラ追加が必要なら `src/route/handler.go` にメソッドを実装 (`UnimplementedQueueServer` の forward-compat により未実装メソッドは自動で `Unimplemented` を返す)

`devcontainer` には `protoc 28.3` + `protoc-gen-go v1.34.2` + `protoc-gen-go-grpc v1.5.1` が事前導入済み (`.devcontainer/Dockerfile`)。バージョンを上げる際はそこと `Makefile` の `PROTOC_INCLUDE` をそろえる。
