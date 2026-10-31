package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harsh3dev/multi-resume/backend/internal/api"
)

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	api.Router(api.Deps{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("healthz returned %d %q", rec.Code, rec.Body.String())
	}
}

// Without auth middleware configured, API routes fail closed (503), never open.
func TestAPIFailsClosedWithoutAuth(t *testing.T) {
	rec := httptest.NewRecorder()
	api.Router(api.Deps{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unprotected /api/me returned %d", rec.Code)
	}
}
