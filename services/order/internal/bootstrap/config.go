package bootstrap

import "os"

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	CatalogBaseURL string
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
