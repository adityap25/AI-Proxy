package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidEmbedding = errors.New("embedding must contain only finite values")

// Repository persists semantic cache entries in PostgreSQL with pgvector.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a semantic-cache repository backed by db.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Entry represents a row in prompt_cache.
type Entry struct {
	ID             int64
	Prompt         string
	Response       string
	Embedding      []float32
	Scope          string
	CreatedAt      time.Time
	ExpiresAt      time.Time
	LastAccessedAt *time.Time
	HitCount       int64
}

// Match is the nearest eligible cache entry returned by FindClosest.
type Match struct {
	Entry
	CacheHit   bool
	Similarity float64
}

// FindClosest returns the most similar, unexpired entry in scope whose cosine
// similarity is at least minSimilarity. Pgvector's cosine operator (<=>)
// returns distance, hence the conversion to 1 - minSimilarity.
func (r *Repository) FindClosest(ctx context.Context, embedding []float32, scope string, minSimilarity float64) (*Match, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("semantic cache database is unavailable")
	}
	if minSimilarity < 0 || minSimilarity > 1 {
		return nil, fmt.Errorf("minimum similarity must be between 0 and 1")
	}

	vector, err := vectorLiteral(embedding)
	if err != nil {
		return nil, err
	}

	const query = `
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
			entry.hit_count, closest.similarity`

	match := &Match{CacheHit: true}
	var lastAccessedAt sql.NullTime
	err = r.db.QueryRowContext(ctx, query, vector, scope, 1-minSimilarity).Scan(
		&match.ID,
		&match.Prompt,
		&match.Response,
		&match.Scope,
		&match.CreatedAt,
		&match.ExpiresAt,
		&lastAccessedAt,
		&match.HitCount,
		&match.Similarity,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find semantic cache entry: %w", err)
	}
	if lastAccessedAt.Valid {
		match.LastAccessedAt = &lastAccessedAt.Time
	}

	return match, nil
}

// Store saves an LLM response so compatible future prompts can reuse it.
func (r *Repository) Store(ctx context.Context, entry Entry) (*Entry, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("semantic cache database is unavailable")
	}
	if entry.Scope == "" {
		return nil, errors.New("cache scope is required")
	}
	if entry.ExpiresAt.IsZero() {
		return nil, errors.New("cache expiration is required")
	}

	vector, err := vectorLiteral(entry.Embedding)
	if err != nil {
		return nil, err
	}

	const query = `
		INSERT INTO prompt_cache (prompt, response, embedding, cache_scope, expires_at)
		VALUES ($1, $2, $3::vector, $4, $5)
		RETURNING id, created_at, expires_at, last_accessed_at, hit_count`

	stored := entry
	var lastAccessedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, entry.Prompt, entry.Response, vector, entry.Scope, entry.ExpiresAt).Scan(
		&stored.ID,
		&stored.CreatedAt,
		&stored.ExpiresAt,
		&lastAccessedAt,
		&stored.HitCount,
	); err != nil {
		return nil, fmt.Errorf("store semantic cache entry: %w", err)
	}
	if lastAccessedAt.Valid {
		stored.LastAccessedAt = &lastAccessedAt.Time
	}

	return &stored, nil
}

// vectorLiteral encodes values using pgvector's text input syntax. lib/pq does
// not natively encode vectors, and parameterization keeps the result separate
// from SQL syntax.
func vectorLiteral(embedding []float32) (string, error) {
	if len(embedding) == 0 {
		return "", ErrInvalidEmbedding
	}

	values := make([]string, len(embedding))
	for i, value := range embedding {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return "", ErrInvalidEmbedding
		}
		values[i] = strconv.FormatFloat(float64(value), 'f', -1, 32)
	}

	return "[" + strings.Join(values, ",") + "]", nil
}
