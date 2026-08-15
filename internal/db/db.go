package db

import (
	"context"
	"database/sql"
	"fmt"

	"ai-proxy/internal/config"

	_ "github.com/lib/pq"
)

func NewPostgres(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DBConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	// Apply dynamic pool limits from configuration
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	// Apply dynamic ping timeout from configuration
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBPingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
