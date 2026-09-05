package cache

import (
	"context"
	"errors"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestVectorLiteral(t *testing.T) {
	got, err := vectorLiteral([]float32{0.25, -1, 0})
	if err != nil {
		t.Fatalf("vectorLiteral() error = %v", err)
	}
	if want := "[0.25,-1,0]"; got != want {
		t.Errorf("vectorLiteral() = %q, want %q", got, want)
	}
}

func TestVectorLiteralRejectsInvalidValues(t *testing.T) {
	for _, embedding := range [][]float32{
		nil,
		{float32(math.NaN())},
		{float32(math.Inf(1))},
	} {
		_, err := vectorLiteral(embedding)
		if !errors.Is(err, ErrInvalidEmbedding) {
			t.Errorf("vectorLiteral(%v) error = %v, want ErrInvalidEmbedding", embedding, err)
		}
	}
}

func TestStorePersistsEntryAndReturnsDatabaseMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	expiresAt := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	entry := Entry{
		Prompt:    "How do I restart a Kubernetes pod?",
		Response:  "Use kubectl delete pod ...",
		Embedding: []float32{0.25, -0.5},
		Scope:     "generation=llama\x00embedding=nomic\x00system-prompt=none",
		ExpiresAt: expiresAt,
	}

	mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO prompt_cache (prompt, response, embedding, cache_scope, expires_at)
		VALUES ($1, $2, $3::vector, $4, $5)
		RETURNING id, created_at, expires_at, last_accessed_at, hit_count`)).
		WithArgs(entry.Prompt, entry.Response, "[0.25,-0.5]", entry.Scope, entry.ExpiresAt).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "expires_at", "last_accessed_at", "hit_count"}).
			AddRow(42, createdAt, expiresAt, nil, 0))

	stored, err := NewRepository(db).Store(context.Background(), entry)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if got, want := stored.ID, int64(42); got != want {
		t.Errorf("stored ID = %d, want %d", got, want)
	}
	if got, want := stored.CreatedAt, createdAt; !got.Equal(want) {
		t.Errorf("stored CreatedAt = %v, want %v", got, want)
	}
	if stored.LastAccessedAt != nil {
		t.Errorf("stored LastAccessedAt = %v, want nil", stored.LastAccessedAt)
	}
	if got, want := stored.HitCount, int64(0); got != want {
		t.Errorf("stored HitCount = %d, want %d", got, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestFindClosestRecordsCacheHit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	accessedAt := time.Date(2026, 9, 5, 13, 0, 0, 0, time.UTC)
	scope := "generation=llama\x00embedding=nomic\x00system-prompt=none"
	maxDistance := 1 - float64(0.90)

	mock.ExpectQuery(regexp.QuoteMeta(`
		WITH closest AS (
			SELECT id, 1 - (embedding <=> $1::vector) AS similarity
			FROM prompt_cache
			WHERE cache_scope = $2
			  AND expires_at > CURRENT_TIMESTAMP
			  AND embedding <=> $1::vector <= $3
			ORDER BY embedding <=> $1::vector
			LIMIT 1
		)
		UPDATE prompt_cache AS entry
		SET hit_count = entry.hit_count + 1,
			last_accessed_at = CURRENT_TIMESTAMP
		FROM closest
		WHERE entry.id = closest.id
		RETURNING entry.id, entry.prompt, entry.response, entry.cache_scope,
			entry.created_at, entry.expires_at, entry.last_accessed_at,
			entry.hit_count, closest.similarity`)).
		WithArgs("[0.25,-0.5]", scope, maxDistance).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "prompt", "response", "cache_scope", "created_at", "expires_at", "last_accessed_at", "hit_count", "similarity",
		}).AddRow(42, "restart pod", "kubectl delete pod", scope, createdAt, expiresAt, accessedAt, 3, 0.94))

	match, err := NewRepository(db).FindClosest(context.Background(), []float32{0.25, -0.5}, scope, 0.90)
	if err != nil {
		t.Fatalf("FindClosest() error = %v", err)
	}
	if match == nil {
		t.Fatal("FindClosest() returned no match")
	}
	if !match.CacheHit {
		t.Error("CacheHit = false, want true")
	}
	if got, want := match.HitCount, int64(3); got != want {
		t.Errorf("HitCount = %d, want %d", got, want)
	}
	if match.LastAccessedAt == nil || !match.LastAccessedAt.Equal(accessedAt) {
		t.Errorf("LastAccessedAt = %v, want %v", match.LastAccessedAt, accessedAt)
	}
	if got, want := match.Similarity, 0.94; got != want {
		t.Errorf("Similarity = %v, want %v", got, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestFindClosestReturnsNilForNoMatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	maxDistance := 1 - float64(0.90)
	mock.ExpectQuery("WITH closest AS").
		WithArgs("[0.25]", "scope", maxDistance).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	match, err := NewRepository(db).FindClosest(context.Background(), []float32{0.25}, "scope", 0.90)
	if err != nil {
		t.Fatalf("FindClosest() error = %v", err)
	}
	if match != nil {
		t.Errorf("FindClosest() = %+v, want nil", match)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
