# Testing Policy

A first wave of sample tests landed in `modules/auth/src/` covering the canonical layers (`domain`, `route/request`, `service`, `route`). Use them as the reference when writing tests for `audit` and `queue`. The auth-side coverage as of that change is ~28%, concentrated in the layers below.

## 1. What tests are expected to exist

| Layer | Test type | Required when |
|---|---|---|
| `domain/` | Unit (table-driven) | The type has logic beyond field assignment (`NewAccessToken`'s expiry math is the canonical example) |
| `service/` | Unit, with `db.Querier` mock | Always — services hold the business rules that route handlers depend on |
| `route/` | Handler test via `httptest` | When the handler does anything beyond `Decode → Service → Encode` (e.g., extracts headers, sets cookies, conditional status codes) |
| `infra/database/` | Integration, with real Postgres | When you add or change a sqlc query that has non-trivial SQL (joins, aggregates, transactions) |
| `shared/utilcache` | Integration, with real Redis | When you change cache logic with TTL or atomicity concerns. Pure helpers (e.g. `makeKey`) can be unit-tested — see `shared/utilcache/cache_test.go`. |
| `cmd/<binary>/` | Skipped | `main.go` is wiring; cover what it wires, not the wiring itself |

Pure data-shape constructors (`NewUser`, `NewLoginInput`) do not need tests — they are mechanical assignment.

## 2. Frameworks and conventions

### 2.1 stdlib `testing` + `go-cmp` for diffs

Use Go's `testing` package and table-driven tests. For deep-equality assertions, use **`github.com/google/go-cmp/cmp`** — adopted as a direct dependency in `auth/go.mod`. The canonical pattern:

```go
if diff := cmp.Diff(want, got, opts...); diff != "" {
    t.Errorf("Foo() mismatch (-want +got):\n%s", diff)
}
```

Reasons go-cmp is preferred over `reflect.DeepEqual` and over assertion libs like testify:
- Produces a readable `-want +got` diff on failure, making CI logs self-explanatory.
- Plays well with options (`cmpopts.EquateApproxTime`, `cmpopts.IgnoreFields`, custom `cmp.Transformer`s) without test-side branching.
- No fluent-API surface to learn — just one function (`cmp.Diff`).

`github.com/stretchr/testify` is present in `auth/go.mod` only as `// indirect` and is **not** sanctioned. Do not import it directly. If you need helpers (factory functions, fixtures), put them under `testutil/` — `.golangci.yml` already relaxes `errcheck` for that path.

#### Comparing types with unexported fields

`cmp.Diff` panics when descending into unexported fields (e.g. `time.Time.wall`). The two go-to escapes:

1. **`cmpopts.EquateApproxTime(margin)`** — registers a `time.Time` comparer with a tolerance. Use this for any test that exercises code calling `time.Now()`. A 1-second margin is generous enough to avoid flakes; do not crank it higher.
2. **`cmp.Transformer`** — when the field is a *named type* over `time.Time` (e.g. `domain.Expired`), cmp does not see it as `time.Time`. Add a transformer:

```go
var tokenCmpOpts = cmp.Options{
    cmp.Transformer("expiredAsTime", func(e domain.Expired) time.Time { return time.Time(e) }),
    cmpopts.EquateApproxTime(time.Second),
}
```

Reference: `modules/auth/src/domain/token_test.go`, `service/login_test.go`, `route/login_test.go` all share this pattern. Define the option set once per package and reuse across tests in that package.

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

See `modules/auth/src/route/request/login_test.go` for the canonical example. Cases include both boundary positives (`5 chars` for `Length(5, 100)`) and boundary negatives (`4 chars`). Always use `t.Run(tc.name, ...)` so failures point at a specific subcase.

For boolean-only assertions (e.g. "did `Validate()` return an error?") prefer stdlib over `cmp.Diff` — the latter adds noise without value when the answer is yes/no.

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

Reference: `modules/auth/src/service/login_test.go` defines a `stubQuerier` and three test cases (success, password-mismatch, repo-error) covering the full set of branches in `LoginService.Post`.

### 3.1 Asserting on typed errors

Service-layer tests assert on the concrete `utilhttp.*Error` wrapper, not on a bare string match:

```go
var dbErr utilhttp.DBError
if !errors.As(err, &dbErr) {
    t.Fatalf("expected utilhttp.DBError, got %T: %v", err, err)
}
if dbErr.Type != utilhttp.ErrDatabase {
    t.Errorf("error type = %v, want %v", dbErr.Type, utilhttp.ErrDatabase)
}
```

Note: this is the **only** sanctioned use of `errors.As` in this codebase, and it works because the target (`*utilhttp.DBError`) and the err's dynamic type (`utilhttp.DBError`) are identical — not because of any unwrapping. Do not use `errors.As(err, &utilhttp.AppError{})` to extract the embedded base struct: `BadRequestError`, `DBError`, etc. embed `AppError` by value rather than implementing `Unwrap`, and Go's `reflect.AssignableTo` does not bridge embedding. (`utilhttp.ResponseError` itself reaches `AppError` via an explicit type switch over each wrapper for the same reason.)

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

Assertions on response bodies should decode into the same `SuccessResponse[T]` / `ErrorResponse` types defined in `shared/utilhttp/response.go` — comparing decoded structs via `cmp.Diff` is more robust than string-matching JSON.

Reference: `modules/auth/src/route/login_test.go` shows the success-path test (decode into `utilhttp.SuccessResponse[response]`, diff with the token cmp options) and `handler_test.go` shows the minimal `/health` and 404 smoke tests.

### 4.1 Pinning a known issue

When a test exposes a real bug in production code that you are not authorizing to fix in the same change, name the test with a `_KNOWN_ISSUE` suffix and assert on the *current* (incorrect) behavior, with a comment naming what the test should assert once the bug is fixed.

Pinning is preferable to skipping (`t.Skip`) because it locks the regression in place — a future "fix" that doesn't actually fix breaks the test loudly. When the bug is fixed, the same change must drop the `_KNOWN_ISSUE` suffix and flip the assertion to the correct value.

### 4.2 Receiver-method gotcha for chi handlers

`route.Router()` has a pointer receiver, but `route.NewHandler` returns a value. `NewHandler(svc).Router()` does NOT compile — the result of a function call is not addressable. Bind to a local variable first:

```go
h := NewHandler(svc)
h.Router().ServeHTTP(rec, req)
```

This matches the pattern in `cmd/api/main.go`.

## 5. Integration tests against Postgres

Some queries cannot be meaningfully unit-tested. For those:

- Tag integration tests with a build tag: `//go:build integration` at the top of the file. Run with `go test -tags=integration ./...`. This keeps `make test` (the pre-push hook) from requiring Docker.
- Connect to the local compose Postgres on `localhost:5432` (audit) or `:5433` (auth) using the credentials in `compose.yml`.
- Each test must own its data: insert fixtures inside the test, clean up via `t.Cleanup` with a `DELETE` (not `TRUNCATE`, to avoid serializing on the table).
- Do **not** share state across tests via `TestMain`. Parallelism is preferred where data isolation allows it (`t.Parallel()` in subtests with unique IDs).

A future `make <service>-test-integration` target would be a reasonable place to land this; until then, document the build tag in any test file that uses it.

## 6. What `make test` does today

```
mkdir -p .coverage
out=$(pwd)/.coverage
for mod in auth audit queue shared; do
  ( cd modules/$mod/src && go test -v -coverprofile=$out/$mod-coverage.txt ./... )
  go tool cover -func=$out/$mod-coverage.txt | grep total
  go tool cover -html=$out/$mod-coverage.txt -o $out/$mod-coverage.html
done
```

Implications:
- Tests run **per module** in the module's own `go test` invocation. Cross-module test imports are impossible (each module is a separate Go module). If you need a fixture in two modules, put it in `shared`.
- `-v` is on, so all test output is shown — keep `t.Log` output meaningful.
- Coverage profiles + HTML for every module land under `.coverage/<mod>-coverage.{txt,html}` at the repo root and are gitignored. `make clean-coverage` removes them (along with any stale `modules/<m>/src/coverage.{txt,html}` left over from before the aggregation moved).
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
- **Don't add `testify` or `gomock` silently.** `go-cmp` is the sanctioned diff tool; new assertion frameworks are a team decision.
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
