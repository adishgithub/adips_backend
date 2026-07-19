package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the service.
// Centralizing env access here means the rest of the codebase
// never calls os.Getenv directly, which makes config easy to
// validate, mock in tests, and extend (e.g. reading from a
// secrets manager instead of the environment later on).
type Config struct {
	Port          string
	Env           string // "development" | "production" | "test"
	DatabaseURL   string
	JWTSecret     string
	JWTExpiration time.Duration
}

var cfg *Config

// Load reads environment variables (and .env if present) into a
// validated Config. It panics on startup if a required variable
// is missing, since a misconfigured service should fail fast
// instead of serving traffic in a broken state.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using system environment variables")
	}

	c := &Config{
		Port:        getEnv("PORT", "8080"),
		Env:         getEnv("ENV", "development"),
		DatabaseURL: os.Getenv("DB"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}

	expHours := getEnvAsInt("JWT_EXPIRATION_HOURS", 24*30)
	c.JWTExpiration = time.Duration(expHours) * time.Hour

	if err := c.validate(); err != nil {
		log.Fatalf("❌ invalid configuration: %v", err)
	}

	cfg = c
	return c
}

// Get returns the already-loaded config. Panics if Load() was
// never called, which is intentional — every entrypoint should
// call Load() once at startup.
func Get() *Config {
	if cfg == nil {
		log.Fatal("❌ config accessed before Load() was called")
	}
	return cfg
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DB environment variable is required")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var out int
	if _, err := fmt.Sscanf(v, "%d", &out); err != nil {
		return fallback
	}
	return out
}
