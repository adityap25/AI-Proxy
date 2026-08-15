package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got, want := recorder.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("expected body %q, got %q", want, got)
	}
}

func TestReadyWithoutDatabase(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
}
