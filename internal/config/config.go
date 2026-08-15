package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env        string
	Port       string
	DBConnStr  string
	OllamaHost string
	EmbedModel string
	LLMModel   string
	Similarity float64

	// Database Configs
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBPingTimeout     time.Duration
}

func Load() (*Config, error) {
	// Attempt to load .env file for local development (ignores error if file doesn't exist, e.g., in Docker or K8s)
	_ = godotenv.Load()

	env := getEnv("ENV", "development")

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbName := getEnv("DB_NAME", "aidb")
	dbPass := os.Getenv("DB_PASSWORD")

	// Fail fast in production if sensitive database credentials are missing
	if env == "production" && dbPass == "" {
		return nil, fmt.Errorf("FATAL: DB_PASSWORD environment variable is required in production mode")
	}

	// Local development fallback for database password
	if dbPass == "" {
		dbPass = "postgres"
	}

	connStr := os.Getenv("DB_CONN_STR")
	if connStr == "" {
		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPass, dbName)
	}

	cfg := &Config{
		Env:        env,
		Port:       getEnv("PORT", "8080"),
		DBConnStr:  connStr,
		OllamaHost: getEnv("OLLAMA_HOST", "http://localhost:11434"),
		EmbedModel: getEnv("EMBED_MODEL", "nomic-embed-text"),
		LLMModel:   getEnv("LLM_MODEL", "llama3.2:1b"),
		Similarity: getEnvAsFloat("SIMILARITY_THRESHOLD", 0.15),

		// DB Configs
		DBMaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		DBPingTimeout:     getEnvAsDuration("DB_PING_TIMEOUT", 5*time.Second),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func getEnvAsFloat(key string, defaultVal float64) float64 {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseFloat(valStr, 64); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := getEnv(key, "")
	if val, err := time.ParseDuration(valStr); err == nil {
		return val
	}
	return defaultVal
}
