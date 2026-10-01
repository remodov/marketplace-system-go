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
	PaymentBaseURL string
	KafkaBrokers   string
	KafkaGroup     string
	OutboxInterval time.Duration
	ExpireAfter    time.Duration
	ExpireInterval time.Duration
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
		PaymentBaseURL: env("PAYMENT_BASE_URL", "http://localhost:8086"),
		KafkaBrokers:   env("KAFKA_BROKERS", "localhost:9094"),
		KafkaGroup:     env("KAFKA_GROUP", "order"),
		OutboxInterval: duration("OUTBOX_INTERVAL", time.Second),
		ExpireAfter:    duration("EXPIRE_UNPAID_AFTER", 15*time.Minute),
		ExpireInterval: duration("EXPIRE_INTERVAL", time.Minute),
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
