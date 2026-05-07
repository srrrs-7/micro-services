# auth

OAuth 2.0 / OpenID Connect 1.0 準拠の **第一者 (first-party) 認可サーバ**。社内 SPA / モバイル / バックエンド向けにアクセストークンを発行し、`audit-api` / `queue-api` などダウンストリームサービスへ認可情報を伝搬する。HTTP API (chi) — このリポジトリで唯一 gRPC を採用していないサービス。

詳細仕様は **[docs/system-design.md](docs/system-design.md)**。本 README はリポジトリ訪問時の入口にとどめ、設計判断は重複させない。

## 状況

`docs/system-design.md` §1.2 のスコープ表に従う。MVP では `authorization_code` (PKCE) / `refresh_token` / `client_credentials` の 3 grant を目標とし、現状は **骨格段階** (login ハンドラ + 認証境界の枠組みのみ)。

| 項目 | 状態 |
|---|---|
| chi router + `/health` + `/auth/v1/...` 構成 | ✅ `route/handler.go` |
| `POST /auth/v1/token/login` (email/password 検証) | ⚠️ 骨格のみ — 検証は通るが `&domain.Token{}` の空トークンを返す |
| password ハッシュ (Argon2id) | ⏳ 現状は平文比較 (設計 §1.2) |
| `/authorize` (auth code + PKCE) | ⏳ MVP |
| `/token` (3 grant 完全対応) | ⏳ MVP |
| `/userinfo`, `/revoke`, `/introspect` | ⏳ MVP |
| Discovery (`/.well-known/openid-configuration`, `/.well-known/jwks.json`) | ⏳ MVP |
| RS256 JWT 署名鍵 (k8s Secret) | ⏳ MVP |
| Redis セッション / `nonce` / `state` | ⏳ MVP (`shared/utilcache` 利用予定) |
| ユニットテスト (domain / request / service / route) | ✅ ~28% カバレッジ — `testing.md` §1 参照 |
| 統合テスト (Postgres / Redis) | ⏳ build tag `integration` で逐次追加 |

## ディレクトリ

```
modules/auth/
├── docs/system-design.md           設計書 (一次資料)
├── deploy/k8s/{base,overlays/dev}  k8s manifests (.claude/rules/kubernetes-conventions.md)
└── src/
    ├── go.mod                      module name = auth
    ├── cmd/api/main.go             env → DI → chi → graceful shutdown (cache→DB の順)
    ├── domain/                     値オブジェクト (UserID/Scope/Role/Expired) + Token / LoginInput / User
    ├── service/                    business logic (db.Querier に依存)
    ├── route/
    │   ├── handler.go              chi.Mux 構築 (`/health`, `/auth/v1/token/login`)
    │   ├── service.go              loginService インターフェース (route → service の seam)
    │   ├── login.go                handler 実装
    │   ├── request/                ozzo-validation の Validate() 付き request struct
    │   └── middleware/auth.go      Bearer token middleware (実装中)
    ├── testutil/                   テストヘルパー置き場 (現状空)
    └── infra/database/
        ├── migrations/             Atlas
        ├── queries/login.sql       sqlc 入力 (現状 GetUser のみ)
        ├── sqlc.yaml
        └── db/                     sqlc 出力 (生成物 — 手で編集しない)
```

`infra/database/db/*.go` は **生成物**。`make auth-sqlc-gen` で再生成する。Redis は `cmd/api/main.go` から `shared/utilcache` を直接使うため、`infra/cache/` ディレクトリは存在しない (設計判断 — `coding-standards.md` §1)。

## 開発

リポジトリルートから:

```bash
make auth                 # build + up + migrate (auth-api/auth-db/migrator)
make auth-down            # 停止
make auth-migrate         # Atlas migrate apply (auth-db に対して)
make auth-new-migrate     # 新規 migration
make auth-sqlc-gen        # queries/migrations から db/*.go を再生成
```

**注意**: `make auth-up` は `auth_cache` (Redis) を **起動しない** (`auth_compose_up` に含まれない)。`auth-api` は `compose.yml` 上で Redis を依存しているため、`utilcache.NewClient` の Ping で失敗して exit する。Redis を別途起動:

```bash
docker compose up -d auth_cache
```

`run-service` skill が一連の手順を自動化する。

単体テストはモジュール内から:

```bash
cd modules/auth/src
go test -run TestLoginService_Post -v ./service/...
```

統合テスト (Postgres / Redis 必要) は build tag `integration` 付き — `testing.md` §5 参照。

## 関連ドキュメント

- 設計の一次資料 — [docs/system-design.md](docs/system-design.md)
- 一次仕様 (RFC / OIDC) — `docs/system-design.md` §2 にリンク集
- リポジトリ規約 — [`.claude/rules/coding-standards.md`](../../.claude/rules/coding-standards.md), [`testing.md`](../../.claude/rules/testing.md), [`kubernetes-conventions.md`](../../.claude/rules/kubernetes-conventions.md)
- ルートの開発手順 / Makefile 一覧 — [`CLAUDE.md`](../../CLAUDE.md)

## schema を変更したとき

- migration: `make auth-new-migrate` → SQL 記述 (`COMMENT ON TABLE/COLUMN` 必須) → `make auth-migrate`
- query: `queries/*.sql` 編集 → `make auth-sqlc-gen` → `cd modules/auth/src && go vet ./...`

新しい `request/` を追加する場合は `Validate() error` の実装が必須 (`utilhttp.RequestBody[T]` の constraint)。既存の `route/request/login.go` がリファレンス。
