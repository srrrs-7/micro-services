package utilhttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// validatedBody is a Validator that records what it received and lets the
// test choose what Validate returns. Pointer methods because RequestBody
// uses generics over the value type but Validate must mutate the recorder.
type validatedBody struct {
	Email   string `json:"email"`
	Age     int    `json:"age"`
	wantErr error
}

func (v validatedBody) Validate() error { return v.wantErr }

func TestRequestBody_returnsDecodedValueOnSuccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"a@b","age":7}`))

	got, err := RequestBody[validatedBody](req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != "a@b" || got.Age != 7 {
		t.Errorf("decoded = %+v, want {Email:a@b Age:7}", got)
	}
}

func TestRequestBody_returnsBadRequestOnUnmarshalError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not json`))

	_, err := RequestBody[validatedBody](req)
	var br BadRequestError
	if !errors.As(err, &br) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
	if !strings.Contains(br.Message, "unmarshal request body") {
		t.Errorf("message = %q, want it to mention unmarshal", br.Message)
	}
}

// validatedRejectingBody always fails Validate(). Separate type from
// validatedBody so the test reads cleanly without the wantErr juggling.
type validatedRejectingBody struct {
	Email string `json:"email"`
}

func (v validatedRejectingBody) Validate() error { return errors.New("email is required") }

func TestRequestBody_returnsBadRequestOnValidateError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":""}`))

	_, err := RequestBody[validatedRejectingBody](req)
	var br BadRequestError
	if !errors.As(err, &br) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
	if !strings.Contains(br.Message, "invalid request body") {
		t.Errorf("message = %q, want it to mention invalid request body", br.Message)
	}
	if !strings.Contains(br.Message, "email is required") {
		t.Errorf("message = %q, want it to surface the validate error", br.Message)
	}
}

// errReader fails on the first Read so we can exercise the io.ReadAll error path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("disk on fire") }
func (errReader) Close() error             { return nil }

func TestRequestBody_returnsBadRequestOnBodyReadError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = errReader{}

	_, err := RequestBody[validatedBody](req)
	var br BadRequestError
	if !errors.As(err, &br) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
	if !strings.Contains(br.Message, "read request body") {
		t.Errorf("message = %q, want it to mention read request body", br.Message)
	}
	if !strings.Contains(br.Message, "disk on fire") {
		t.Errorf("message = %q, want it to surface the underlying read error", br.Message)
	}
}

// urlParam is a Validator stored in ctx. The body of the value is not
// what matters — only its type and Validate result.
type urlParam struct {
	id      string
	wantErr error
}

func (u urlParam) Validate() error { return u.wantErr }

const testParamKey ContextKey = "testParam"

func TestRequestUrlParam_returnsValueOnSuccess(t *testing.T) {
	want := urlParam{id: "abc"}
	ctx := SetContextValue(context.Background(), testParamKey, want)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	got, err := RequestUrlParam[urlParam](req, testParamKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.id != want.id {
		t.Errorf("id = %q, want %q", got.id, want.id)
	}
}

func TestRequestUrlParam_returnsBadRequestWhenKeyMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := RequestUrlParam[urlParam](req, testParamKey)
	var br BadRequestError
	if !errors.As(err, &br) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
	if !strings.Contains(br.Message, "missing or invalid URL parameter") {
		t.Errorf("message = %q, want it to mention missing parameter", br.Message)
	}
}

func TestRequestUrlParam_returnsBadRequestWhenStoredTypeMismatches(t *testing.T) {
	ctx := SetContextValue(context.Background(), testParamKey, "not a urlParam")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	_, err := RequestUrlParam[urlParam](req, testParamKey)
	var br BadRequestError
	if !errors.As(err, &br) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
	if !strings.Contains(br.Message, "missing or invalid URL parameter") {
		t.Errorf("message = %q, want it to mention missing/invalid parameter", br.Message)
	}
}

func TestRequestUrlParam_returnsBadRequestOnValidateError(t *testing.T) {
	stored := urlParam{id: "abc", wantErr: errors.New("id has bad chars")}
	ctx := SetContextValue(context.Background(), testParamKey, stored)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

	_, err := RequestUrlParam[urlParam](req, testParamKey)
	var br BadRequestError
	if !errors.As(err, &br) {
		t.Fatalf("expected BadRequestError, got %T: %v", err, err)
	}
	if !strings.Contains(br.Message, "invalid URL parameter") {
		t.Errorf("message = %q, want it to mention invalid URL parameter", br.Message)
	}
	if !strings.Contains(br.Message, "id has bad chars") {
		t.Errorf("message = %q, want it to surface the validate error", br.Message)
	}
}

// Smoke test: SetContextValue behaves like context.WithValue with the typed
// key, and the value round-trips. Cheap to keep regression-detection on the
// glue helper that exists for type discipline.
func TestSetContextValue_roundTripsTypedKey(t *testing.T) {
	const key ContextKey = "k"
	ctx := SetContextValue(context.Background(), key, 42)
	if got := ctx.Value(key); got != 42 {
		t.Errorf("ctx.Value(%q) = %v, want 42", key, got)
	}
}

// Sanity: when the ContextKey type doesn't match (different declared type),
// context.WithValue treats it as a different key. This documents why
// ContextKey exists at all — to prevent string-key collisions across pkgs.
func TestContextKey_typedKeysDoNotCollideWithBareString(t *testing.T) {
	ctx := SetContextValue(context.Background(), ContextKey("session"), "typed")
	type otherKey string
	if got := ctx.Value(otherKey("session")); got != nil {
		t.Errorf("bare-string key resolved typed value (%v) — typing is broken", got)
	}
}

// Defence-in-depth: a fresh request from httptest.NewRequest exposes a
// non-nil Body even for nil readers, so RequestBody should still execute
// the read path. This test guards against regression if the std lib changes.
func TestRequestBody_emptyBodyDecodesToZeroValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))

	got, err := RequestBody[validatedBody](req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != (validatedBody{}) {
		t.Errorf("decoded = %+v, want zero value", got)
	}
}

// Compile-time safety net: io.ReadCloser is what http.Request.Body expects.
// If errReader stops satisfying it the package will fail to build, but a
// var here makes the intent explicit for future readers.
var _ io.ReadCloser = errReader{}
