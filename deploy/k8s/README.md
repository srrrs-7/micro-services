# Kubernetes manifests

このドキュメントは本リポジトリの Kubernetes 構成（kind ローカル開発を起点に、staging / prod を見据えた設計）を、マニフェスト1枚ずつではなく **構成全体として** 説明する資料です。個別の規約（命名・ラベル・Service 種別・migration ポリシー等）は `.claude/rules/kubernetes-conventions.md` が拘束力を持つので、両者を併読してください。

## 1. 設計方針

- **サービス所有のマニフェスト**: 各サービスのマニフェストは `modules/<svc>/deploy/k8s/{base,overlays/<env>}/` に置く。`deploy/k8s/<env>/kustomization.yaml` はそれらを *合成するだけ* のエントリポイントで、独自リソースは持たない。これは Go の per-module 構成と一致しており、将来「あるサービスを別リポジトリに切り出す」が機械的に行えるようにするための選択。
- **1サービス = 1名前空間**: `audit` / `auth` / `queue` の 3 つ。Namespace リソースは各サービスの `base/` で定義する（共通の場所には置かない）。
- **Compose と並走可能**: `compose.yml` は廃止せず、Docker Compose 経由の素早い反復ループ用に残す。k8s は追加の経路。
- **kind での再現性最優先**: `:dev` タグの local image を `kind load` する前提。`imagePullPolicy: IfNotPresent` を全コンテナで明示し、レジストリ依存を持たない。
- **Stateless API は LB 可能な構成で配置**: `*-api` Deployment は replicas: 2 + ClusterIP + RollingUpdate(maxUnavailable: 0) + PodDisruptionBudget を備える（後述）。
- **Stateful 依存は dev overlay に閉じる**: Postgres/Redis は `overlays/dev/` でのみ提供。staging/prod overlay では削除し、Secret 経由でマネージドDBに接続する想定（未実装）。

## 2. ディレクトリレイアウト

```
deploy/k8s/
├── istio.md                        # env-agnostic な Istio Ambient 解説
└── dev/                            # `kubectl apply -k deploy/k8s/dev` のエントリポイント
    ├── kustomization.yaml          # modules/<svc>/overlays/dev + otel/k8s/overlays/dev + peerauthentication.yaml
    └── peerauthentication.yaml     # Istio 自前 CR (env 固有 — dev は PERMISSIVE)

# 将来 stg/, prd/ も同じ shape (kustomization.yaml + 自前 CR)

modules/<svc>/deploy/k8s/
├── base/                           # 環境非依存の形（Deployment, Service, Job, NetworkPolicy, PDB）
│   ├── namespace.yaml
│   ├── configmap.yaml
│   ├── api-deployment.yaml         # queue は deployment.yaml という名前
│   ├── api-service.yaml            # queue は service.yaml
│   ├── api-pdb.yaml                # PodDisruptionBudget（minAvailable: 1）
│   ├── (worker-deployment.yaml)    # audit のみ
│   ├── migrate-job.yaml            # audit / auth のみ（queue は DB を持たない）
│   └── networkpolicy.yaml
└── overlays/
    └── dev/                        # in-cluster Postgres / Redis / dev secret + image tag
        ├── kustomization.yaml      # namespace: <svc>, images: { newTag: dev }
        ├── postgres.yaml           # StatefulSet + Service（audit / auth のみ）
        ├── redis.yaml              # Deployment + Service（auth のみ）
        └── secret.yaml             # DB_URL placeholder（compose と等価のダミー値）
```

`shared/` はライブラリ用モジュールなので `deploy/` を持たない。

## 3. 名前空間とサービス境界

| Namespace | サービス | 提供する公開エンドポイント |
|---|---|---|
| `audit` | audit-api（gRPC）、audit-worker（pull型・Service なし）、audit-db、audit-migrate Job | `audit-api.audit.svc.cluster.local:8080` |
| `auth`  | auth-api（HTTP）、auth-db、auth-cache（Redis）、auth-migrate Job | `auth-api.auth.svc.cluster.local:8080` |
| `queue` | queue-api（gRPC） | `queue-api.queue.svc.cluster.local:8080` |

