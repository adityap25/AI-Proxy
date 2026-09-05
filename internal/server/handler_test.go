package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-proxy/internal/cache"
)

type fakeGenerator struct {
	response string
	err      error
	prompt   string
}

type fakeEmbedder struct {
	embedding []float32
	err       error
	calls     int
}

func (e *fakeEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	e.calls++
	return e.embedding, e.err
}

type fakeCacheRepository struct {
	match      *cache.Match
	findErr    error
	storeErr   error
	findCalls  int
	storeCalls int
	stored     cache.Entry
}

func (c *fakeCacheRepository) FindClosest(_ context.Context, _ []float32, _ string, _ float64) (*cache.Match, error) {
	c.findCalls++
	return c.match, c.findErr
}

func (c *fakeCacheRepository) Store(_ context.Context, entry cache.Entry) (*cache.Entry, error) {
	c.storeCalls++
	c.stored = entry
	return &entry, c.storeErr
}

func (g *fakeGenerator) Generate(_ context.Context, prompt string) (string, error) {
	g.prompt = prompt
	return g.response, g.err
}

func TestHealthz(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil, nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got, want := recorder.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("expected body %q, got %q", want, got)
	}
}

func TestReadyWithoutDatabase(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil, nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
}

func TestGenerate(t *testing.T) {
	model := &fakeGenerator{response: "Semantic caching stores reusable responses."}
	recorder := httptest.NewRecorder()
	NewHandler(nil, model, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"What is semantic caching?"}`)))

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
	NewHandler(nil, &fakeGenerator{}, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":" "}`)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestGenerateReturnsBadGatewayWhenModelFails(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(nil, &fakeGenerator{err: errors.New("Ollama is unavailable")}, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"hello"}`)))

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, recorder.Code)
	}
}

func TestGenerateReturnsCachedResponseWithoutCallingModel(t *testing.T) {
	model := &fakeGenerator{err: errors.New("must not be called")}
	embedder := &fakeEmbedder{embedding: []float32{0.25, -0.5}}
	repository := &fakeCacheRepository{match: &cache.Match{Entry: cache.Entry{Response: "cached response"}}}
	recorder := httptest.NewRecorder()

	NewHandler(nil, model, testSemanticCache(repository, embedder)).ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"hello"}`)),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got, want := recorder.Body.String(), "{\"response\":\"cached response\"}\n"; got != want {
		t.Errorf("expected body %q, got %q", want, got)
	}
	if got, want := recorder.Header().Get(cacheStatusHeader), "ai-proxy; hit"; got != want {
		t.Errorf("Cache-Status = %q, want %q", got, want)
	}
	if model.prompt != "" {
		t.Errorf("model was called with %q on cache hit", model.prompt)
	}
	if repository.storeCalls != 0 {
		t.Errorf("Store called %d times on cache hit", repository.storeCalls)
	}
}

func TestGenerateStoresModelResponseOnCacheMiss(t *testing.T) {
	now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC)
	model := &fakeGenerator{response: "fresh response"}
	embedder := &fakeEmbedder{embedding: []float32{0.25, -0.5}}
	repository := &fakeCacheRepository{}
	semanticCache := testSemanticCache(repository, embedder)
	semanticCache.now = func() time.Time { return now }
	recorder := httptest.NewRecorder()

	NewHandler(nil, model, semanticCache).ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"hello"}`)),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if repository.storeCalls != 1 {
		t.Fatalf("Store called %d times, want 1", repository.storeCalls)
	}
	if got, want := repository.stored.Response, "fresh response"; got != want {
		t.Errorf("stored response = %q, want %q", got, want)
	}
	if got, want := repository.stored.ExpiresAt, now.Add(time.Hour); !got.Equal(want) {
		t.Errorf("stored expiration = %v, want %v", got, want)
	}
	if embedder.calls != 1 {
		t.Errorf("Embed called %d times, want 1", embedder.calls)
	}
	if got, want := recorder.Header().Get(cacheStatusHeader), "ai-proxy; fwd=miss; stored"; got != want {
		t.Errorf("Cache-Status = %q, want %q", got, want)
	}
}

func TestGenerateFailsOpenWhenCacheLookupFails(t *testing.T) {
	model := &fakeGenerator{response: "fresh response"}
	embedder := &fakeEmbedder{embedding: []float32{0.25}}
	repository := &fakeCacheRepository{findErr: errors.New("database unavailable")}
	recorder := httptest.NewRecorder()

	NewHandler(nil, model, testSemanticCache(repository, embedder)).ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"hello"}`)),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got, want := recorder.Body.String(), "{\"response\":\"fresh response\"}\n"; got != want {
		t.Errorf("expected body %q, got %q", want, got)
	}
	if got, want := recorder.Header().Get(cacheStatusHeader), "ai-proxy; fwd=bypass; stored"; got != want {
		t.Errorf("Cache-Status = %q, want %q", got, want)
	}
}

func TestGenerateFailsOpenWhenEmbeddingFails(t *testing.T) {
	model := &fakeGenerator{response: "fresh response"}
	embedder := &fakeEmbedder{err: errors.New("embedding model unavailable")}
	repository := &fakeCacheRepository{}
	recorder := httptest.NewRecorder()

	NewHandler(nil, model, testSemanticCache(repository, embedder)).ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"hello"}`)),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if repository.findCalls != 0 || repository.storeCalls != 0 {
		t.Errorf("repository calls = find %d, store %d; want none", repository.findCalls, repository.storeCalls)
	}
	if got, want := recorder.Header().Get(cacheStatusHeader), "ai-proxy; fwd=bypass"; got != want {
		t.Errorf("Cache-Status = %q, want %q", got, want)
	}
}

func TestGenerateFailsOpenWhenCacheStoreFails(t *testing.T) {
	model := &fakeGenerator{response: "fresh response"}
	embedder := &fakeEmbedder{embedding: []float32{0.25}}
	repository := &fakeCacheRepository{storeErr: errors.New("database unavailable")}
	recorder := httptest.NewRecorder()

	NewHandler(nil, model, testSemanticCache(repository, embedder)).ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"prompt":"hello"}`)),
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if got, want := recorder.Body.String(), "{\"response\":\"fresh response\"}\n"; got != want {
		t.Errorf("expected body %q, got %q", want, got)
	}
	if got, want := recorder.Header().Get(cacheStatusHeader), "ai-proxy; fwd=miss"; got != want {
		t.Errorf("Cache-Status = %q, want %q", got, want)
	}
}

func testSemanticCache(repository cacheRepository, embedder *fakeEmbedder) *SemanticCache {
	return &SemanticCache{
		Repository: repository,
		Embedder:   embedder,
		Scope: cache.Scope{
			GenerationModel:     "llama",
			EmbeddingModel:      "nomic",
			SystemPromptVersion: "none",
		},
		MinSimilarity: 0.9,
		TTL:           time.Hour,
	}
}
