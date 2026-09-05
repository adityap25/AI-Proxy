package cache

import "testing"

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
