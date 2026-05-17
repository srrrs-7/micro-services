# Kubernetes Architecture

```mermaid
graph TB
    subgraph "istio-system"
        PA["PeerAuthentication<br/>(mTLS: PERMISSIVE)"]
        ISTIO["Istio Control Plane<br/>istiod / ztunnel / istio-cni"]
    end

    subgraph "auth namespace"
        GW["Gateway (istio)<br/>:80 HTTP"]
        HR["HTTPRoute /<br/>→ auth-api:8080"]
        AUTH_API["auth-api Deployment<br/>replicas: 2, HTTP :8080"]
        AUTH_DB["auth-db StatefulSet<br/>PostgreSQL :5432, 1Gi PVC"]
        AUTH_CACHE["auth-cache Deployment<br/>Redis :6379"]
        AUTH_MIG["auth-migrate Job<br/>atlas migrate apply"]
    end

    subgraph "audit namespace"
        AUDIT_API["audit-api Deployment<br/>replicas: 2, gRPC :8080"]
        AUDIT_WORKER["audit-worker Deployment<br/>replicas: 1"]
        AUDIT_DB["audit-db StatefulSet<br/>PostgreSQL :5432, 1Gi PVC"]
        AUDIT_MIG["audit-migrate Job<br/>atlas migrate apply"]
    end

    subgraph "queue namespace"
        QUEUE_API["queue-api Deployment<br/>replicas: 2, gRPC :8080"]
    end

    subgraph "observability namespace"
        COLLECTOR["otel-collector<br/>OTLP gRPC:4317 / HTTP:4318<br/>Prom export:8889"]
        PROM["prometheus :9090<br/>rules + 24h retention"]
        TEMPO["tempo :3200<br/>traces backend"]
        LOKI["loki :3100<br/>logs backend (idle)"]
        GRAFANA["grafana :3000<br/>anonymous Admin"]
    end

    CLIENT["External Client"] -->|port-forward :8081| GW
    GW --> HR
    HR --> AUTH_API

    AUTH_API --> AUTH_DB
    AUTH_API --> AUTH_CACHE
    AUTH_MIG --> AUTH_DB

    AUDIT_API --> AUDIT_DB
    AUDIT_MIG --> AUDIT_DB
    AUDIT_WORKER -->|gRPC consume| QUEUE_API
    AUDIT_WORKER --> AUDIT_DB

    AUDIT_API -.->|gRPC call| QUEUE_API

    AUTH_API -.->|OTLP| COLLECTOR
    AUDIT_API -.->|OTLP| COLLECTOR
    AUDIT_WORKER -.->|OTLP| COLLECTOR
    QUEUE_API -.->|OTLP| COLLECTOR

    COLLECTOR -->|scrape| PROM
    COLLECTOR -->|traces| TEMPO
    COLLECTOR -->|logs| LOKI

    GRAFANA --> PROM
    GRAFANA --> TEMPO
    GRAFANA --> LOKI

    ISTIO -.->|HBONE mTLS| AUTH_API
    ISTIO -.->|HBONE mTLS| AUDIT_API
    ISTIO -.->|HBONE mTLS| AUDIT_WORKER
    ISTIO -.->|HBONE mTLS| QUEUE_API
    ISTIO -.->|HBONE mTLS| COLLECTOR

    classDef svc fill:#e1f5fe,stroke:#0288d1,stroke-width:2px
    classDef infra fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px
    classDef obs fill:#e8f5e9,stroke:#388e3c,stroke-width:2px
    classDef mesh fill:#fff3e0,stroke:#f57c00,stroke-width:2px
    classDef external fill:#fce4ec,stroke:#c2185b,stroke-width:2px

    class AUTH_API,AUDIT_API,AUDIT_WORKER,QUEUE_API,AUTH_MIG,AUDIT_MIG svc
    class AUTH_DB,AUDIT_DB,AUTH_CACHE infra
    class COLLECTOR,PROM,TEMPO,LOKI,GRAFANA obs
    class ISTIO,PA,GW,HR mesh
    class CLIENT external
```

## Namespace 構成

| Namespace | リソース | 備考 |
|---|---|---|
| **istio-system** | istiod / ztunnel / istio-cni-node / PeerAuthentication | Istio Ambient mesh。devは `PERMISSIVE` |
| **auth** | auth-api (×2), auth-db (StatefulSet), auth-cache (Redis), auth-migrate (Job), Gateway + HTTPRoute | OAuth 2.0/OIDC AS。HTTP APIのみ。Gateway APIで外部公開 |
| **audit** | audit-api (×2, gRPC), audit-worker (×1), audit-db (StatefulSet), audit-migrate (Job) | 5W1H監査証跡。gRPC。workerはqueue-apiをconsume |
| **queue** | queue-api (×2, gRPC) | 優先度付きメッセージキュー。gRPC |
| **observability** | otel-collector, prometheus, tempo, loki, grafana | OTel Phase 1-2完了。LokiはPhase 3保留中でidle |

## 通信パターン

- **南北**: Gateway API (Istio) → auth-api のみ外部公開。`make istio-port-forward` で `localhost:8081` からアクセス
- **東西**: Istio Ambient HBONE (mTLS) で全サービス間通信を暗号化。各namespaceに `istio.io/dataplane-mode: ambient` ラベル
- **観測性**: 各バイナリ → OTel Collector (`otel-collector.observability:4317`) → Prometheus/Tempo/Loki → Grafana

## デプロイ

```
make k8s-up
```

kindクラスタ作成 → イメージbuild/load → Istio install → kustomize apply → 全Pod待機 → 状態表示