ルートの `deploy/k8s/dev/kustomization.yaml` は **`namespace:` を設定しない**。各 overlay 側が `namespace: <svc>` を宣言しているので、その指定がそのままリソースに伝播する。

### 3.1 サービス間通信

| from | to | protocol / port |
|---|---|---|
| audit-worker (`audit` ns) | queue-api (`queue` ns) | gRPC / 8080 |
| auth-api (`auth` ns) | auth-db (`auth` ns) | Postgres / 5432 |
| auth-api (`auth` ns) | auth-cache (`auth` ns) | Redis / 6379 |
| audit-api (`audit` ns) | audit-db (`audit` ns) | Postgres / 5432 |
| audit-migrate Job (`audit` ns) | audit-db (`audit` ns) | Postgres / 5432 |
| auth-migrate Job (`auth` ns) | auth-db (`auth` ns) | Postgres / 5432 |

クロス名前空間で許可する通信は `audit-worker → queue-api` の1経路だけ。NetworkPolicy 上は両側に対称な allow を置く（送信側に egress、受信側に ingress）。

## 4. ワークロード一覧

| Workload | 種別 | replicas | 公開 Service | 用途 |
|---|---|---|---|---|
| audit-api | Deployment | 2 | ClusterIP `audit-api:8080` | 監査イベントの同期取り込み API |
| audit-worker | Deployment | 1 | （なし） | キューからのプル消費。LB 不要 |
| audit-db | StatefulSet (1 replica) + ClusterIP | 1 | `audit-db:5432` | dev 専用 Postgres、PVC 1Gi |
| audit-migrate | Job (one-shot) | — | — | `atlas migrate apply` |
| auth-api | Deployment | 2 | ClusterIP `auth-api:8080` | OAuth/OIDC API |
| auth-db | StatefulSet (1 replica) + ClusterIP | 1 | `auth-db:5432` | dev 専用 Postgres、PVC 1Gi |
| auth-cache | Deployment + ClusterIP | 1 | `auth-cache:6379` | Redis、emptyDir |
| auth-migrate | Job (one-shot) | — | — | `atlas migrate apply` |
| queue-api | Deployment | 2 | ClusterIP `queue-api:8080` (`appProtocol: grpc`) | gRPC priority queue |

> **stub バイナリ**: `audit-worker` のみ（起動直後に `fmt.Println` して exit するため CrashLoopBackOff）。`audit-api` / `queue-api` は gRPC サーバ（`route/server.go` で `grpc.health.v1` + reflection を register、interceptor チェイン込み）を起動して常駐するので Running になる。`auth-api` も chi で動作する。

## 5. ロードバランシングと可用性

API ワークロード（`*-api`）は **「同一名前空間内で常に複数レプリカが処理可能」** を満たす設計にしている。本番要件相当の構成を base 段階で備えており、overlay は image tag のピン留めしか上書きしない。

### 5.1 ロードバランシングの仕組み

ClusterIP Service が selector でマッチした Pod に対し、kube-proxy が iptables/IPVS でラウンドロビン分散する。これが east-west（pod 間）の「LB」の正体。

north-south（外部入口）は **Istio Gateway API** が担当する。`make k8s-up` で Istio Ambient を導入済みで、`auth` namespace の `Gateway` + `HTTPRoute` (`modules/auth/deploy/k8s/base/gateway.yaml`) が auth-api への入口になる。dev でホストから到達するには `make istio-port-forward` (`localhost:8081` → `svc/auth-gateway-istio:80`)。詳細は `deploy/k8s/istio.md`。

east-west の自動 mTLS は Istio Ambient の **ztunnel** が担当（namespace に `istio.io/dataplane-mode: ambient` ラベルが付く Pod 間で透過的に HBONE 暗号化）。

