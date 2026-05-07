# microservices monorepo

Go ワークスペース上に **3 つのアプリサービス + 1 つの共有ライブラリモジュール** を抱える社内マイクロサービス基盤。Docker Compose と Kubernetes (kind) の両方で同等のローカル動作を提供する。

| モジュール | 役割 | ワイヤプロトコル | README |
|---|---|---|---|
| [`audit`](modules/audit/) | 5W1H 監査ログ基盤 (`audit-api` + 将来の `audit-worker`) | gRPC | [modules/audit/README.md](modules/audit/README.md) |
| [`auth`](modules/auth/) | OAuth 2.0 / OIDC 認可サーバ | HTTP (chi) | [modules/auth/README.md](modules/auth/README.md) |
| [`queue`](modules/queue/) | 優先度付きメッセージブローカ | gRPC | [modules/queue/README.md](modules/queue/README.md) |
| [`shared`](modules/shared/) | 横断ユーティリティ (`utilhttp`, `utillog`, `utilcache`, `utilgrpc`) | — | [modules/shared/README.md](modules/shared/README.md) |

## はじめに

このリポジトリには 4 階層のドキュメントが揃っている。目的に応じて読み始める場所を選ぶ:

- **エージェント / Claude Code が読み込むべき横断規約** → [`CLAUDE.md`](CLAUDE.md) (cwd で常時ロード) と [`.claude/rules/`](.claude/rules/) 配下の詳細ルール
- **特定サービスを触る人** → そのモジュールの README + `docs/system-design.md` + `CLAUDE.md`
- **ローカル開発環境を立ち上げる人** → 下の「ローカル開発」節 → `make help`
- **k8s デプロイ構成を読む人** → [`deploy/k8s/README.md`](deploy/k8s/README.md)

## ローカル開発

devcontainer (`.devcontainer/`) を起動するか、以下のツールをホストに揃える: Go 1.26.0, sqlc, atlas, golangci-lint v2, protoc 28.3 + protoc-gen-go v1.34.2 + protoc-gen-go-grpc v1.5.1, kubectl + kind, docker compose。

**Docker Compose 経路** (Go コードを iterate する時の主経路):

```bash
make hooks                 # pre-commit (fmt+vet+lint) と pre-push (test) フックを設定
make audit                 # build + up + migrate (audit-api/audit-worker/audit-db/queue-api/migrator)
make auth                  # build + up + migrate (auth-api/auth-db/migrator)
docker compose up -d auth_cache   # auth-api は Redis に依存するが auth-up に含まれない
make audit-down            # 停止
```

`make help` で全ターゲットを表示。詳細は [`CLAUDE.md`](CLAUDE.md) の "Common Commands" 節。

**Kubernetes (kind) 経路** (manifest / deploy 時の動作を検証する時):

```bash
make k8s-up                # kind cluster 作成 → 全 image build + load → kustomize apply → 起動待ち
make k8s-status            # 全 namespace の Pod / Service / Job
make k8s-down              # kustomization 削除 (cluster は残る)
```

詳細は [`deploy/k8s/README.md`](deploy/k8s/README.md)。

## レイアウト

```
.
├── modules/
│   ├── go.work                     ワークスペース解決 (4 module)
│   ├── audit/                      gRPC 監査基盤
│   ├── auth/                       HTTP OAuth/OIDC AS
│   ├── queue/                      gRPC メッセージブローカ
│   └── shared/                     横断ライブラリ (バイナリなし)
├── deploy/k8s/dev/                 cross-service kustomization (kind 用 overlay)
├── compose.yml                     Docker Compose (audit / auth スタック)
├── Makefile                        ワークスペース横断 + per-service ターゲット
├── CLAUDE.md                       エージェント向け横断インデックス
└── .claude/
    ├── rules/                      coding-standards / testing / kubernetes-conventions
    ├── skills/                     ワークフロー自動化 (run-service / run-k8s / regen-sqlc / ...)
    └── agents/                     専門化された subagent 定義
```

## 関連ドキュメント

- [`CLAUDE.md`](CLAUDE.md) — エージェント向け規約インデックス
- [`.claude/rules/coding-standards.md`](.claude/rules/coding-standards.md) — Go コード規約
- [`.claude/rules/testing.md`](.claude/rules/testing.md) — テスト方針
- [`.claude/rules/kubernetes-conventions.md`](.claude/rules/kubernetes-conventions.md) — k8s manifest 規約
- 各サービス設計書 — [audit](modules/audit/docs/system-design.md), [auth](modules/auth/docs/system-design.md), [queue](modules/queue/docs/system-design.md)
