// Package cache defines the safety boundary for semantic cache entries.
package cache

import (
	"strconv"
	"strings"
)

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
// nearest-neighbor searches. Values are length-prefixed so the key remains
// unambiguous while containing only text PostgreSQL can store.
func (s Scope) Key() string {
	return strings.Join([]string{
		scopeKeyPart("generation", s.GenerationModel),
		scopeKeyPart("embedding", s.EmbeddingModel),
		scopeKeyPart("system-prompt", s.SystemPromptVersion),
	}, "|")
}

func scopeKeyPart(name, value string) string {
	return name + "=" + strconv.Itoa(len(value)) + ":" + value
}
