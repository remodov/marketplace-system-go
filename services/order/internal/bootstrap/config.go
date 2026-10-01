package bootstrap

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	CatalogBaseURL string
	KafkaBrokers   string
	OutboxInterval time.Duration
	AuthMode       string
	JWKSURL        string
	Issuer         string
	Audience       string
}

func FromEnv() Config {
	return Config{
		HTTPAddr:       env("HTTP_ADDR", ":8084"),
		DatabaseURL:    env("DATABASE_URL", "postgres://catalog:catalog@localhost:5440/orders?sslmode=disable"),
		CatalogBaseURL: env("CATALOG_BASE_URL", "http://localhost:8083"),
		KafkaBrokers:   env("KAFKA_BROKERS", "localhost:9094"),
		OutboxInterval: duration("OUTBOX_INTERVAL", time.Second),
		AuthMode:       env("AUTH_MODE", "local"),
		JWKSURL:        env("JWKS_URL", "http://localhost:8180/realms/marketplace/protocol/openid-connect/certs"),
		Issuer:         env("JWT_ISSUER", "http://localhost:8180/realms/marketplace"),
		Audience:       env("JWT_AUDIENCE", ""),
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func (c Config) Brokers() []string {
	if strings.TrimSpace(c.KafkaBrokers) == "" {
		return nil
	}
	return strings.Split(c.KafkaBrokers, ",")
}

func duration(name string, fallback time.Duration) time.Duration {
	if v := os.Getenv(name); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
