---
name: regen-sqlc
description: Use after editing files under modules/<service>/src/infra/database/queries/ or modules/<service>/src/infra/database/migrations/ to regenerate the sqlc-typed Go code and verify nothing downstream broke.
---

# Regenerate sqlc code

The `infra/database/db/` package in `audit` and `auth` is generated from:
- `infra/database/migrations/*.sql` (schema sqlc reads from)
- `infra/database/queries/*.sql` (queries sqlc compiles)
- `infra/database/sqlc.yaml` (config)

Never hand-edit `infra/database/db/*.go`. Regenerate after any change to migrations or queries.

## Steps

### 1. Run the generator

```bash
make audit-sqlc-gen   # for the audit service
# or
make auth-sqlc-gen    # for the auth service
```

These run `sqlc generate` from the module's `infra/database/` directory. `sqlc` is pre-installed in the devcontainer; if running outside, install via `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`.

### 2. Inspect what changed

```bash
git status modules/<service>/src/infra/database/db/
git diff modules/<service>/src/infra/database/db/
```

Expected outputs are regenerations of `<table>.sql.go`, `models.go`, `querier.go`, and `db.go`. Anything else suggests sqlc config drift.

### 3. Verify it compiles

```bash
cd modules/<service>/src && go vet ./...
```

If `vet` fails, the most likely causes are:
- A query references a column that doesn't exist in any migration (schema/query mismatch).
- An existing call site uses a function signature that changed (e.g. parameter order or return type).
- A test mock implements `db.Querier` and is now missing a method — the `sqlc.yaml` has `emit_interface: true`, so adding a query expands the interface.

### 4. Run lint

```bash
cd modules/<service>/src && golangci-lint run ./...
```

`gofmt` / `goimports` are formatters in the lint config — generated code should already satisfy them, but run it to be sure.

### 5. Run tests

```bash
cd modules/<service>/src && go test ./...
```

Or `make test` to run all four modules.

## Config notes

`sqlc.yaml` for both services uses:
- `engine: postgresql`
- `package: db`
- `out: db`
- `emit_json_tags: true` — generated structs have `json:"..."` tags
- `emit_prepared_queries: false` — uses `database/sql` query methods directly
- `emit_interface: true` — generates a `Querier` interface (the abstraction services depend on)
- `emit_exact_table_names: false` — sqlc singularizes/pluralizes struct names

Don't change these without coordination — service code depends on these exact emission settings.

## When sqlc regeneration is NOT enough

If you changed `migrations/`, you also need to apply the migration to your local DB. `sqlc generate` reads schema files but does not apply them — apply via:

```bash
make <service>-migrate
```

Schema changes that drop or rename columns will likely break existing queries. Search for uses of the affected column:

```bash
grep -rn "ColumnName" modules/<service>/src/infra/database/queries/
```

Fix the queries first, then regenerate.
