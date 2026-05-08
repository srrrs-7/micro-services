# Istio (Ambient mode)

このリポジトリの k8s デプロイに **Istio Ambient mode** を導入し、east-west の自動 mTLS と north-south の入口 (Gateway API) を 1 スタックで担う。Compose 経路は対象外 (Istio は k8s-only)。

## 1. 構成要素

| コンポーネント | 種別 | 役割 | 備考 |
|---|---|---|---|
| `istiod` | Deployment (`istio-system`) | 制御プレーン (xDS / 証明書発行) | `istioctl install` で投入 |
| `istio-cni-node` | DaemonSet (`istio-system`) | ノードに iptables redirect を仕込む CNI plugin | kindnet と共存 |
| `ztunnel` | DaemonSet (`istio-system`) | east-west L4 mTLS データプレーン (HBONE) | Rust 製、~30MB / node |
| `auth-gateway` (auto-provisioned) | Deployment + ClusterIP Svc (`auth`) | north-south 入口 (Envoy) | Gateway API リソースから自動生成 |
| `PeerAuthentication default` | CR (`istio-system`) | mTLS posture (`PERMISSIVE` / `STRICT`) | dev は PERMISSIVE |

waypoint proxy (L7 east-west) は今は導入しない — 1 経路 (audit-worker → queue-api) では過剰、必要になったら namespace 単位で追加。

## 2. ディレクトリレイアウト

```
deploy/k8s/
├── istio.md                        # このファイル (env-agnostic Istio 解説)
├── dev/
│   ├── kustomization.yaml          # peerauthentication.yaml + per-service overlays + otel
│   └── peerauthentication.yaml     # dev mTLS posture (PERMISSIVE)
└── (将来 stg/, prd/ も同様にそれぞれ kustomization.yaml + 自前 CR)

modules/auth/deploy/k8s/
└── base/gateway.yaml               # Gateway + HTTPRoute (Gateway API)
```

### なぜ `<component>/k8s/{base,overlays/<env>}/` パターンを使わないか

per-service module や `otel/` は **env-agnostic な base manifest** (Deployment / Service / ConfigMap 等) を持つので `base + overlays/<env>` の構造が必要。Istio は逆で、env-agnostic な部分は **`istioctl install` 側にあって kustomize 管轄外**。kustomize に置くのは env 固有 CR (`PeerAuthentication`、将来の `AuthorizationPolicy` 等) だけなので、env 固有の CR を直接 `deploy/k8s/<env>/` に置く方が二重ネストにならず素直。

`otel/` が repo root に居るのは k8s に加えて Compose / Collector 設定 / Grafana ダッシュボードなど **k8s 以外の成果物** を抱えるためであって、Istio とは性質が違う。配置を揃える必要は無い。

## 3. ホストから auth-api へ到達する仕組み

dev では `kubectl port-forward` を canonical な経路にする。NodePort + kind extraPortMappings の組み合わせは kindnet と Istio CNI plugin の iptables 相互作用でパケットが pod に届かないケースがあり、複数の kind 環境で再現したため dev では**使わない**。

```
host:8081  ←──── kubectl port-forward ────  svc/auth-gateway-istio:80 (ClusterIP)
                                                           │
                                                           ▼
                                              auth-gateway pod (Envoy)
                                                           │
                                                           │ HTTPRoute (PathPrefix /)
                                                           │ + ztunnel HBONE (mTLS, ambient)
                                                           ▼
                                              auth-api pod
```

確認:

```bash
make k8s-up
make istio-port-forward          # 別シェルで起動しっぱなしにしておく
curl -i http://localhost:8081/health   # auth/route/handler.go の /health を経由
# → 200 OK
# → server: istio-envoy
# → x-envoy-upstream-service-time: <ms>
```

prod / staging では Istio Gateway の auto-provisioned `auth-gateway-istio` Service を `LoadBalancer` 型のまま (kind では `<pending>`、本物の k8s ではクラウド LB が IP を割当) external ingress 経路に直結する。dev のみ port-forward で代替。

### NodePort + extraPortMappings を採用しなかった理由

実測値:
- `iptables -t nat KUBE-NODEPORTS` のカウンタは increment する (DNAT は走る)
- 他方 gateway pod の veth では SYN 後の packet が観測されない
- conntrack に対応 entry が作られない
- ホスト/devcontainer から `host:8081` / `kind-node:30080` 共に応答無し
- kind ノード hostNetwork からの `127.0.0.1:30080` は問題なく 200 OK
- pod-to-pod の ClusterIP 経由も 200 OK

つまり「外部 source IP からの NodePort 入口」だけが落ちる症状。Istio CNI plugin と kindnet/iptables の相互作用が疑わしいが、原因深掘りより `kubectl port-forward` への倒し込みのほうが投資対効果が高い。本番では Gateway API の auto-provisioned Service が正しい入口になる。

## 4. mTLS の posture と STRICT 移行

`overlays/dev/peerauthentication.yaml` で `mtls.mode` を切り替える。

| mode | 効果 | 適用シーン |
|---|---|---|
| `PERMISSIVE` (dev 既定) | mTLS と plaintext を共存可能 | 段階移行中、検証中 |
| `STRICT` (prod 想定) | ambient 外からの plaintext を拒否 | 全 caller を ambient 検証完了後 |

### STRICT 移行手順

