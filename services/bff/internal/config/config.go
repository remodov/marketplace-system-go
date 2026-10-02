package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr          string
	RedisAddr         string
	OrderURL          string
	CatalogURL        string
	PaymentURL        string
	RequestsPerMinute int
}

func FromEnv() Config {
	return Config{
		HTTPAddr:          env("HTTP_ADDR", ":8090"),
		RedisAddr:         env("REDIS_ADDR", "localhost:6381"),
		OrderURL:          env("ORDER_URL", "http://localhost:8084"),
		CatalogURL:        env("CATALOG_URL", "http://localhost:8083"),
		PaymentURL:        env("PAYMENT_URL", "http://localhost:8086"),
		RequestsPerMinute: envInt("RATE_LIMIT_PER_MINUTE", 60),
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func envInt(name string, fallback int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
