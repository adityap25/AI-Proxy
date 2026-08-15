package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-proxy/internal/config"
)

func TestNewServer(t *testing.T) {
	srv := NewServer(&config.Config{Port: "8080"}, nil)

	if got, want := srv.httpServer.Addr, ":8080"; got != want {
		t.Errorf("expected address %q, got %q", want, got)
	}
	if srv.httpServer.Handler == nil {
		t.Fatal("expected server handler")
	}
	if srv.httpServer.ReadHeaderTimeout != readHeaderTimeout {
		t.Errorf("expected read-header timeout %v, got %v", readHeaderTimeout, srv.httpServer.ReadHeaderTimeout)
	}
	if srv.shutdownTimeout != shutdownTimeout {
		t.Errorf("expected shutdown timeout %v, got %v", shutdownTimeout, srv.shutdownTimeout)
	}

	recorder := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected health route status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestRunShutsDownWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := New("0", http.NewServeMux())
	if err := srv.Run(ctx); err != nil {
		t.Fatalf("expected clean shutdown, got %v", err)
	}
}
