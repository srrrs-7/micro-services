# Testing Policy

The codebase currently has **zero test files**. This document is therefore a forward-looking policy: the rules below dictate how tests must look when they are introduced. They are derived from the architecture, lint config, and toolchain — not from existing test code.

When the first tests land, this document should be revisited and tightened with concrete references.

## 1. What tests are expected to exist

| Layer | Test type | Required when |
|---|---|---|
| `domain/` | Unit (table-driven) | The type has logic beyond field assignment (`NewAccessToken`'s expiry math is the canonical example) |
| `service/` | Unit, with `db.Querier` mock | Always — services hold the business rules that route handlers depend on |
| `route/` | Handler test via `httptest` | When the handler does anything beyond `Decode → Service → Encode` (e.g., extracts headers, sets cookies, conditional status codes) |
| `infra/database/` | Integration, with real Postgres | When you add or change a sqlc query that has non-trivial SQL (joins, aggregates, transactions) |
| `infra/cache/` | Integration, with real Redis | When you add cache logic with TTL or atomicity concerns |
| `cmd/<binary>/` | Skipped | `main.go` is wiring; cover what it wires, not the wiring itself |

Pure data-shape constructors (`NewUser`, `NewLoginInput`) do not need tests — they are mechanical assignment.

## 2. Frameworks and conventions

### 2.1 Default to stdlib

Use Go's `testing` package and table-driven tests. The codebase has **no sanctioned assertion library yet**. `github.com/stretchr/testify` is present in `auth/go.mod` only as `// indirect`, which means it is not deliberately adopted. Do not introduce direct testify usage without team agreement; if you need richer assertions, write small helpers under `testutil/` (the `.golangci.yml` already has a relaxation for `testutil/` paths).

### 2.2 File layout

- Test files live next to the source: `service/login.go` → `service/login_test.go`.
- Same package (`package service`) for white-box tests; `package service_test` only when you specifically want black-box visibility (rare in this codebase given the small interfaces).
- Test helpers shared within a service go under `modules/<service>/src/testutil/`. Helpers shared across services go under `modules/shared/src/testutil/` and must remain dependency-free aside from stdlib.

### 2.3 Naming

```go
func TestLoginService_Post_returnsTokenForValidCredentials(t *testing.T) { ... }
func TestLoginService_Post_returnsUnauthorizedOnPasswordMismatch(t *testing.T) { ... }
```

`Test<Type>_<Method>_<scenario>` — read at a glance, no need to open the test body to know what failed.

### 2.4 Table-driven pattern

```go
func TestLoginRequest_Validate(t *testing.T) {
    cases := []struct {
        name    string
        req     LoginRequest
        wantErr bool
    }{
        {"valid", LoginRequest{Email: "a@b.co", Password: "pw1234"}, false},
        {"empty email", LoginRequest{Password: "pw1234"}, true},
        {"short password", LoginRequest{Email: "a@b.co", Password: "x"}, true},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            err := tc.req.Validate()
            if (err != nil) != tc.wantErr {
                t.Fatalf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
            }
        })
    }
}
```

`t.Run` with a `name` field is required so failures point at a specific subcase.

## 3. Mocking the database

The codebase already exposes the seam: `sqlc.yaml` has `emit_interface: true`, so each service has a `db.Querier` interface listing every query. Services depend on this interface (see `auth/service/login.go:13` — `repo db.Querier`).

To unit-test a service, hand-write a minimal stub of `Querier` in the test file:

```go
type stubQuerier struct {
    db.Querier  // embed to satisfy any methods we don't override
    getUser func(ctx context.Context, email string) (db.User, error)
}

func (s stubQuerier) GetUser(ctx context.Context, email string) (db.User, error) {
    return s.getUser(ctx, email)
}
```

Embedding the interface lets you stub one method and panic on the rest (calls to non-overridden methods will nil-panic, which is the desired loud failure). Do **not** introduce mocking frameworks (`gomock`, `mockery`) without team agreement — adding a code-gen step has cost and the embed-and-override pattern is sufficient.

## 4. HTTP handler tests

Use `net/http/httptest` directly:

```go
func TestHandler_login_returnsTokenJSON(t *testing.T) {
    h := NewHandler(stubLoginService{ /* ... */ })
    srv := httptest.NewServer(h.Router())
    defer srv.Close()

    resp, err := http.Post(srv.URL+"/auth/v1/token/login", "application/json",
        strings.NewReader(`{"email":"a@b.co","password":"pw1234"}`))
    // assert status, decode body via shared/utilhttp.SuccessResponse[response]
}
```

For lower-level tests (skip the network), use `httptest.NewRecorder` + call `h.Router().ServeHTTP(rec, req)` directly.

Assertions on response bodies should decode into the same `SuccessResponse[T]` / `ErrorResponse` types defined in `shared/utilhttp/response.go` — comparing decoded structs is more robust than string-matching JSON.

## 5. Integration tests against Postgres

Some queries cannot be meaningfully unit-tested. For those:

- Tag integration tests with a build tag: `//go:build integration` at the top of the file. Run with `go test -tags=integration ./...`. This keeps `make test` (the pre-push hook) from requiring Docker.
- Connect to the local compose Postgres on `localhost:5432` (audit) or `:5433` (auth) using the credentials in `compose.yml`.
- Each test must own its data: insert fixtures inside the test, clean up via `t.Cleanup` with a `DELETE` (not `TRUNCATE`, to avoid serializing on the table).
- Do **not** share state across tests via `TestMain`. Parallelism is preferred where data isolation allows it (`t.Parallel()` in subtests with unique IDs).

A future `make <service>-test-integration` target would be a reasonable place to land this; until then, document the build tag in any test file that uses it.

## 6. What `make test` does today

```
for mod in auth audit queue shared; do
  cd modules/$mod/src && go test -v -coverprofile=coverage.out ./...
  go tool cover -func=coverage.txt | grep total
  go tool cover -html=coverage.txt -o coverage.html
done
```

Implications:
- Tests run **per module** in the module's own `go test` invocation. Cross-module test imports are impossible (each module is a separate Go module). If you need a fixture in two modules, put it in `shared`.
- `-v` is on, so all test output is shown — keep `t.Log` output meaningful.
- Coverage HTML is written to `modules/<m>/src/coverage.html` and is gitignored.
- The pre-push hook runs `make test` — broken tests block `git push` (per `.githooks/pre-push`).

## 7. Running a single test

There is no make target for this. Run from the module:

```bash
cd modules/auth/src
go test -run TestLoginService_Post -v ./service/...
go test -run TestLoginService_Post/returnsUnauthorized -v ./service/...   # subtest
```

## 8. What not to do

- **Don't test generated code.** Files in `infra/database/db/` are sqlc output; sqlc itself is tested upstream. Bug suspicion in generated code means an incorrect `.sql` query, not a missing test.
- **Don't test `main.go` wiring.** Test the things `main.go` constructs.
- **Don't add `testify` or `gomock` silently.** Either is a team decision; raise it.
- **Don't write tests that hit the network or external services without a build tag.** Pre-push hook runs `make test` — flaky network in tests means flaky pushes.
- **Don't mock `*sql.DB`.** Mock `db.Querier` instead. The whole point of `emit_interface: true` is to give you a clean seam.
- **Don't comment-out failing tests** to get a push through. Either fix or `t.Skip("reason: link-to-issue")` with a tracking issue.

## 9. Coverage expectations

There is no enforced coverage threshold today. As tests land, the natural targets are:

- `service/` — high (>80%), since this is where business logic lives and the seam is cheap to mock.
- `route/` — medium, focused on non-trivial logic (header extraction, error mapping). Don't unit-test handlers that only call into `service`.
- `domain/` — high for any constructor with logic (token expiry, validations), zero for pure assignment constructors.
- `shared/utilhttp` — high; the error catalog and response helpers are the hottest contract in the system.

If you add a coverage gate to CI in the future, gate by **module** (each module's total) rather than overall, since the modules are independently tested and have different ratios of pure-glue code.
