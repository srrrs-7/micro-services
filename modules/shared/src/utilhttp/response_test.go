package utilhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-json"
	"github.com/google/go-cmp/cmp"
)

type payload struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func decodeSuccess[T any](t *testing.T, rec *httptest.ResponseRecorder) SuccessResponse[T] {
	t.Helper()
	var got SuccessResponse[T]
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode SuccessResponse: %v", err)
	}
	return got
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var got ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode ErrorResponse: %v", err)
	}
	return got
}

func TestResponseOk_writes200AndJSONBody(t *testing.T) {
	rec := httptest.NewRecorder()
	want := payload{Name: "alice", N: 7}

	ResponseOk(rec, want)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get(CONTENT_TYPE); ct != APPLICATION_JSON {
		t.Errorf("Content-Type = %q, want %q", ct, APPLICATION_JSON)
	}
	got := decodeSuccess[payload](t, rec)
	if diff := cmp.Diff(SuccessResponse[payload]{Data: want}, got); diff != "" {
		t.Errorf("body mismatch (-want +got):\n%s", diff)
	}
}

func TestResponseAccepted_writes202AndJSONBody(t *testing.T) {
	rec := httptest.NewRecorder()
	want := payload{Name: "bob", N: 1}

	ResponseAccepted(rec, want)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	got := decodeSuccess[payload](t, rec)
	if diff := cmp.Diff(SuccessResponse[payload]{Data: want}, got); diff != "" {
		t.Errorf("body mismatch (-want +got):\n%s", diff)
	}
}

// One row per typed wrapper covers the 8-case type switch in ResponseError.
// The plain-AppError row exercises the AppError branch directly, and the
// non-AppError row exercises the default fallback to 500.
func TestResponseError_mapsTypedErrorsToStatus(t *testing.T) {
	src := errors.New("underlying message")

	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"NotFound", NewNotFoundError(src), http.StatusNotFound},
		{"BadRequest", NewBadRequestError(src), http.StatusBadRequest},
		{"InternalServer", NewInternalServerError(src), http.StatusInternalServerError},
		{"Unauthorized", NewUnauthorizedError(src), http.StatusUnauthorized},
		{"Forbidden", NewForbiddenError(src), http.StatusForbidden},
		{"Conflict", NewConflictError(src), http.StatusConflict},
		{"TooManyRequests", NewTooManyRequestsError(src), http.StatusTooManyRequests},
		{"Database", NewDBError(src), http.StatusInternalServerError},
		{"AppError direct (BadRequest)",
			AppError{Type: ErrBadRequest, Message: "underlying message"},
			http.StatusBadRequest},
		{"AppError direct (NotFound)",
			AppError{Type: ErrNotFound, Message: "underlying message"},
			http.StatusNotFound},
		{"AppError unknown ErrorType falls back to 500",
			AppError{Type: ErrorType(999), Message: "underlying message"},
			http.StatusInternalServerError},
		{"plain error falls back to 500",
			errors.New("underlying message"),
			http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			ResponseError(rec, tc.err)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if ct := rec.Header().Get(CONTENT_TYPE); ct != APPLICATION_JSON {
				t.Errorf("Content-Type = %q, want %q", ct, APPLICATION_JSON)
			}
			got := decodeError(t, rec)
			if got.Error != "underlying message" {
				t.Errorf("body.Error = %q, want %q", got.Error, "underlying message")
			}
		})
	}
}

// ResponseError must round-trip the original error message into the
// ErrorResponse body — auth tests rely on substring matching against this.
func TestResponseError_preservesUnderlyingMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	ResponseError(rec, NewBadRequestError(errors.New("email is required")))

	got := decodeError(t, rec)
	if got.Error != "email is required" {
		t.Errorf("body.Error = %q, want %q", got.Error, "email is required")
	}
}

// SuccessResponse[T] / ErrorResponse both implement isResponse(); use this
// to detect accidental removal of the marker (the writeJSON helper accepts
// only the Response interface for type discipline).
func TestResponse_markerInterfaceIsSatisfied(t *testing.T) {
	var _ Response = ErrorResponse{}
	var _ Response = SuccessResponse[int]{}
}
