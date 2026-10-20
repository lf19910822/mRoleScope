package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds settings supplied by the process environment. Secrets are never
// committed and configuration files are deliberately not loaded implicitly.
type Config struct {
	Environment      string
	HTTPAddress      string
	DatabaseURL      string
	CollectorPython  string
	CollectorScript  string
	CollectorTimeout time.Duration
	ArchiveLocalDir  string
}

func Load() (Config, error) {
	timeout, err := durationFromEnv("COLLECTOR_TIMEOUT", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:      valueOrDefault("APP_ENV", "development"),
		HTTPAddress:      valueOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		CollectorPython:  valueOrDefault("COLLECTOR_PYTHON", "python3"),
		CollectorScript:  valueOrDefault("COLLECTOR_SCRIPT", "collector/mihoyo_collector.py"),
		CollectorTimeout: timeout,
		ArchiveLocalDir:  valueOrDefault("ARCHIVE_LOCAL_DIR", "./var/archives"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", key)
	}
	return parsed, nil
}
