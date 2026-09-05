// Package cache defines the safety boundary for semantic cache entries.
package cache

import "strings"

// Scope identifies requests that may safely share a semantically cached
// response. This application is intentionally single-tenant, so no tenant or
// user identifier is included. Add one before sharing the service across
// independent customers or authorization domains.
//
// A change to the generation model or system prompt creates a different scope:
// answers produced under one set of instructions are never reused under
// another. The embedding model is included because vectors from different
// models must not be compared in the same pgvector index.
type Scope struct {
	GenerationModel     string
	EmbeddingModel      string
	SystemPromptVersion string
}

// Key returns the stable namespace stored with a cache entry and used to limit
// nearest-neighbor searches. Model names cannot contain a NUL byte, which lets
// it serve as an unambiguous separator.
func (s Scope) Key() string {
	return strings.Join([]string{
		"generation=" + s.GenerationModel,
		"embedding=" + s.EmbeddingModel,
		"system-prompt=" + s.SystemPromptVersion,
	}, "\x00")
}
