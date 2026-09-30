package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

func FromEnv() Config {
	return Config{
		HTTPAddr:    env("HTTP_ADDR", ":8082"),
		DatabaseURL: env("DATABASE_URL", "postgres://catalog:catalog@localhost:5440/catalog_starter?sslmode=disable"),
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