```
caller pod
   │  Cluster DNS: auth-api.auth.svc.cluster.local
   ▼
[ClusterIP: auth-api]
   │  kube-proxy が iptables/IPVS で分散
   │  + ztunnel が HBONE (mTLS) でラップ
   ├──▶ pod auth-api-xxxxx-aaaaa  (replica 1)
   └──▶ pod auth-api-xxxxx-bbbbb  (replica 2)
```

レプリカ数が単独だと「ロールアウト中・ノード退避中・OOM 発生中」のいずれかでサービスがゼロ容量になる。`replicas: 2` がそれを防ぐ最低ライン。

### 5.2 ロールアウトでゼロダウンタイムを担保する設定

`*-api` Deployment は以下を **base** で備える（overlay 不要）。

```yaml
spec:
  replicas: 2
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1          # 一時的に1台だけ上回って良い
      maxUnavailable: 0    # 既存の利用可能数を一度も減らさない
```

`maxUnavailable: 0` により「常に最低でも `replicas` 台が Ready」を保つ。

### 5.3 ノード分散（Pod Anti-Affinity）

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          topologyKey: kubernetes.io/hostname
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: <svc>
              app.kubernetes.io/component: api
```

`preferred...` を選んでいるのが要点。kind 単一ノードでも `required` 違反にならず schedule できる。マルチノード環境では同一ホストへの相乗りを避けるヒントとして機能する。

### 5.4 自発的中断からの保護（PodDisruptionBudget）

各 `*-api` には `api-pdb.yaml` を base に置いている。

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: <svc>-api
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: <svc>
      app.kubernetes.io/component: api
```

`kubectl drain` やノードアップグレード時、このバジェットが「最低1台は常に利用可能」をスケジューラに強制させる。`minAvailable: 1`（`replicas - 1`）にしているのは、複数台同時 drain を **明示的に許す** ため（同時更新を可能にする）。

### 5.5 graceful shutdown（preStop ドレイン）

```yaml
terminationGracePeriodSeconds: 30
containers:
  - lifecycle:
      preStop:
        exec:
          command: ["sh", "-c", "sleep 5"]
```

Pod 終了時、kubelet が SIGTERM を送るのと同時に kube-proxy は Endpoints からその Pod を外す。両者は **非同期** であり、SIGTERM のほうが先に届くと、まだ転送されてくるリクエストを掴んだまま落ちることがある。`preStop sleep 5` で SIGTERM 着信を 5 秒遅らせ、その間に kube-proxy がトラフィックを流し終えることを期待する。Alpine ベースのイメージのため `sh` と `sleep` が常に利用可能。

サービス側も 30 秒の grace period 内に in-flight リクエストを完了させる責務を持つ。`auth/cmd/api/main.go:99-132` の graceful shutdown と整合する。

### 5.6 LB 構成のサマリ

| 観点 | 採用設定 | 効果 |
|---|---|---|
| 分散方式 | ClusterIP + kube-proxy | 追加 LB を持たない |
| 容量 | `replicas: 2` | 1 台落ちても処理継続 |
| ロールアウト | `maxSurge:1, maxUnavailable:0` | 配信容量が更新中も100% |
| ノード分散 | preferred podAntiAffinity | マルチノードで自然に分散 |
| 自発的中断 | PDB `minAvailable:1` | drain 時に全停止しない |
| 終了処理 | preStop sleep 5 + grace 30s | Endpoints 外しを待つ |

## 6. プローブ

| Workload | readiness | liveness |
|---|---|---|
| auth-api | `httpGet /health` (port `http`) | 同上 |
| audit-api | native `grpc:` (port `8080`) | 同上 |
| queue-api | native `grpc:` (port `8080`) | 同上 |
| audit-worker | （未設定） | 実装後に `/healthz` HTTP を推奨。`exec: pgrep` は避ける |
| audit-db / auth-db | `exec: pg_isready -U <user> -d <db>` | 同上 |
| auth-cache | `exec: redis-cli ping` | 同上 |

