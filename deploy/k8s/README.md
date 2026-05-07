# Kubernetes manifests

Per-service manifests live under `modules/<svc>/deploy/k8s/`. This directory holds the cross-service overlays that compose them into a single deployable system.

## Layout

```
deploy/k8s/
└── dev/                            # entry point: `kubectl apply -k deploy/k8s/dev`
    └── kustomization.yaml          # references modules/<svc>/deploy/k8s/overlays/dev

modules/<svc>/deploy/k8s/
├── base/                           # environment-agnostic shape (Deployment, Service, Job, ...)
└── overlays/
    └── dev/                        # in-cluster Postgres/Redis (StatefulSet/Deployment), image tag pin, dev secrets
```

## Namespaces

One per service: `audit`, `auth`, `queue`. Each is created by its own service base.

| from | to | DNS |
|---|---|---|
| audit-worker (`audit` ns) | queue-api (`queue` ns) | `queue-api.queue.svc.cluster.local:8080` |
| auth-api (`auth` ns) | auth-db (`auth` ns) | `auth-db.auth.svc.cluster.local:5432` |
| auth-api (`auth` ns) | auth-cache (`auth` ns) | `auth-cache.auth.svc.cluster.local:6379` |
| audit-api (`audit` ns) | audit-db (`audit` ns) | `audit-db.audit.svc.cluster.local:5432` |

## Local dev quickstart (kind)

```bash
make k8s-up        # cluster + build + load + apply + wait + status
make k8s-status    # show pods/services/jobs across namespaces
make k8s-down      # delete the kustomization (cluster stays up)
make k8s-cluster-delete  # delete the kind cluster entirely
```

Prerequisites in the devcontainer (added via `.devcontainer/devcontainer.json`):

- `docker` (outside-of-docker socket share)
- `kubectl`
- `kind`

## Image build & load

`make k8s-build` builds these `:dev` images locally:

- `audit-api` (`.images/audit/api.Dockerfile`)
- `audit-worker` (`.images/audit/worker.Dockerfile`)
- `auth-api` (`.images/auth/api.Dockerfile`)
- `queue-api` (`.images/queue/Dockerfile`)
- `migrator` (`.images/migrator/Dockerfile`) — used by all `*-migrate` Jobs

`make k8s-load` then runs `kind load docker-image` for each. Pods reference these by `<image>:dev` with `imagePullPolicy: IfNotPresent`, so kind's local cache is sufficient — no registry needed.

## Migrations

Each service's `migrate-job.yaml` runs `atlas migrate apply` against its DB. The `migrator` image bakes in the entire `modules/` tree (per `.images/migrator/Dockerfile`), so the Job's `--dir file:///go/modules/<svc>/src/infra/database/migrations` resolves inside the container.

Note: `migrate hash` is **not** run in the Job (the image is read-only). Ensure `atlas.sum` is up-to-date in the source tree before building the migrator image — the existing Makefile does `migrate hash` in dev, so this is already the case if you migrate via compose first.

## Adding a new service

1. Create `modules/<svc>/deploy/k8s/{base,overlays/dev}/` with the same shape as an existing service (auth is the most complete example).
2. Add a Namespace per the existing pattern, name it after the module.
3. Add the new overlay path to `deploy/k8s/dev/kustomization.yaml`'s `resources:`.
4. Add the new image(s) to `Makefile`'s `K8S_IMAGES` so `make k8s-build` and `make k8s-load` pick them up.

## Caveats

- **kind does not enforce NetworkPolicy by default.** kindnet (the default CNI) ignores NetworkPolicy. The policies are written for correctness in clusters with a CNI that supports them (Calico, Cilium). To enforce locally, install Calico into the kind cluster.
- **Stub services crashloop.** `audit-api`, `audit-worker`, and `queue-api` currently have stub `main.go` that exits immediately. They will appear as `CrashLoopBackOff` in `make k8s-status` until the binaries are implemented. `auth-api` is the only working service today.
- **Secrets are dev-only placeholders.** Values in `modules/<svc>/deploy/k8s/overlays/dev/secret.yaml` are equivalent to `compose.yml` and are NOT real secrets. For staging/prod, replace with secretGenerator from a gitignored file or with ExternalSecrets.
