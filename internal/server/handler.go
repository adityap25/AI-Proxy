package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-proxy/internal/cache"
	"ai-proxy/internal/ollama"
)

const (
	readinessTimeout  = 2 * time.Second
	cacheStatusHeader = "Cache-Status"
	cacheName         = "ai-proxy"
)

type handler struct {
	db        *sql.DB
	generator ollama.Generator
	cache     *SemanticCache
}

type cacheRepository interface {
	FindClosest(ctx context.Context, embedding []float32, scope string, minSimilarity float64) (*cache.Match, error)
	Store(ctx context.Context, entry cache.Entry) (*cache.Entry, error)
}

// SemanticCache contains the dependencies and policy used to cache generated
// responses. Leaving it unset disables caching.
type SemanticCache struct {
	Repository    cacheRepository
	Embedder      ollama.Embedder
	Scope         cache.Scope
	MinSimilarity float64
	TTL           time.Duration
	now           func() time.Time
}

// NewHandler builds the application's HTTP router. A nil semanticCache
// disables semantic caching.
func NewHandler(db *sql.DB, generator ollama.Generator, semanticCache *SemanticCache) http.Handler {
	h := handler{db: db, generator: generator, cache: semanticCache}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /ready", h.ready)
	mux.HandleFunc("POST /generate", h.generate)
	return mux
}

// healthz confirms that the HTTP service is running.
func (h handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeStatus(w, http.StatusOK, "ok")
}

// ready confirms that the service can reach PostgreSQL.
func (h handler) ready(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeStatus(w, http.StatusServiceUnavailable, "not ready")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		writeStatus(w, http.StatusServiceUnavailable, "not ready")
		return
	}

	writeStatus(w, http.StatusOK, "ready")
}

type generationRequest struct {
	Prompt string `json:"prompt"`
}

type generationResponse struct {
	Response string `json:"response"`
}

// generate returns a compatible cached response when one exists, otherwise it
// calls the model and makes the result available to later requests.
func (h handler) generate(w http.ResponseWriter, r *http.Request) {
	if h.generator == nil {
		writeStatus(w, http.StatusServiceUnavailable, "model is unavailable")
		return
	}

	var request generationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeStatus(w, http.StatusBadRequest, "request body must contain JSON")
		return
	}
	if strings.TrimSpace(request.Prompt) == "" {
		writeStatus(w, http.StatusBadRequest, "prompt is required")
		return
	}

	ctx := r.Context()
	var embedding []float32
	cacheStatus := ""
	if h.cacheEnabled() {
		var err error
		embedding, err = h.cache.Embedder.Embed(ctx, request.Prompt)
		if err != nil {
			log.Printf("semantic cache embedding failed: %v", err)
			cacheStatus = cacheName + "; fwd=bypass"
		} else {
			match, lookupErr := h.cache.Repository.FindClosest(ctx, embedding, h.cache.Scope.Key(), h.cache.MinSimilarity)
			if lookupErr != nil {
				log.Printf("semantic cache lookup failed: %v", lookupErr)
				cacheStatus = cacheName + "; fwd=bypass"
			} else if match != nil {
				w.Header().Set(cacheStatusHeader, cacheName+"; hit")
				writeJSON(w, http.StatusOK, generationResponse{Response: match.Response})
				return
			} else {
				cacheStatus = cacheName + "; fwd=miss"
			}
		}
	}

	response, err := h.generator.Generate(ctx, request.Prompt)
	if err != nil {
		writeStatus(w, http.StatusBadGateway, "model generation failed")
		return
	}
	if len(embedding) > 0 {
		now := time.Now
		if h.cache.now != nil {
			now = h.cache.now
		}
		_, storeErr := h.cache.Repository.Store(ctx, cache.Entry{
			Prompt:    request.Prompt,
			Response:  response,
			Embedding: embedding,
			Scope:     h.cache.Scope.Key(),
			ExpiresAt: now().Add(h.cache.TTL),
		})
		if storeErr != nil {
			log.Printf("semantic cache store failed: %v", storeErr)
		} else if cacheStatus != "" {
			cacheStatus += "; stored"
		}
	}
	if cacheStatus != "" {
		w.Header().Set(cacheStatusHeader, cacheStatus)
	}

	writeJSON(w, http.StatusOK, generationResponse{Response: response})
}

func (h handler) cacheEnabled() bool {
	return h.cache != nil && h.cache.Repository != nil && h.cache.Embedder != nil &&
		h.cache.MinSimilarity >= 0 && h.cache.MinSimilarity <= 1 && h.cache.TTL > 0
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	writeJSON(w, code, map[string]string{"status": status})
}

func writeJSON(w http.ResponseWriter, code int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(response)
}
