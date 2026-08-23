package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"ai-proxy/internal/ollama"
)

const readinessTimeout = 2 * time.Second

type handler struct {
	db        *sql.DB
	generator ollama.Generator
}

// NewHandler builds the application's HTTP router.
func NewHandler(db *sql.DB, generator ollama.Generator) http.Handler {
	h := handler{db: db, generator: generator}
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

// generate sends a prompt to the configured model. It does not cache or
// persist the prompt or model response.
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

	response, err := h.generator.Generate(r.Context(), request.Prompt)
	if err != nil {
		writeStatus(w, http.StatusBadGateway, "model generation failed")
		return
	}

	writeJSON(w, http.StatusOK, generationResponse{Response: response})
}

func writeStatus(w http.ResponseWriter, code int, status string) {
	writeJSON(w, code, map[string]string{"status": status})
}

func writeJSON(w http.ResponseWriter, code int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(response)
}
