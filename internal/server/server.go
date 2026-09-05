package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-proxy/internal/cache"
	"ai-proxy/internal/config"
	"ai-proxy/internal/ollama"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Server owns the HTTP server lifecycle.
type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
}

// New creates a server for port using handler.
func New(port string, handler http.Handler) *Server {
	return &Server{
		shutdownTimeout: shutdownTimeout,
		httpServer: &http.Server{
			Addr:              ":" + port,
			Handler:           handler,
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
		},
	}
}

// NewServer creates the gateway server with its application routes.
func NewServer(cfg *config.Config, db *sql.DB, generator ollama.Generator) *Server {
	semanticCache := &SemanticCache{
		Repository: cache.NewRepository(db),
		Embedder:   ollama.New(cfg.OllamaHost, cfg.EmbedModel),
		Scope: cache.Scope{
			GenerationModel:     cfg.LLMModel,
			EmbeddingModel:      cfg.EmbedModel,
			SystemPromptVersion: cfg.SystemPromptVersion,
		},
		MinSimilarity: cfg.CacheSimilarity,
		TTL:           cfg.CacheTTL,
	}
	return New(cfg.Port, NewHandler(db, generator, semanticCache))
}

// Start runs the HTTP server until the process receives SIGINT or SIGTERM.
func (s *Server) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return s.Run(ctx)
}

// Run starts the server and gracefully shuts it down when ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()

	log.Printf("Gateway server listening on %s", s.httpServer.Addr)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Println("Shutdown requested. Initiating graceful shutdown...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	err := <-errCh
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	log.Println("Gateway server stopped cleanly")
	return nil
}

// Close immediately closes the listener and active connections. Prefer Run or
// Start for normal shutdowns, which allow in-flight requests to finish.
func (s *Server) Close() error {
	return s.httpServer.Close()
}
