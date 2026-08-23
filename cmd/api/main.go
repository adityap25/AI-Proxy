package main

import (
	"fmt"
	"log"

	"ai-proxy/internal/config"
	"ai-proxy/internal/db"
	"ai-proxy/internal/ollama"
	"ai-proxy/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	postgres, err := db.NewPostgres(cfg)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer func() {
		if closeErr := postgres.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close database: %w", closeErr)
		}
	}()

	model := ollama.New(cfg.OllamaHost, cfg.LLMModel)
	return server.NewServer(cfg, postgres, model).Start()
}
