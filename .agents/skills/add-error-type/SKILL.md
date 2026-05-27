---
name: add-error-type
description: Use when adding a new error category to the shared/utilhttp error system (e.g. ErrPaymentRequired, ErrServiceUnavailable). The change spans two files that must be kept in sync; this skill prevents the common bug of editing only one.
---

# Add a new error type to `shared/utilhttp`

The `utilhttp` package defines a typed `AppError` with an `ErrorType` enum. Adding a new category requires three coordinated edits in two files, plus an HTTP status mapping. Skipping any step yields silent fallthrough to HTTP 500.

## Files to edit

1. `modules/shared/src/utilhttp/error.go`
2. `modules/shared/src/utilhttp/response.go`

## Steps

### 1. Extend the `ErrorType` enum (`error.go`)

Add a new constant in the existing `iota` block. Order matters less than completeness — the `exhaustive` linter will flag any switch that doesn't cover all values.

```go
const (
    ErrNotFound ErrorType = iota
    ErrBadRequest
    ErrInternalServer
    ErrUnauthorized
    ErrForbidden
    ErrConflict
    ErrTooManyRequests
    ErrDatabase
    ErrYourNewType   // <-- add here
)
```

### 2. Add the `String()` case (`error.go`)

Update the `func (e ErrorType) String() string` switch:

```go
case ErrYourNewType:
    return "your new type"
```

### 3. Add the typed wrapper (`error.go`)

Follow the existing pattern (one-line struct + constructor):

```go
type YourNewTypeError struct{ AppError }

func NewYourNewTypeError(err error) YourNewTypeError {
    return YourNewTypeError{newAppError(ErrYourNewType, err)}
}
```

### 4. Map to HTTP status (`response.go`)

In `ResponseError`, add a case to the switch:

```go
case ErrYourNewType:
    responseYourNewType(w, appErr)
```

Then add the helper alongside the existing `responseXxx` family:

```go
func responseYourNewType(w http.ResponseWriter, err error) {
    writeJSON(w, http.StatusXxx, ErrorResponse{Error: err.Error()})
}
```

Pick the HTTP status carefully — this is the contract callers see. Common additions:

| Category | Status |
|---|---|
| `ErrPaymentRequired` | 402 |
| `ErrUnprocessable` | 422 |
| `ErrServiceUnavailable` | 503 |
| `ErrGatewayTimeout` | 504 |

### 5. Verify

```
cd modules/shared/src && go vet ./... && golangci-lint run ./...
```

The `exhaustive` linter will fail at this point if step 2 or step 4 was missed. The `dupl` linter will not fail on a new `responseXxx` helper because they're structurally similar by design.

### 6. Use it from a service

```go
return nil, utilhttp.NewYourNewTypeError(fmt.Errorf("rate limit exceeded for tenant %s", tenant))
```

The route handler's existing `utilhttp.ResponseError(w, err)` call will pick up the new mapping automatically — no route changes needed.

## Anti-patterns

- **Adding the constant without the wrapper**: services have no way to construct the error.
- **Adding the wrapper without the response case**: `ResponseError` falls through to `responseInternalServerError`, so callers see 500 instead of your intended status.
- **Mapping multiple categories to the same helper**: each category should have its own `responseXxx` so future status tweaks are local.
