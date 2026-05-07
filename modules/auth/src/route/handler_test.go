package route

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_health_returns200WithEmptyBody(t *testing.T) {
	h := NewHandler(stubLoginService{})
	router := h.Router()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if got := rec.Body.Len(); got != 0 {
		t.Errorf("body length = %d, want 0; body = %q", got, rec.Body.String())
	}
}

func TestHandler_unmatchedRoute_returns404(t *testing.T) {
	h := NewHandler(stubLoginService{})

	req := httptest.NewRequest(http.MethodGet, "/does/not/exist", nil)
	rec := httptest.NewRecorder()
	h.Router().ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
}
