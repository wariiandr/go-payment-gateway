package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	DatabaseURL  string
	KafkaBrokers []string
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:         os.Getenv("PORT"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		KafkaBrokers: []string{os.Getenv("KAFKA_BROKERS")},
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
