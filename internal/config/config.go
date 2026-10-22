package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
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
	COS              COSConfig
	SyncRetryDelays  []time.Duration
	SyncTempRoot     string
	SyncTimeout      time.Duration
}

// COSConfig contains only runtime-supplied values. It is deliberately not
// loaded from a project file, and String methods are avoided so a secret is
// never accidentally written to a log.
type COSConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
}

var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

const maxSyncTimeout = 45 * time.Minute

func Load() (Config, error) {
	timeout, err := durationFromEnv("COLLECTOR_TIMEOUT", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	retryDelays, err := durationsFromEnv("SYNC_RETRY_DELAYS", []time.Duration{15 * time.Minute, 45 * time.Minute})
	if err != nil {
		return Config{}, err
	}
	syncTimeout, err := durationFromEnv("SYNC_TIMEOUT", 45*time.Minute)
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
		COS: COSConfig{
			Endpoint:        os.Getenv("COS_ENDPOINT"),
			Region:          os.Getenv("COS_REGION"),
			Bucket:          os.Getenv("COS_BUCKET"),
			Prefix:          valueOrDefault("COS_PREFIX", "mrolescope"),
			AccessKeyID:     os.Getenv("COS_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("COS_SECRET_ACCESS_KEY"),
		},
		SyncRetryDelays: retryDelays,
		SyncTempRoot:    valueOrDefault("SYNC_TEMP_ROOT", "/run/mrolescope-sync"),
		SyncTimeout:     syncTimeout,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

// ValidateSync performs the stricter checks that only the worker needs. The
// long-running API can start without COS credentials while synchronization is
// intentionally not configured.
func (cfg Config) ValidateSync() error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.COS.Endpoint == "" || cfg.COS.Region == "" || cfg.COS.Bucket == "" || cfg.COS.AccessKeyID == "" || cfg.COS.SecretAccessKey == "" {
		return fmt.Errorf("COS_ENDPOINT, COS_REGION, COS_BUCKET, COS_ACCESS_KEY_ID, and COS_SECRET_ACCESS_KEY are required for synchronization")
	}
	endpoint, err := url.Parse(cfg.COS.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return fmt.Errorf("COS_ENDPOINT must be a bare HTTPS endpoint without credentials, path, query, or fragment")
	}
	if !bucketPattern.MatchString(cfg.COS.Bucket) {
		return fmt.Errorf("COS_BUCKET is invalid")
	}
	prefix := strings.Trim(cfg.COS.Prefix, "/")
	if prefix == "" || prefix == "." || strings.HasPrefix(prefix, "../") || strings.Contains(prefix, "/../") || path.Clean(prefix) != prefix {
		return fmt.Errorf("COS_PREFIX must be a normalized non-empty object prefix")
	}
	if len(cfg.SyncRetryDelays) != 2 {
		return fmt.Errorf("SYNC_RETRY_DELAYS must contain exactly two retry delays")
	}
	for _, delay := range cfg.SyncRetryDelays {
		if delay <= 0 {
			return fmt.Errorf("SYNC_RETRY_DELAYS values must be positive")
		}
	}
	if !filepath.IsAbs(cfg.SyncTempRoot) || filepath.Clean(cfg.SyncTempRoot) != cfg.SyncTempRoot || cfg.SyncTempRoot == "/" {
		return fmt.Errorf("SYNC_TEMP_ROOT must be a normalized, non-root absolute path")
	}
	if cfg.SyncTimeout <= 0 || cfg.SyncTimeout > maxSyncTimeout {
		return fmt.Errorf("SYNC_TIMEOUT must be positive and no more than %s", maxSyncTimeout)
	}
	return nil
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

func durationsFromEnv(key string, fallback []time.Duration) ([]time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return append([]time.Duration(nil), fallback...), nil
	}
	parts := strings.Split(value, ",")
	durations := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		parsed, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil || parsed <= 0 {
			return nil, fmt.Errorf("%s must be a comma-separated list of positive Go durations", key)
		}
		durations = append(durations, parsed)
	}
	return durations, nil
}
