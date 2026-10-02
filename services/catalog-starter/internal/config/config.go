package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	CacheKind        string
	RedisAddr        string
	CacheTTL         time.Duration
	ServiceName      string
	OTLPEndpoint     string
	TraceSampleRatio float64
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

		ServiceName:      env("SERVICE_NAME", "catalog-starter"),
		OTLPEndpoint:     env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
		TraceSampleRatio: ratio(env("TRACE_SAMPLE_RATIO", "1.0")),
	}
}

func ratio(raw string) float64 {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 || value > 1 {
		return 1.0
	}
	return value
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