auth-api の `/health` は `auth/route/handler.go:22` の chi ハンドラで200を返す前提。audit-api / queue-api は両方とも `route/server.go` で `health.NewServer()` を `grpc.health.v1` として register しており、`health.NewServer()` はデフォルトで空 service を `SERVING` にマークするので、native `grpc:` プローブに `service:` を指定する必要はない。`port:` は数値必須（k8s 1.24+ GA の native gRPC プローブは named port を受け付けない）。

## 7. リソース要求 / 制限

dev では kind ノードを OOM させないよう小さめに設定。staging/prod overlay で上書きする想定（未実装）。

| Workload | requests | limits |
|---|---|---|
| API (`*-api`) | cpu 50m / memory 64Mi | cpu 500m / memory 256Mi |
| Worker (`audit-worker`) | cpu 50m / memory 64Mi | cpu 500m / memory 256Mi |
| Postgres | cpu 100m / memory 128Mi | cpu 1 / memory 512Mi |
| Redis | cpu 50m / memory 32Mi | cpu 200m / memory 128Mi |
| Migrate Job | cpu 50m / memory 64Mi | cpu 500m / memory 256Mi |

## 8. ConfigMap と Secret

| Resource | 場所 | 内容 | 備考 |
|---|---|---|---|
| `auth-api-config` (ConfigMap) | `auth/base/configmap.yaml` | `CACHE_ADDR`, `CACHE_PREFIX` | 機密でない設定 |
| `auth-api-secret` (Secret) | `auth/overlays/dev/secret.yaml` | `DB_URL` | dev placeholder。**本物の秘密ではない** |
| `audit-api-config` (ConfigMap) | `audit/base/configmap.yaml` | `QUEUE_ADDR=queue-api.queue.svc.cluster.local:8080` | audit-api 用 |
| `audit-worker-config` (ConfigMap) | 同上 | `QUEUE_ADDR=...` | audit-worker 用（同値だが宣言上分離） |
| `audit-api-secret` (Secret) | `audit/overlays/dev/secret.yaml` | `DB_URL` | dev placeholder |

`*-api-secret` は migrate Job からも `envFrom: secretRef:` で読み込まれる（`auth/base/migrate-job.yaml` 参照）ので、DB_URL の単一ソースになっている。

staging / prod 化の方針:

- 単純: `secretGenerator` で gitignore された `.env` から生成
- 本格: `ExternalSecret` CRD + 実 Secret Store（AWS SSM / Vault 等）

SOPS や Sealed Secrets は **チーム合意なしに導入しない**（規約 §8）。

## 9. マイグレーション

各サービスの `migrate-job.yaml` は以下を共通形にしている。

```yaml
spec:
  backoffLimit: 10                 # DB ウォームアップ中の失敗を許容
  activeDeadlineSeconds: 300       # 5分でタイムアウト
  ttlSecondsAfterFinished: 600     # 完了後10分で GC
  template:
    spec:
      restartPolicy: Never
      containers:
        - image: migrator
          args: [migrate, apply, --url, $(DB_URL), --dir, file:///go/modules/<svc>/src/infra/database/migrations]
          envFrom:
            - secretRef: { name: <svc>-api-secret }
```

ポイント:

- **`migrate hash` は Job 内で実行しない**: コンテナイメージは read-only。`atlas.sum` はイメージビルド前にホスト側で更新済みである必要がある。Compose の `make <svc>-migrate` は `hash` → `apply` の順で実行するので、通常はこの流れで自動更新される。
- **`$(VAR_NAME)` は k8s ネイティブの env 展開**（シェルではない）。クォートしない。
- **再実行**: kustomize は Helm の hook を持たないので、完了済み Job への再適用は no-op（`ttl` で消えるまで）。明示的に再実行するには `kubectl -n <ns> delete job <svc>-migrate` の後で `make k8s-apply`。

queue は DB を持たないので migrate Job も持たない。

