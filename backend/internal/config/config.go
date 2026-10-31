// Package config loads environment configuration. Secrets never appear in
// errors or logs.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the backend's runtime configuration.
type Config struct {
	DatabaseURL         string
	SupabaseURL         string
	SupabaseJWKSURL     string
	R2AccountID         string
	R2AccessKeyID       string
	R2SecretAccessKey   string
	R2Bucket            string
	LLMAPIKey           string
	LLMBaseURL          string
	LLMModel            string
	MaxParallelCompiles int
	Port                string
}

// Load reads the environment. For local dev it first loads `.env` from the
// working directory (backend/.env when run via `go run ./cmd/server` from
// backend/) — real environment variables always win, so production is
// unaffected. In dev (no DATABASE_URL), the server still boots; DB-backed
// handlers return 503. Missing values are reported by name only.
func Load() Config {
	// .env never overrides real environment variables (prod-safe).
	_ = godotenv.Load(".env")
	cfg := Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		SupabaseURL:       os.Getenv("SUPABASE_URL"),
		SupabaseJWKSURL:   os.Getenv("SUPABASE_JWKS_URL"),
		R2AccountID:       os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:          os.Getenv("R2_BUCKET"),
		LLMAPIKey:        os.Getenv("LLM_API_KEY"),
		LLMBaseURL:       os.Getenv("LLM_BASE_URL"),
		LLMModel:         os.Getenv("LLM_MODEL"),
		Port:              os.Getenv("PORT"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.LLMBaseURL == "" {
		cfg.LLMBaseURL = "https://api.groq.com/openai/v1"
	}
	if cfg.LLMModel == "" {
		cfg.LLMModel = "llama-3.3-70b-versatile"
	}
	cfg.MaxParallelCompiles = 4
	if v := os.Getenv("MAX_PARALLEL_COMPILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 32 {
			cfg.MaxParallelCompiles = n
		}
	}
	if cfg.SupabaseJWKSURL == "" && cfg.SupabaseURL != "" {
		cfg.SupabaseJWKSURL = strings.TrimSuffix(cfg.SupabaseURL, "/") + "/auth/v1/.well-known/jwks.json"
	}
	return cfg
}

// Missing returns the names of unset variables required for full operation.
func (c Config) Missing() []string {
	var missing []string
	for name, ok := range map[string]bool{
		"DATABASE_URL":         c.DatabaseURL != "",
		"SUPABASE_URL":         c.SupabaseURL != "",
		"SUPABASE_JWKS_URL":    c.SupabaseJWKSURL != "",
		"R2_ACCOUNT_ID":        c.R2AccountID != "",
		"R2_ACCESS_KEY_ID":     c.R2AccessKeyID != "",
		"R2_SECRET_ACCESS_KEY": c.R2SecretAccessKey != "",
		"R2_BUCKET":            c.R2Bucket != "",
		"LLM_API_KEY":          c.LLMAPIKey != "",
	} {
		if !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

var _ = fmt.Sprintf // reserved for formatted errors
