---
name: run-k8s
description: Use when the user wants to run, restart, debug, or inspect the local Kubernetes (kind) deployment of the audit/auth/queue services. Handles cluster creation, image build+load, kustomize apply, and the common debugging paths when pods aren't healthy.
---

# Run the local Kubernetes (kind) stack

The repo has a kind-based local k8s setup parallel to (not replacing) the Docker Compose flow. Manifests live per-service under `modules/<svc>/deploy/k8s/{base,overlays/dev}/`, composed by `deploy/k8s/dev/kustomization.yaml`. Three namespaces (`audit`, `auth`, `queue`) — one per module.

## When to use this vs `run-service` (compose)

| Use this skill when… | Use `run-service` (compose) when… |
|---|---|
| Verifying k8s manifest changes | Iterating on Go code with fast rebuild |
| Reproducing a deploy-time issue (Job ordering, NetworkPolicy, probes) | Need a single service running with full host port exposure |
| Validating cross-namespace DNS or service discovery | Quick smoke test, port-forward to host's `5432`/`5433`/`6379` |

The two stacks are fully independent — running both at once is fine if host resources allow.

## Bring it all up

```bash
make k8s-up
```

This is the full lifecycle and is idempotent:

1. `kind get clusters` → creates cluster `dev` if missing
2. `docker build` for all five `:dev` images (`audit-api`, `audit-worker`, `auth-api`, `queue-api`, `migrator`)
3. `kind load docker-image` each image into the cluster
4. `kubectl apply -k deploy/k8s/dev`
5. Waits for Postgres × 2, Redis, and both migrate Jobs
6. Calls `make k8s-status` to print the result

Expected end state:

- `auth-api` pod: Running 1/1 Ready
- `auth-db`, `audit-db`: Running 1/1 Ready
- `auth-cache`: Running 1/1 Ready
- `auth-migrate`, `audit-migrate`: Completed
- `audit-api`, `audit-worker`, `queue-api`: **CrashLoopBackOff** (these are stub binaries that `fmt.Println` and exit; not a bug)

## Inspect status

```bash
make k8s-status
```

Outputs pods, services, and jobs grouped by namespace. Use this as the first step when triaging — it's a wide enough view to spot Pending pods, ImagePullBackOff, missing secrets, etc.

## Tear down

```bash
make k8s-down                # delete the kustomization, keep the cluster
make k8s-cluster-delete      # delete the kind cluster entirely (data is lost)
```

`k8s-down` is the right choice when iterating — it leaves the kind node alive so the next `make k8s-up` is fast. `k8s-cluster-delete` is for resetting from scratch (e.g. after editing `Dockerfile`s).

## Common failures

### `ImagePullBackOff` on a `*-api` or `migrator` pod
The image wasn't loaded into the kind cluster. `imagePullPolicy: IfNotPresent` is set, but kind has no registry to fall back to. Fix:
```bash
make k8s-build && make k8s-load
```
Then `kubectl rollout restart deployment/<name> -n <ns>`.

### Migrate Job stuck in `Backoff`
The corresponding `*-db` Pod isn't ready yet. `backoffLimit: 10` plus initial backoff buys ~2–3 minutes; if it's still failing, inspect:
```bash
kubectl -n <ns> logs job/<svc>-migrate
kubectl -n <ns> describe pod -l app.kubernetes.io/component=db
```
Most common cause: PVC pending (kind's `local-path-provisioner` is slow to bind on first run). Wait or `kubectl get pvc -A`.

### `auth-api` is in CrashLoopBackOff but DB is Ready
The `DB_URL` value in `auth-api-secret` is probably stale. Inspect:
```bash
kubectl -n auth get secret auth-api-secret -o jsonpath='{.data.DB_URL}' | base64 -d
```
The expected value is `postgres://auth:auth@auth-db.auth.svc.cluster.local:5432/auth?sslmode=disable`. If it differs, edit `modules/auth/deploy/k8s/overlays/dev/secret.yaml` and re-apply.

### NetworkPolicy seems "wrong" (everything still works)
kind's default CNI (kindnet) doesn't enforce NetworkPolicy. The policies render correctly but have no effect locally. This is documented in `.claude/rules/kubernetes-conventions.md` §9. To actually enforce in dev, install Calico into the kind cluster.

### Old image cached after rebuild
`kind load docker-image` is incremental but uses the local docker image ID. After `docker build -t auth-api:dev ...`, a fresh load is required:
```bash
make k8s-load            # reloads all five images
```
The pod also needs to be restarted (k8s won't notice the underlying image swap):
```bash
kubectl -n auth rollout restart deployment/auth-api
```

### Job already exists when re-applying
`kustomize` has no Helm hooks; a completed Job stays around for `ttlSecondsAfterFinished: 600` (10 min) and re-applying is a no-op. To re-run the migration deliberately:
```bash
kubectl -n <ns> delete job <svc>-migrate
make k8s-apply
```

## Useful one-liners

```bash
# Tail logs for a deployment (auto-follows the latest pod)
kubectl -n auth logs deployment/auth-api -f

# psql into in-cluster Postgres
kubectl -n auth exec -it statefulset/auth-db -- psql -U auth -d auth

# Redis CLI
kubectl -n auth exec -it deployment/auth-cache -- redis-cli

# Port-forward auth-api to host (8080 → 8080)
kubectl -n auth port-forward svc/auth-api 8080:8080
# then: curl -s http://localhost:8080/health

# Force-restart a deployment after a config change
kubectl -n auth rollout restart deployment/auth-api

# See events for a flaky pod
kubectl -n auth describe pod -l app.kubernetes.io/name=auth,app.kubernetes.io/component=api
```

## When NOT to use this skill

- Iterating on Go business logic — compose (`run-service`) is faster (no image rebuild + kind load step).
- Authoring new manifests from scratch — read `.claude/rules/kubernetes-conventions.md` and `deploy/k8s/README.md` first; this skill is for *running* the stack, not designing it.
- Verifying production-shaped changes (Ingress, HPA, ExternalSecrets) — those overlays don't exist yet.