## 10. ストレージ

- audit-db / auth-db は StatefulSet。`volumeClaimTemplates` で 1Gi の PVC を要求。kind の `local-path-provisioner` が動的にバインドする（初回はバインド完了まで数十秒かかる場合がある）。
- auth-cache は emptyDir（明示せず Pod のデフォルト）。Redis のデータは再生成可能なので永続化していない。
- `make k8s-cluster-delete` で kind クラスタを消すと PVC 含め全データが消える。`make k8s-down` は kustomize リソースのみ削除し、PV/PVC は残る。

## 11. ネットワークポリシー

各サービスの `base/networkpolicy.yaml` に最低3本（クロスネームスペース通信があれば追加で1本）。

| Policy | 適用範囲 | 役割 |
|---|---|---|
| `default-deny` | `podSelector: {}` | 全 Pod の Ingress / Egress をデフォルト拒否 |
| `allow-dns-egress` | 全 Pod | `kube-system:53/UDP+TCP` を許可（CoreDNS） |
| `allow-intra-namespace` | 全 Pod | 同一 NS 内の双方向通信を許可 |
| `allow-worker-egress-to-queue` | audit/worker | `queue` ns の queue-api 8080/TCP に egress |
| `allow-api-ingress-from-audit` | queue/api | `audit` ns からの ingress 8080/TCP |

> **kindnet は NetworkPolicy を強制しない**。ルールは記述上正しいが、kind ローカルでは「通信が制限されている錯覚」が起こりうる。実際に強制したい場合は kind に Calico を入れて kindnet を置換する。

## 12. ローカル開発フロー

### 12.1 一発立ち上げ

```bash
make k8s-up        # cluster + build + load + apply + wait + status
make k8s-status    # 名前空間横断で pods/services/jobs を表示
make k8s-down      # kustomization を削除（kind クラスタは残す）
make k8s-cluster-delete  # kind クラスタを消す
```

`make k8s-up` の中身は idempotent な lifecycle:

1. `kind get clusters` → なければ `dev` クラスタを作成
2. 5つの `:dev` イメージをビルド（`audit-api`, `audit-worker`, `auth-api`, `queue-api`, `migrator`）
3. それぞれを `kind load docker-image`
4. `kubectl apply -k deploy/k8s/dev`
5. Postgres × 2 / Redis / migrate Job × 2 が ready/complete になるまで待機
6. `make k8s-status` を表示

期待される定常状態:

- `auth-api`, `audit-api`, `queue-api` Pod: 2台 Running 1/1 Ready
- `auth-db`, `audit-db`: 1台 Running 1/1 Ready
- `auth-cache`: 1台 Running 1/1 Ready
- `auth-migrate`, `audit-migrate`: Completed
- `audit-worker`: **CrashLoopBackOff**（stub バイナリで `fmt.Println` 後 exit。バグではない）

### 12.2 devcontainer 特有の初回セットアップ

#### Docker ソケットの権限

devcontainer はホストの `/var/run/docker.sock` をマウントするが、ソケットの GID はホスト依存でイメージに焼き込まれた group とは一致しない。`.devcontainer/setup.sh`（`postCreateCommand` で起動）が GID を実行時に検出して `docker-host` グループを作り、`vscode` を所属させる。

`make k8s-up` が `permission denied while trying to connect to the docker API` で失敗する場合:

1. グループ所属を確認: `id vscode | grep docker-host`。なければ `sudo .devcontainer/setup.sh` を再実行。
2. `id` で見えていてもシェル自身が古い group set で起動している可能性: `exec newgrp docker-host` か、VS Code ターミナルを開き直し、または "Developer: Reload Window"。
3. 確認: `groups | tr ' ' '\n' | grep docker-host` と `docker info --format '{{.ServerVersion}}'`。

#### kubectl から kind API サーバへの到達

