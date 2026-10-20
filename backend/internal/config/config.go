// Package config reads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime settings.
type Config struct {
	DatabaseURL    string
	JWTSecret      string
	Port           string
	CookieSecure   bool
	SessionTTL     time.Duration
	AutoMigrate    bool
	SeedOnStart    bool
	AllowSuperuser bool // dev escape hatch; RLS is NOT enforced for superusers
}

// Load reads configuration from the environment with sensible dev defaults.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:    getenv("DATABASE_URL", "postgres://gym:gym@localhost:5432/gym?sslmode=disable"),
		JWTSecret:      getenv("JWT_SECRET", "dev-secret-change-me"),
		Port:           getenv("PORT", "8080"),
		CookieSecure:   getbool("COOKIE_SECURE", false),
		SessionTTL:     getduration("SESSION_TTL", 7*24*time.Hour),
		AutoMigrate:    getbool("AUTO_MIGRATE", false),
		SeedOnStart:    getbool("SEED_ON_START", false),
		AllowSuperuser: getbool("ALLOW_SUPERUSER", false),
	}
	if len(cfg.JWTSecret) < 8 {
		return cfg, fmt.Errorf("JWT_SECRET must be at least 8 characters")
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getduration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
