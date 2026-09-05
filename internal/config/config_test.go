package config

import "testing"

func TestLoadSemanticCacheDefaults(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("CACHE_SIMILARITY_THRESHOLD", "")
	t.Setenv("CACHE_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.CacheSimilarity, 0.90; got != want {
		t.Errorf("CacheSimilarity = %v, want %v", got, want)
	}
	if got, want := cfg.SystemPromptVersion, "none"; got != want {
		t.Errorf("SystemPromptVersion = %q, want %q", got, want)
	}
}

func TestLoadRejectsInvalidCacheSimilarity(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("CACHE_SIMILARITY_THRESHOLD", "1.01")

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with an invalid cache similarity threshold")
	}
}

func TestLoadRejectsNonPositiveCacheTTL(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("CACHE_TTL", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with a non-positive cache TTL")
	}
}