kind はホスト docker daemon 上にノードコンテナを立てるので、デフォルト kubeconfig は **ホスト側** の `127.0.0.1:<published-port>` を指す。devcontainer 内からはこの loopback が別 netns となり、API サーバに到達できず:

```
error validating "deploy/k8s/dev": failed to download openapi:
  Get "https://127.0.0.1:NNNNN/openapi/v2?timeout=32s": dial tcp 127.0.0.1:NNNNN: connect: connection refused
```

`make k8s-cluster`（と `make k8s-up`）は `k8s-kubeconfig` サブターゲットでこれを解消する:

1. このコンテナを `kind` docker network に attach（`docker network connect kind <self>`）
2. `~/.kube/config` を `kind get kubeconfig --name dev --internal` で書き換え（`server: https://dev-control-plane:6443` になる。docker DNS で解決）

両方とも idempotent で、`/.dockerenv` または container cgroup を検出した時のみ走るので、ホスト直での同じ Makefile も安全。

クラスタを `make k8s-cluster-delete` で消した場合は次回 `make k8s-up` で network も kubeconfig も自動的に再生成される。

`~/.kube/config` を手動で壊して API に到達できなくなったら:

```bash
make k8s-kubeconfig
```

で復旧する。

### 12.3 イメージビルドと kind への load

`make k8s-build` は次の `:dev` タグ付きイメージをローカルでビルドする:

| Image | Dockerfile |
|---|---|
| `audit-api` | `.images/audit/api.Dockerfile` |
| `audit-worker` | `.images/audit/worker.Dockerfile` |
| `auth-api` | `.images/auth/api.Dockerfile` |
| `queue-api` | `.images/queue/Dockerfile` |
| `migrator` | `.images/migrator/Dockerfile` |（全 migrate Job 共通） |

`make k8s-load` がそれぞれを `kind load docker-image` する。Pod は `<image>:dev` を `imagePullPolicy: IfNotPresent` で参照するので、kind のローカルキャッシュだけで動く（レジストリ不要）。

イメージ更新後は Pod を再起動する必要がある（k8s は同タグ・別 image-id の差し替えを検知しない）:

```bash
make k8s-load                                       # すべての :dev を再 load
kubectl -n auth rollout restart deployment/auth-api # 該当を再起動
```

## 13. サービスを追加するときの手順

1. `modules/<svc>/deploy/k8s/{base,overlays/dev}/` を既存サービスと同じ形で作る。**auth が最も完全**（cache・db・PDB・probe フル装備）、**queue が最も軽量**（DB なし・PDB のみ）なので雛形として参照。
2. `Namespace` リソースを追加。`name` はモジュール名と同じにする。
3. `deploy/k8s/dev/kustomization.yaml` の `resources:` に新しい overlay パスを追加。
4. 新規イメージを `Makefile` の `K8S_IMAGES` に追加（`make k8s-build` / `make k8s-load` が拾うようにする）。
5. クロスネームスペース通信が発生する場合: **送信側** の base に egress を、**受信側** の base に ingress を、それぞれ書く。`audit/networkpolicy.yaml` の `allow-worker-egress-to-queue` と `queue/networkpolicy.yaml` の `allow-api-ingress-from-audit` がペアの実例。
6. API ワークロードであれば `api-deployment.yaml` + `api-service.yaml` + `api-pdb.yaml` をセットで揃え、kustomization に登録する。

## 14. 既知の制約と将来の方向性

| 状態 | 内容 |
|---|---|
| ✅ ある | dev overlay（kind）、base 側の HA 設定（replicas:2, RollingUpdate, Anti-Affinity, PDB, preStop）、Istio Ambient（mTLS + Gateway API 入口） |
| ❌ まだない | staging / prod overlay。ExternalSecrets、HPA、ServiceMonitor、ClusterIssuer 等の本番配線。audit/queue 用の Gateway（外部公開予定なら追加） |
| ⚠️ stub | audit-worker のバイナリのみ。Pod は CrashLoopBackOff で正常（`audit-api` / `queue-api` は gRPC サーバとして常駐する） |
| ⚠️ kindnet 制限 | NetworkPolicy が dev 環境では強制されない。Calico を入れれば強制される |
| ⚠️ 単一 PV | dev の PVC は kind の local-path-provisioner（ノード固有）。マルチノード kind では Pod が別ノードに schedule された瞬間にデータを失う |
| ⚠️ mTLS posture | dev は PERMISSIVE。STRICT への切替は `deploy/k8s/istio.md` §4 を参照 |

