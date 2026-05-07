package route

import (
	"auth/domain"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"shared/utilhttp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var tokenCmpOpts = cmp.Options{
	cmp.Transformer("expiredAsTime", func(e domain.Expired) time.Time { return time.Time(e) }),
	cmpopts.EquateApproxTime(time.Second),
}

// stubLoginService satisfies the route-package loginService interface.
// Define a single shared stub here so handler_test.go can reuse it for
// /health smoke tests without a per-test definition.
type stubLoginService struct {
	post func(ctx context.Context, input domain.LoginInput) (*domain.Token, error)
}

func (s stubLoginService) Post(ctx context.Context, input domain.LoginInput) (*domain.Token, error) {
	if s.post == nil {
		return &domain.Token{}, nil
	}
	return s.post(ctx, input)
}

func TestHandler_login_returns200WithTokenForValidRequest(t *testing.T) {
	const (
		email    = "alice@example.com"
		password = "pw1234"
	)

	var called bool
	svc := stubLoginService{
		post: func(_ context.Context, in domain.LoginInput) (*domain.Token, error) {
			called = true
			if in.Email != email || in.Password != password {
				t.Errorf("service got LoginInput{%q, %q}, want {%q, %q}",
					in.Email, in.Password, email, password)
			}
			return &domain.Token{UserID: domain.UserID("u-1")}, nil
		},
	}

	body := `{"email":"` + email + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/token/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h := NewHandler(svc)
	h.Router().ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d; body = %s", got, want, rec.Body.String())
	}
	if !called {
		t.Error("loginService.Post was not called")
	}

	var got utilhttp.SuccessResponse[response]
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	want := utilhttp.SuccessResponse[response]{
		Data: response{Token: &domain.Token{UserID: domain.UserID("u-1")}},
	}
	if diff := cmp.Diff(want, got, tokenCmpOpts); diff != "" {
		t.Errorf("response body mismatch (-want +got):\n%s", diff)
	}
}

// Pins a known issue in shared/utilhttp.ResponseError: it extracts the
// typed wrapper via `errors.As(err, &AppError{})`, but BadRequestError /
// DBError / etc. embed AppError by value rather than implementing Unwrap,
// so Go's reflect.AssignableTo does not bridge the gap. As a result every
// service-returned error currently falls through to 500.
//
// When ResponseError is fixed (e.g. by switching on the concrete wrapper
// types or adding Unwrap to AppError), this test will fail and the
// expected status should be updated to http.StatusBadRequest.
func TestHandler_login_returnsServerErrorForMalformedBody_KNOWN_ISSUE(t *testing.T) {
	svc := stubLoginService{
		post: func(context.Context, domain.LoginInput) (*domain.Token, error) {
			t.Error("service should not be invoked when body decode fails")
			return nil, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/v1/token/login", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h := NewHandler(svc)
	h.Router().ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusInternalServerError; got != want {
		t.Errorf("status = %d, want %d (after ResponseError fix, this should be %d)",
			got, want, http.StatusBadRequest)
	}

	var errResp utilhttp.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(errResp.Error, "unmarshal request body") {
		t.Errorf("error response %q does not surface the decode failure", errResp.Error)
	}
}