1. `make k8s-up` で PERMISSIVE 状態で起動
2. **検証**: 全ての通信経路が ambient + HBONE になっていることを確認
   ```bash
   # ztunnel ログで HBONE handshake を確認
   kubectl -n istio-system logs ds/ztunnel | grep -i hbone

   # 各 Pod が ambient 配下にいるか
   istioctl ztunnel-config workload
   ```
3. ファイル編集: `mtls.mode: PERMISSIVE` → `mtls.mode: STRICT`
4. 再 apply: `kubectl apply -k deploy/k8s/dev` (env-specific `peerauthentication.yaml` がそこに居る)
5. **回帰チェック**: NodePort 経由のホスト到達が継続するか確認 (gateway pod が ambient 経由で auth-api に到達できているか)
   ```bash
   curl -i http://localhost:8081/health
   ```

ロールバック: PERMISSIVE に戻して再 apply。

## 5. Telemetry

Phase 1 で投入する scrape は **istiod のみ**。

- `otel/prometheus/prometheus.yml` の `job_name: istiod` が `istiod.istio-system.svc.cluster.local:15014` を 15s 間隔で scrape する
- Compose 経路 (`make obs-up`) では DNS 解決失敗で down target 扱いになる — 想定挙動

ztunnel の per-pod metrics は将来追加予定 (kubernetes_sd を有効化する RBAC 整備が前提)。

`utilotel` は変更しない — L7 (HTTP/gRPC RED + business 属性付き span) はアプリ側、L4 (TCP bytes / mTLS handshake) は Istio 側、の役割分担。

## 6. NetworkPolicy との関係

NetworkPolicy は **そのまま維持**。Istio mTLS と直交した defense in depth として機能する:

- NetworkPolicy: L3/4 (どのポッドからどのポッドへの TCP/UDP を許可するか)
- Istio mTLS: L4 暗号化 + 将来の L7 AuthorizationPolicy

kindnet は NetworkPolicy を強制しないので dev では事実上アドバイザリ。Calico を入れた場合に効く。

## 7. アンインストール

```bash
make istio-down       # istioctl uninstall --purge -y + kubectl delete ns istio-system
```

namespace の `istio.io/dataplane-mode: ambient` ラベルは残るが、ztunnel が居ないので無効化される (mTLS なし、plaintext)。完全に元に戻したい場合は各 namespace.yaml からラベルを外す。

## 8. 既知の制約

| 状況 | 内容 |
|---|---|
| ⚠️ dev kindnet で NetworkPolicy 非強制 | Istio AuthorizationPolicy も併せて入れたいなら waypoint proxy 必須 |
| ⚠️ Compose 経路は対象外 | Istio は k8s-only。Compose では plaintext のまま |
| ⚠️ ztunnel metrics 未統合 | Phase 4 では istiod のみ。ztunnel は後続 |
| ⚠️ dev は PERMISSIVE | 本番想定の STRICT は §4 の検証手順を踏んでから |

## 9. トラブルシュート

### `curl http://localhost:8081/health` → `Empty reply from server` / 反応なし

最頻原因: **`make istio-port-forward` が実行されていない**。`make k8s-up` だけでは host:8081 は何にもバインドされない (§3 参照、dev は port-forward 経由)。

復旧:

```bash
make istio-port-forward            # 別シェルで起動しっぱなし
curl -i http://localhost:8081/health
```

それでも応答が無い場合は、auto-provisioned gateway pod の状態を確認:

```bash
kubectl get crd gateways.gateway.networking.k8s.io   # NotFound なら CRD 不在 → make istio-down && make istio-up
kubectl get gatewayclass istio                        # NotFound なら istiod が Gateway API モード未起動
kubectl -n auth get gateway,httproute                 # Status.Conditions が Accepted: false なら Gateway 未承認
kubectl -n auth get pods -l gateway.networking.k8s.io/gateway-name=auth-gateway   # Ready 1/1 を確認
kubectl -n auth logs -l gateway.networking.k8s.io/gateway-name=auth-gateway --tail=20
```

### gateway pod は Running なのに 503 / 404

HTTPRoute の `backendRefs.name` / `port` が auth-api Service と合っていない可能性。

```bash
kubectl -n auth get svc auth-api -o jsonpath='{.spec.ports}'
kubectl -n auth get httproute auth-route -o yaml | grep -A 5 backendRefs
```

### Envoy のアクセスログを見たい (デフォルトでは無効)

`Telemetry` CR で gateway 単位に有効化:

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: gateway-access-logs
  namespace: auth
spec:
  selector:
    matchLabels:
      gateway.networking.k8s.io/gateway-name: auth-gateway
  accessLogging:
    - providers:
        - name: envoy
```

apply 後は `kubectl -n auth logs -l gateway.networking.k8s.io/gateway-name=auth-gateway` で 1 行 / リクエスト出る。

### gateway pod が Running だが応答が無い (ambient 関連)

PeerAuthentication が STRICT だと ambient 外からの plaintext 入口が拒否される。dev 既定は PERMISSIVE。

```bash
kubectl -n istio-system get peerauthentication default -o jsonpath='{.spec.mtls.mode}{"\n"}'
```

## 10. 関連ドキュメント

- 上位の Istio 採用判断と段階導入計画は会話履歴を参照
- `.claude/rules/kubernetes-conventions.md` §13 — Service Mesh の binding rules
- `deploy/k8s/README.md` §5.1 — LB / 入口の上位設計
- `otel/README.md` — 観測スタックとの境界
