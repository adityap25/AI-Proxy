package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

const readinessTimeout = 2 * time.Second

type handler struct {
	db *sql.DB
}

// NewHandler builds the application's HTTP router.
func NewHandler(db *sql.DB) http.Handler {
	h := handler{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /ready", h.ready)
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

func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}
