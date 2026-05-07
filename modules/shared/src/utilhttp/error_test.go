package utilhttp

import (
	"errors"
	"testing"
)

func TestErrorType_String(t *testing.T) {
	cases := []struct {
		name string
		t    ErrorType
		want string
	}{
		{"NotFound", ErrNotFound, "not found"},
		{"BadRequest", ErrBadRequest, "bad request"},
		{"InternalServer", ErrInternalServer, "internal server error"},
		{"Unauthorized", ErrUnauthorized, "unauthorized"},
		{"Forbidden", ErrForbidden, "forbidden"},
		{"Conflict", ErrConflict, "conflict"},
		{"TooManyRequests", ErrTooManyRequests, "too many requests"},
		{"Database", ErrDatabase, "database error"},
		{"Unknown", ErrorType(999), "unknown error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.t.String(); got != tc.want {
				t.Errorf("ErrorType(%d).String() = %q, want %q", tc.t, got, tc.want)
			}
		})
	}
}

func TestAppError_Error_returnsMessage(t *testing.T) {
	e := AppError{Type: ErrBadRequest, Message: "boom"}
	if got := e.Error(); got != "boom" {
		t.Errorf("Error() = %q, want %q", got, "boom")
	}
}

// Each factory wraps the supplied error's message and stamps the matching
// ErrorType. Table-drive across all 8 factories so a regression in any one
// shows up as a single named subtest failure rather than a generic miss.
func TestNewErrorFactories_setTypeAndMessage(t *testing.T) {
	src := errors.New("underlying failure")

	cases := []struct {
		name     string
		got      AppError
		wantType ErrorType
	}{
		{"NotFound", NewNotFoundError(src).AppError, ErrNotFound},
		{"BadRequest", NewBadRequestError(src).AppError, ErrBadRequest},
		{"InternalServer", NewInternalServerError(src).AppError, ErrInternalServer},
		{"Unauthorized", NewUnauthorizedError(src).AppError, ErrUnauthorized},
		{"Forbidden", NewForbiddenError(src).AppError, ErrForbidden},
		{"Conflict", NewConflictError(src).AppError, ErrConflict},
		{"TooManyRequests", NewTooManyRequestsError(src).AppError, ErrTooManyRequests},
		{"Database", NewDBError(src).AppError, ErrDatabase},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got.Type != tc.wantType {
				t.Errorf("Type = %v, want %v", tc.got.Type, tc.wantType)
			}
			if tc.got.Message != src.Error() {
				t.Errorf("Message = %q, want %q", tc.got.Message, src.Error())
			}
		})
	}
}

// Each typed wrapper satisfies the error interface via the embedded AppError.
// Returning the wrappers as `error` (not the concrete type) is the dominant
// call pattern in the service layer, so verify the contract here.
func TestTypedWrappers_satisfyErrorInterface(t *testing.T) {
	src := errors.New("x")

	wrappers := []struct {
		name string
		err  error
	}{
		{"NotFound", NewNotFoundError(src)},
		{"BadRequest", NewBadRequestError(src)},
		{"InternalServer", NewInternalServerError(src)},
		{"Unauthorized", NewUnauthorizedError(src)},
		{"Forbidden", NewForbiddenError(src)},
		{"Conflict", NewConflictError(src)},
		{"TooManyRequests", NewTooManyRequestsError(src)},
		{"Database", NewDBError(src)},
	}
	for _, tc := range wrappers {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Error() != src.Error() {
				t.Errorf("Error() = %q, want %q", tc.err.Error(), src.Error())
			}
		})
	}
}
