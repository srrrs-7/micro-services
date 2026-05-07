# Kubernetes Conventions

Project-specific rules for the local-kind / future-staging-prod Kubernetes setup. The full directory layout lives in `deploy/k8s/README.md`; this file captures the *binding* conventions agents must follow when authoring or modifying manifests.

## 1. Manifest location

Per-service manifests live under the service module:

```
modules/<svc>/deploy/k8s/
├── base/                # environment-agnostic shape
└── overlays/<env>/      # env-specific (dev only today; staging/prod follow)
```

Cross-service composition lives at the repo root:

```
deploy/k8s/<env>/kustomization.yaml   # references each service's overlays/<env>
```

Do NOT introduce a centralized `deploy/k8s/<svc>/` directory — service-owned manifests is the deliberate choice (matches the per-module Go layout and keeps a future "extract a service to its own repo" trivial).

`shared` is a library module and has no `deploy/`.

## 2. Namespaces

One namespace per service, named after the module:

| Service | Namespace |
|---|---|
| audit | `audit` |
| auth | `auth` |
| queue | `queue` |

The `Namespace` resource is created in **that service's `base/`**, not in a shared place. Each `kustomization.yaml` (base and overlay) sets `namespace: <svc>` at the top level so all resources land there.

The root `deploy/k8s/<env>/kustomization.yaml` does NOT set a namespace — it composes per-service overlays as-is.

## 3. Image naming

Images are named after the binary, with no project prefix:

| Image | Dockerfile |
|---|---|
| `audit-api` | `.images/audit/api.Dockerfile` |
| `audit-worker` | `.images/audit/worker.Dockerfile` |
| `auth-api` | `.images/auth/api.Dockerfile` |
| `queue-api` | `.images/queue/Dockerfile` |
| `migrator` | `.images/migrator/Dockerfile` |

`base/` deployments reference the image without a tag (e.g. `image: auth-api`); the **overlay** sets the tag via `images:`:

```yaml
# modules/auth/deploy/k8s/overlays/dev/kustomization.yaml
images:
  - name: auth-api
    newTag: dev
  - name: migrator
    newTag: dev
```

`imagePullPolicy: IfNotPresent` is set explicitly on every container — kind loads images locally and there is no registry to pull from.

When adding a new image: also append it to `K8S_IMAGES` in the root `Makefile` so `make k8s-build` and `make k8s-load` pick it up.

## 4. Labels

Each resource carries these labels (set manually, not via `commonLabels`/`labels` — see "Why not commonLabels" below):

```yaml
metadata:
  labels:
    app.kubernetes.io/name: <service>          # audit / auth / queue
    app.kubernetes.io/component: <role>        # api / worker / db / cache / migrate
    app.kubernetes.io/managed-by: kustomize
```

**Do NOT add `app.kubernetes.io/part-of`** — there is no project-wide identifier (the repo has no formal name and we deliberately avoided inventing one).

Pod-template labels and Deployment selectors must include both `name` and `component`. `component` is what distinguishes `audit-api` pods from `audit-worker` pods within the `audit` namespace.

### Why not commonLabels

`kustomize`'s `commonLabels` mutates `selector.matchLabels` in workloads. With multiple Deployments in the same namespace sharing a `name` label, this would produce overlapping selectors and break pod ownership. Manual labels are verbose but predictable. Don't try to "clean this up" with `commonLabels` or the newer `labels` field unless you fully migrate every workload's selector at the same time.

## 5. Service exposure

| Service | Service type | Port name | Notes |
|---|---|---|---|
| `audit-api` | ClusterIP | `http:8080` | Future Ingress lives in a `components/ingress/` (not built yet). |
| `auth-api` | ClusterIP | `http:8080` | Same. |
| `queue-api` | ClusterIP | `grpc:8080` | `appProtocol: grpc` set on the port for future Gateway-API consumers. |
| `audit-worker` | NO Service | — | Workers are pull-based; nothing addresses them. |
| `audit-db` / `auth-db` | ClusterIP (StatefulSet headless not used) | `postgres:5432` | Dev only — overlays/dev provides these. Prod overlay (when added) MUST omit them and point env at managed DB via Secret. |
| `auth-cache` | ClusterIP | `redis:6379` | Dev only. |

