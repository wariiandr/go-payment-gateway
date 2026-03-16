package config

import (
	"log"
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
		log.Fatalln("DATABASE_URL is required")
	}

	if cfg.KafkaBrokers[0] == "" {
		log.Fatalln("KAFKA_BROKERS is required")
	}

	return cfg
}
