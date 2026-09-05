package cache

import (
	"strings"
	"testing"
)

func TestScopeKeyChangesWhenAnswerSemanticsChange(t *testing.T) {
	base := Scope{
		GenerationModel:     "llama3.2:1b",
		EmbeddingModel:      "nomic-embed-text",
		SystemPromptVersion: "support-v1",
	}

	if base.Key() == (Scope{GenerationModel: base.GenerationModel, EmbeddingModel: base.EmbeddingModel, SystemPromptVersion: "support-v2"}).Key() {
		t.Fatal("system-prompt change must create a distinct cache scope")
	}
	if base.Key() == (Scope{GenerationModel: "llama3.3", EmbeddingModel: base.EmbeddingModel, SystemPromptVersion: base.SystemPromptVersion}).Key() {
		t.Fatal("generation-model change must create a distinct cache scope")
	}
	if base.Key() == (Scope{GenerationModel: base.GenerationModel, EmbeddingModel: "other-embedder", SystemPromptVersion: base.SystemPromptVersion}).Key() {
		t.Fatal("embedding-model change must create a distinct cache scope")
	}
}

func TestScopeKeyIsPostgreSQLSafeText(t *testing.T) {
	key := (Scope{
		GenerationModel:     "llama3.2:1b",
		EmbeddingModel:      "nomic-embed-text",
		SystemPromptVersion: "none",
	}).Key()

	if strings.ContainsRune(key, '\x00') {
		t.Fatalf("Scope.Key() contains a NUL byte: %q", key)
	}
	if got, want := key, "generation=11:llama3.2:1b|embedding=16:nomic-embed-text|system-prompt=4:none"; got != want {
		t.Errorf("Scope.Key() = %q, want %q", got, want)
	}
}
