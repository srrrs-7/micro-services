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

### First-time setup: Docker socket permission

The devcontainer mounts the host's `/var/run/docker.sock`, but its GID is host-defined and won't match any group baked into the image. `.devcontainer/setup.sh` (run on container creation via `postCreateCommand`) detects the socket's GID, creates a `docker-host` group with that GID, and adds `vscode` to it.

If `make k8s-up` fails with `permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`:

1. Check the user is actually in the group:
   ```bash
   id vscode | grep docker-host
   ```
   If missing, re-run setup: `sudo .devcontainer/setup.sh`.

2. Even when `id` shows the group, an existing shell still uses the group set it was exec'd with. Pick one:
   ```bash
   exec newgrp docker-host        # re-exec the current shell
   ```
   or close and reopen the VS Code terminal, or "Developer: Reload Window".

3. Verify before re-running `make k8s-up`:
   ```bash
   groups | tr ' ' '\n' | grep docker-host
   docker info --format '{{.ServerVersion}}'
   ```

### First-time setup: kubectl reaching the kind API server

`kind` runs its node containers on the host docker daemon (since we share `docker.sock`), so its default kubeconfig points at `127.0.0.1:<published-port>` on the **host**. From inside the devcontainer that loopback is a different network namespace and the API server is unreachable, manifesting as:

```
error validating "deploy/k8s/dev": failed to download openapi:
  Get "https://127.0.0.1:NNNNN/openapi/v2?timeout=32s": dial tcp 127.0.0.1:NNNNN: connect: connection refused
```

`make k8s-cluster` (and therefore `make k8s-up`) handles this via the `k8s-kubeconfig` sub-target:

1. Attaches this container to the `kind` docker network (`docker network connect kind <self>`).
2. Rewrites `~/.kube/config` from `kind get kubeconfig --name dev --internal`, which sets `server: https://dev-control-plane:6443` (resolved via docker DNS on the `kind` network).

Both steps are idempotent and only run when `/.dockerenv` (or a container cgroup) is detected, so the same Makefile is safe outside a container.

If you delete the cluster with `make k8s-cluster-delete`, the network is recreated next time and the kubeconfig is regenerated; nothing manual is required.

If you've manually edited `~/.kube/config` and `make k8s-up` is now refusing to reach the API server, run:

```bash
make k8s-kubeconfig
```

to restore the in-container-friendly config.

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