## 15. トラブルシューティング

### `ImagePullBackOff` が `*-api` か `migrator` で出る

イメージが kind に load されていない。`imagePullPolicy: IfNotPresent` だが kind にはレジストリのフォールバックがない。

```bash
make k8s-build && make k8s-load
kubectl rollout restart deployment/<name> -n <ns>
```

### Migrate Job が Backoff で止まる

対応する `*-db` Pod がまだ Ready でない。`backoffLimit: 10` + 初期 backoff で約 2〜3 分は粘るが、それでも失敗するなら:

```bash
kubectl -n <ns> logs job/<svc>-migrate
kubectl -n <ns> describe pod -l app.kubernetes.io/component=db
```

最頻の原因は PVC Pending（kind の `local-path-provisioner` が初回起動時に遅い）。待つか `kubectl get pvc -A` で確認。

### `auth-api` だけ CrashLoopBackOff（DB は Ready）

`auth-api-secret` の `DB_URL` がずれていることがある:

```bash
kubectl -n auth get secret auth-api-secret -o jsonpath='{.data.DB_URL}' | base64 -d
```

期待値は `postgres://auth:auth@auth-db.auth.svc.cluster.local:5432/auth?sslmode=disable`。違っていれば `modules/auth/deploy/k8s/overlays/dev/secret.yaml` を編集して再 apply。

### NetworkPolicy が「効いていない」

kind のデフォルト CNI（kindnet）は NetworkPolicy を強制しない。マニフェストのレンダリング自体は正常で、本番相当の CNI（Calico / Cilium）であれば期待通り動く。dev で実際に試したいなら kind に Calico を入れて kindnet を置換する。

### 既存の Job が再 apply で動かない

`kustomize` には Helm hook がないので、完了済み Job は `ttlSecondsAfterFinished: 600`（10分）で消えるまで「同マニフェスト = no-op」。明示的に再実行:

```bash
kubectl -n <ns> delete job <svc>-migrate
make k8s-apply
```

### 便利な one-liner 集

```bash
# Deployment のログを follow（最新 pod を自動追従）
kubectl -n auth logs deployment/auth-api -f

# in-cluster Postgres に psql
kubectl -n auth exec -it statefulset/auth-db -- psql -U auth -d auth

# Redis CLI
kubectl -n auth exec -it deployment/auth-cache -- redis-cli

# auth-api をホストに port-forward（8080→8080）
kubectl -n auth port-forward svc/auth-api 8080:8080
# 別ターミナル: curl -s http://localhost:8080/health

# 設定変更後に強制再起動
kubectl -n auth rollout restart deployment/auth-api

# flaky な Pod のイベントを見る
kubectl -n auth describe pod -l app.kubernetes.io/name=auth,app.kubernetes.io/component=api
```

## 16. 関連ドキュメント

- `.claude/rules/kubernetes-conventions.md` — 命名・ラベル・Service 種別・migration・Service Mesh ルール（§13）
- `deploy/k8s/istio.md` — Istio Ambient の構成・ホスト到達・STRICT mTLS 移行手順
- `modules/auth/docs/system-design.md` — auth サービスの設計（OAuth/OIDC, MVP）
- `modules/audit/docs/system-design.md` — audit サービスの設計（5W1H 監査基盤）
- `modules/queue/docs/system-design.md` — queue サービスの設計（priority queue）
- `Makefile` — `K8S_IMAGES`、`K8S_NAMESPACES`、`k8s-*` / `istio-*` ターゲットの実装
