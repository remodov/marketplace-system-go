package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	CacheKind   string
	RedisAddr   string
	CacheTTL    time.Duration
}

func FromEnv() Config {
	ttl, err := time.ParseDuration(env("CACHE_TTL", "10m"))
	if err != nil {
		ttl = 10 * time.Minute
	}
	return Config{
		HTTPAddr:    env("HTTP_ADDR", ":8082"),
		DatabaseURL: env("DATABASE_URL", "postgres://catalog:catalog@localhost:5440/catalog_starter?sslmode=disable"),
		CacheKind:   env("CACHE", "redis"),
		RedisAddr:   env("REDIS_ADDR", "localhost:6381"),
		CacheTTL:    ttl,
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
