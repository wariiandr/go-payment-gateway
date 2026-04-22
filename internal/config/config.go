package config

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	KafkaBrokers         []string
	OTelEndpoint         string
	AuthorizeMaxAttempts int
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:                 os.Getenv("PORT"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		KafkaBrokers:         []string{os.Getenv("KAFKA_BROKERS")},
		OTelEndpoint:         os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		AuthorizeMaxAttempts: parsePositiveIntEnv("AUTHORIZE_MAX_ATTEMPTS", 3),
	}

	if cfg.OTelEndpoint == "" {
		cfg.OTelEndpoint = "jaeger:4318"
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	if cfg.KafkaBrokers[0] == "" {
		slog.Error("KAFKA_BROKERS is required")
		os.Exit(1)
	}

	return cfg
}

func parsePositiveIntEnv(name string, defaultValue int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultValue
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		slog.Warn("invalid env value, using default", "name", name, "value", raw, "default", defaultValue)
		return defaultValue
	}

	return v
}