In-cluster Postgres uses a StatefulSet + `volumeClaimTemplates` (kind's `local-path-provisioner` provides storage). Redis uses a plain Deployment + emptyDir — Redis state is regenerable.

## 6. Probes

| Workload | Probe |
|---|---|
| `auth-api` | `httpGet /health` on port `http` (the `/health` route is mandated by the chi handler — see `auth/route/handler.go:22`). |
| `audit-api` | `httpGet /health` (will exist when implemented; stub crashloops are expected for now). |
| `queue-api` | `tcpSocket grpc:8080` — TCP probe avoids requiring a gRPC health-check implementation. Switch to `grpc:` probe (k8s 1.24+) when a gRPC health server is added. |
| `audit-worker` | None today. When implemented, prefer a tiny HTTP `/healthz` over `exec: [pgrep, worker]` for liveness. |
| `*-db` (Postgres) | `exec: [pg_isready, -U, <user>, -d, <db>]` for both readiness and liveness. |
| `auth-cache` (Redis) | `exec: [redis-cli, ping]` for both. |

## 7. Migrations

Each service that has migrations gets a `migrate-job.yaml` Job in its base. The Job:

- Uses image `migrator:dev`
- Runs `atlas migrate apply --url $(DB_URL) --dir file:///go/modules/<svc>/src/infra/database/migrations`
- Reads `DB_URL` via `envFrom: secretRef: <svc>-api-secret`
- Has `backoffLimit: 10` and `activeDeadlineSeconds: 300` to ride out the DB warm-up window
- Has `ttlSecondsAfterFinished: 600` for auto-cleanup

Note: `migrate hash` is **not** run in the Job — the migrator image is read-only at runtime. Ensure `atlas.sum` is up-to-date in the source tree before building the migrator image. The compose-based `make <svc>-migrate` target runs `hash` then `apply`, so this is normally already the case.

`kustomize` has no Helm-style hooks. Re-applying the same Job manifest is a no-op for completed Jobs; if you need a fresh run, `kubectl delete job <svc>-migrate` first.

## 8. Configuration & secrets

- **ConfigMap** holds non-sensitive env (e.g. `CACHE_ADDR`, `CACHE_PREFIX`, `QUEUE_ADDR`). Defined in `base/configmap.yaml`.
- **Secret** holds sensitive env (`DB_URL`). Defined as a placeholder in `overlays/dev/secret.yaml` with values equivalent to `compose.yml` — these are NOT real secrets.
- For staging/prod, the placeholder secret is replaced by either:
  - `secretGenerator` from a gitignored `.env` (simple cases), or
  - An `ExternalSecret` CRD pointing at a real Secret Store (AWS SSM, Vault, etc).

Do NOT introduce a new Secret pattern (e.g. SOPS, Sealed Secrets) without team agreement.

The k8s `$(VAR_NAME)` substitution syntax in container `args` is used for `DB_URL` in migrate Jobs. This is k8s-native env expansion, not shell — don't quote it.

## 9. NetworkPolicy

Each service's `base/networkpolicy.yaml` ships:

1. `default-deny` (selects all pods, `policyTypes: [Ingress, Egress]`, no allow rules)
2. `allow-dns-egress` (egress to `kube-system` on UDP/TCP 53)
3. `allow-intra-namespace` (ingress + egress to/from same-ns pods)
4. Service-specific cross-namespace allows (e.g. `audit` allows worker → `queue.queue:8080`; `queue` allows ingress from `audit`).

**Caveat:** kind's default CNI (kindnet) does NOT enforce NetworkPolicy. The policies are written for correctness in clusters with a CNI that supports them (Calico, Cilium). Do not rely on policy enforcement during local development. To validate enforcement, install Calico into the kind cluster.

## 10. Resource requests/limits

Dev defaults (modest, so the kind cluster doesn't OOM):

| Workload | requests | limits |
|---|---|---|
| API (`*-api`) | `cpu: 50m, memory: 64Mi` | `cpu: 500m, memory: 256Mi` |
| Worker (`audit-worker`) | `cpu: 50m, memory: 64Mi` | `cpu: 500m, memory: 256Mi` |
| Postgres | `cpu: 100m, memory: 128Mi` | `cpu: 1, memory: 512Mi` |
| Redis | `cpu: 50m, memory: 32Mi` | `cpu: 200m, memory: 128Mi` |
| Migrate Job | `cpu: 50m, memory: 64Mi` | `cpu: 500m, memory: 256Mi` |

Tune in overlays for staging/prod — don't bake prod numbers into `base/`.

## 11. Adding a new service

1. Create `modules/<svc>/deploy/k8s/{base,overlays/dev}/` mirroring an existing service. `auth/` is the most complete reference; `queue/` is the lightest.
2. Add a `Namespace` resource named after the module.
3. Add the new overlay path to `deploy/k8s/dev/kustomization.yaml`'s `resources:`.
4. Add image(s) to the `K8S_IMAGES` Makefile variable.
5. If the service produces cross-namespace traffic, write the egress rule in *its own* base, and add the matching ingress rule to the **target** namespace's base.

## 12. Anti-patterns

- **Editing `compose.yml` instead of overlay** for things meant to be k8s-only. The two stacks are independent; compose is for the Docker-Compose flow, k8s for kind/staging/prod.
- **Hand-editing files under `modules/<svc>/deploy/k8s/overlays/dev/secret.yaml` thinking it's gitignored**. It isn't (the values are equivalent to compose; the file is committed). Treat any real secret as belonging to a different mechanism (see §8).
- **Adding a `part-of` label**. There is no project name; we agreed not to invent one.
- **Trying to use `commonLabels` for `app.kubernetes.io/name`**. See §4.
- **Running `migrate hash` in the Job**. The Atlas image is read-only at runtime; hash must be done before image build.
- **Centralizing manifests under `deploy/k8s/<svc>/`**. Per-service ownership under `modules/<svc>/deploy/` is the rule (§1).
