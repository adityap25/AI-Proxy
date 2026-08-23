// Package ollama provides access to the local Ollama generation API.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const requestTimeout = 2 * time.Minute

// Generator is the model capability used by the application. Keeping this
// interface small makes the later cache layer easy to test and replace.
type Generator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Client calls an Ollama model over its HTTP API. It does not persist prompts
// or responses.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// New creates a client for an Ollama host, such as http://localhost:11434.
func New(host, model string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(host, "/"),
		model:      model,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

// Generate sends prompt to the configured model and returns its complete text
// response. Streaming is disabled so a single response can be returned.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(generateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("encode Ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create Ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		if readErr != nil {
			return "", fmt.Errorf("read Ollama error response: %w", readErr)
		}
		return "", fmt.Errorf("Ollama returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Ollama response: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("Ollama generation error: %s", result.Error)
	}

	return result.Response, nil
}
