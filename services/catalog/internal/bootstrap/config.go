package bootstrap

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	AuthMode        string
	JWKSURL         string
	Issuer          string
	Audience        string
	ImagesEndpoint  string
	ImagesRegion    string
	ImagesAccessKey string
	ImagesSecretKey string
	ImagesBucket    string
	ImagesUploadTTL time.Duration
}

func FromEnv() Config {
	return Config{
		HTTPAddr:    env("HTTP_ADDR", ":8083"),
		DatabaseURL: env("DATABASE_URL", "postgres://catalog:catalog@localhost:5440/catalog?sslmode=disable"),
		AuthMode:    env("AUTH_MODE", "local"),
		JWKSURL:     env("JWKS_URL", "http://localhost:8180/realms/marketplace/protocol/openid-connect/certs"),
		Issuer:      env("JWT_ISSUER", "http://localhost:8180/realms/marketplace"),
		Audience:    env("JWT_AUDIENCE", ""),

		ImagesEndpoint:  env("IMAGES_ENDPOINT", "http://localhost:9000"),
		ImagesRegion:    env("IMAGES_REGION", "us-east-1"),
		ImagesAccessKey: env("IMAGES_ACCESS_KEY", "marketplace"),
		ImagesSecretKey: env("IMAGES_SECRET_KEY", "marketplace"),
		ImagesBucket:    env("IMAGES_BUCKET", "marketplace-images"),
		ImagesUploadTTL: duration("IMAGES_UPLOAD_TTL", 10*time.Minute),
	}
}

func duration(name string, fallback time.Duration) time.Duration {
	if v := os.Getenv(name); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
