package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeGenerator struct {
	response string
	err      error
	prompt   string
}

func (g *fakeGenerator) Generate(_ context.Context, prompt string) (string, error) {
	g.prompt = prompt
	return g.response, g.err
}

func TestHealthz(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got, want := recorder.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("expected body %q, got %q", want, got)
	}
}

func TestReadyWithoutDatabase(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
}

func TestGenerate(t *testing.T) {
	model := &fakeGenerator{response: "Semantic caching stores reusable responses."}
	recorder := httptest.NewRecorder()
	NewHandler(nil, model).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"What is semantic caching?"}`)))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got, want := model.prompt, "What is semantic caching?"; got != want {
		t.Errorf("expected model prompt %q, got %q", want, got)
	}
	if got, want := recorder.Body.String(), "{\"response\":\"Semantic caching stores reusable responses.\"}\n"; got != want {
		t.Errorf("expected body %q, got %q", want, got)
	}
}

func TestGenerateRejectsInvalidPrompt(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil, &fakeGenerator{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":" "}`)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestGenerateReturnsBadGatewayWhenModelFails(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil, &fakeGenerator{err: errors.New("Ollama is unavailable")}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"hello"}`)))

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, recorder.Code)
	}
}
