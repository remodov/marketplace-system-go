package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	KafkaBrokers []string
	KafkaGroup   string
	KafkaTopic   string
	AdminToken   string
}

func FromEnv() Config {
	return Config{
		HTTPAddr:     env("HTTP_ADDR", ":8085"),
		DatabaseURL:  env("DATABASE_URL", "postgres://catalog:catalog@localhost:5440/notifications?sslmode=disable"),
		KafkaBrokers: strings.Split(env("KAFKA_BROKERS", "localhost:9094"), ","),
		KafkaGroup:   env("KAFKA_GROUP", "notification"),
		KafkaTopic:   env("KAFKA_TOPIC", "marketplace.orders.v1"),
		AdminToken:   env("ADMIN_TOKEN", "admin"),
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
