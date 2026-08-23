package ollama

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Fatalf("expected /api/generate, got %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		const want = `{"model":"llama3.2:1b","prompt":"hello","stream":false}`
		if got := string(body); got != want {
			t.Errorf("expected request %s, got %s", want, got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"Hello!","done":true}`))
	}))
	defer server.Close()

	response, err := New(server.URL, "llama3.2:1b").Generate(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if response != "Hello!" {
		t.Errorf("expected response %q, got %q", "Hello!", response)
	}
}

func TestGenerateReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := New(server.URL, "missing-model").Generate(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected Ollama error")
	}
}
