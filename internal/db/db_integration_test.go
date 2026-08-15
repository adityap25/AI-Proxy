//go:build integration

package db

import (
	"testing"
	"time"

	"ai-proxy/internal/config"
)

func TestNewPostgres_Success(t *testing.T) {
	// Setup config pointing to a running test database container/instance
	cfg := &config.Config{
		DBConnStr:         "host=localhost port=5432 user=postgres password=postgres dbname=aidb_test sslmode=disable",
		DBMaxOpenConns:    5,
		DBMaxIdleConns:    2,
		DBConnMaxLifetime: 1 * time.Minute,
		DBPingTimeout:     2 * time.Second,
	}

	db, err := NewPostgres(cfg)
	if err != nil {
		t.Fatalf("expected successful database connection, got: %v", err)
	}
	defer db.Close()

	// Verify stats match configured limits
	stats := db.Stats()
	if stats.MaxOpenConnections != cfg.DBMaxOpenConns {
		t.Errorf("expected MaxOpenConns %d, got %d", cfg.DBMaxOpenConns, stats.MaxOpenConnections)
	}
}

func TestNewPostgres_ConnectionTimeout(t *testing.T) {
	cfg := &config.Config{
		DBConnStr:     "host=127.0.0.1 port=54321 user=postgres password=postgres dbname=aidb_test sslmode=disable connect_timeout=1",
		DBPingTimeout: 50 * time.Millisecond,
	}

	_, err := NewPostgres(cfg)
	if err == nil {
		t.Fatalf("expected connection timeout error, got nil")
	}
}
