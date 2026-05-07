# audit

5W1H 観点で社内システムのイベントを記録する **監査基盤**。`audit-api` は gRPC で監査イベントを取り込み (`Ingest`) / 検索 (`GetEvent` / `ListEvents`) し、`audit-worker` は `queue-api` から `audit.events` topic を pull して同じ DB に書き込む。

詳細仕様は **[docs/system-design.md](docs/system-design.md)**。本 README はリポジトリ訪問時の入口にとどめ、設計判断は重複させない。

## 状況

`docs/system-design.md` §13 の Phase 進行に従う。MVP コアとして Phase 1.0 (取り込み + 検索) に到達することを目標とする。

| 項目 | 状態 |
|---|---|
| proto (`audit.v1`, `service Audit`) | ✅ Ingest / GetEvent / ListEvents を定義 |
| gRPC server + interceptor (logging, recovery) | ✅ `route/server.go`, `route/interceptor/` |
| gRPC reflection + `grpc.health.v1` | ✅ (dev) |
| graceful shutdown (30s) | ✅ `cmd/api/main.go` |
| DB schema (`audit_events`, append-only, 5W1H) | ✅ Phase 1.0 (`infra/database/migrations/20260507075754_audit_events.sql`) |
| sqlc クエリ (`InsertEvent` / `GetEventByEventID` / `ListEventsByTimeRange`) | ✅ |
| handler 実装 (現状は `UnimplementedAuditServer`) | ⏳ Phase 1.0 |
| Bearer JWT interceptor (auth JWKS) | ⏳ Phase 1.1 |
| audit-worker (queue → audit-db) | ⏳ Phase 1.1 (`cmd/worker/main.go` は stub) |
| 月次 RANGE パーティショニング + 365 日 retention | ⏳ Phase 1.2 |
| ハッシュチェーン (改竄検知) | ⏳ Phase 1.5 |
| queueclient wrapper (audit → queue gRPC) | ✅ `infra/queueclient/` |

## ディレクトリ

```
modules/audit/
├── docs/system-design.md           設計書 (一次資料)
├── deploy/k8s/{base,overlays/dev}  k8s manifests (.claude/rules/kubernetes-conventions.md)
└── src/
    ├── go.mod                      module name = audit
    ├── cmd/
    │   ├── api/main.go             gRPC server (env → DI → graceful shutdown)
    │   └── worker/main.go          stub — Phase 1.1 で queue consumer 実装
    ├── route/
    │   ├── server.go               grpc.NewServer + interceptor chain + reflection + health
    │   ├── handler.go              *handler (UnimplementedAuditServer 埋め込み)
    │   ├── interceptor/            手書き interceptor (logging / recovery)
    │   └── grpc/                   生成物のみ (package grpc)
    │       ├── audit.proto         一次資料 — 設計書 §6 に対応
    │       ├── audit.pb.go         protoc-gen-go 出力
    │       └── audit_grpc.pb.go    protoc-gen-go-grpc 出力
    └── infra/
        ├── database/
        │   ├── migrations/         Atlas
        │   ├── queries/audit.sql   sqlc 入力
        │   ├── sqlc.yaml
        │   └── db/                 sqlc 出力 (生成物 — 手で編集しない)
        └── queueclient/            audit → queue gRPC を呼ぶ唯一の箇所 (型 alias 経由)
```

`route/grpc/*.pb.go` と `infra/database/db/*.go` は **生成物**。手で編集せず、それぞれ `make audit-proto-gen` / `make audit-sqlc-gen` で再生成する。

## 開発

リポジトリルートから:

```bash
make audit                # build + up + migrate (audit-api/audit-worker/audit-db/queue-api/migrator)
make audit-down           # 停止
make audit-migrate        # Atlas migrate apply (audit-db に対して)
make audit-new-migrate    # 新規 migration ファイル生成 (migrator コンテナ経由)
make audit-sqlc-gen       # queries/migrations から db/*.go を再生成
make audit-proto-gen      # audit.proto から .pb.go を再生成
```

`make audit` は `audit-api` と一緒に `queue-api` も立ち上げる (audit-worker が consumer として queue を必要とするため、`audit_compose_up` に同梱)。

単体テストはモジュール内から:

```bash
cd modules/audit/src
go test -run TestName ./path/to/pkg
```

## 関連ドキュメント

- 設計の一次資料 — [docs/system-design.md](docs/system-design.md)
- 監査契約の発行側 — [`modules/queue/docs/system-design.md`](../queue/docs/system-design.md) §3.4
- リポジトリ規約 — [`.claude/rules/coding-standards.md`](../../.claude/rules/coding-standards.md), [`testing.md`](../../.claude/rules/testing.md), [`kubernetes-conventions.md`](../../.claude/rules/kubernetes-conventions.md)
- ルートの開発手順 / Makefile 一覧 — [`CLAUDE.md`](../../CLAUDE.md)

## proto / schema を変更したとき

- proto: `src/route/grpc/audit.proto` 編集 → `make audit-proto-gen` → 生成物ごとコミット → 必要なら `route/handler.go` にメソッド実装 (`UnimplementedAuditServer` の forward-compat により未実装メソッドは自動で `Unimplemented` を返す)
- schema: `src/infra/database/migrations/*.sql` を `make audit-new-migrate` で新規追加 → SQL 記述 → `make audit-migrate` (`migrate hash` + `apply` を内部で実行)
- query: `src/infra/database/queries/*.sql` 編集 → `make audit-sqlc-gen` → `cd modules/audit/src && go vet ./...`

`audit_events` は append-only。新しいカラムは NULL 許容で追加し、既存行の更新を伴う変更は **しない** (設計 §5.2)。
