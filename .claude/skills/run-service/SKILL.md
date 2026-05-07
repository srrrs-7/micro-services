---
name: run-service
description: Use when the user wants to run, restart, or smoke-test the audit or auth service stack locally. Handles the Docker Compose orchestration, the auth_cache gotcha, migrations, and a curl-based health check.
---

# Run a service stack locally

Each service has a `make <service>` meta-target that builds, starts, and migrates. Two non-obvious gotchas this skill exists to handle:

1. `make auth-up` does NOT start `auth_cache` even though `auth-api` depends on it via `compose.yml`. The auth API will fail its `cache.Ping` and exit on startup if Redis isn't running.
2. `make audit-migrate` and `make auth-migrate` require the corresponding `*-db` container to be healthy first — the `migrate apply` step connects directly.

## Auth service

```bash
# First time or after Dockerfile changes:
make auth-build

# Bring up DB, Redis (NOT included in auth-up — start explicitly), API, migrator:
docker compose up -d auth-db auth_cache
make auth-up
make auth-migrate
```

Or, in one shot for a clean slate:
```bash
docker compose up -d auth_cache && make auth
```

The auth API exposes `:8080` inside the container; map it with `compose.override.yml` if you want host access. The auth Postgres is on host port `5433` (note: not 5432).

Smoke test:
```bash
docker compose exec auth-api wget -qO- http://localhost:8080/health
# Or, if 8080 is mapped to host:
curl -s http://localhost:8080/health
```

## Audit service

```bash
make audit
```

This brings up `audit-api`, `audit-worker`, `audit-db`, `queue-api`, and `migrator`. Audit Postgres is on host port `5432`. The audit binaries are still placeholders (`fmt.Println("hello audit api")`), so don't expect HTTP yet — verify only that the containers start without crashing:

```bash
docker compose ps
docker compose logs audit-api --tail=20
```

## Tear down

```bash
make auth-down    # stops auth-api, auth-db
make audit-down   # stops audit-api, audit-worker, audit-db, queue-api
docker compose down auth_cache  # if you brought it up manually
```

For a hard reset (drop DB volumes too):
```bash
docker compose down -v
```

## Common failures

- **`auth-api` exits with `dial tcp ... connection refused`** to Redis: `auth_cache` isn't running. Start it.
- **`migrate apply` errors with `connection refused`**: the `*-db` container isn't healthy yet. Wait for the healthcheck (`docker compose ps` shows `(healthy)`) and re-run.
- **Port conflict on 5432/5433/6379**: another Postgres/Redis is bound on the host. Either stop it or override the host port in `compose.override.yml`.
- **Stale image after code change**: `make <service>-build` rebuilds; `make <service>` does build → up → migrate so it's the safer choice after edits.

## Useful one-liners

```bash
# Tail logs for one service
docker compose logs -f auth-api

# psql into a service DB (audit example)
docker compose exec audit-db psql -U audit -d audit

# Redis CLI
docker compose exec auth_cache redis-cli
```
