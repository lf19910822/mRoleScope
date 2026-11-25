package config

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config holds settings supplied by the process environment. Secrets are never
// committed and configuration files are deliberately not loaded implicitly.
type Config struct {
	Environment          string
	HTTPAddress          string
	DatabaseURL          string
	TrendAIAPIKey        string
	TrendAIWorkspaceID   string
	AIDebugEnabled       bool
	AIDebugMaxEvents     int
	AIDebugMaxBytes      int
	AIDebugTimeout       time.Duration
	LangfuseBaseURL      string
	LangfuseProjectID    string
	LangfusePublicKey    string
	LangfuseSecretKey    string
	InviteBcryptHash     string
	SessionSigningSecret string
	CollectorPython      string
	CollectorScript      string
	CollectorTimeout     time.Duration
	ArchiveLocalDir      string
	COS                  COSConfig
	SyncRetryDelays      []time.Duration
	SyncTempRoot         string
	SyncTimeout          time.Duration
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
var sessionSecretPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

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
		Environment:          valueOrDefault("APP_ENV", "development"),
		HTTPAddress:          valueOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		TrendAIAPIKey:        os.Getenv("MROLESCOPE_QWEN_API_KEY"),
		TrendAIWorkspaceID:   os.Getenv("MROLESCOPE_QWEN_WORKSPACE_ID"),
		AIDebugEnabled:       false,
		AIDebugMaxEvents:     256,
		AIDebugMaxBytes:      8 << 20,
		AIDebugTimeout:       2 * time.Minute,
		LangfuseBaseURL:      os.Getenv("MROLESCOPE_LANGFUSE_BASE_URL"),
		LangfuseProjectID:    os.Getenv("MROLESCOPE_LANGFUSE_PROJECT_ID"),
		LangfusePublicKey:    os.Getenv("MROLESCOPE_LANGFUSE_PUBLIC_KEY"),
		LangfuseSecretKey:    os.Getenv("MROLESCOPE_LANGFUSE_SECRET_KEY"),
		InviteBcryptHash:     os.Getenv("MROLESCOPE_INVITE_BCRYPT_HASH"),
		SessionSigningSecret: os.Getenv("MROLESCOPE_SESSION_SIGNING_SECRET"),
		CollectorPython:      valueOrDefault("COLLECTOR_PYTHON", "python3"),
		CollectorScript:      valueOrDefault("COLLECTOR_SCRIPT", "collector/mihoyo_collector.py"),
		CollectorTimeout:     timeout,
		ArchiveLocalDir:      valueOrDefault("ARCHIVE_LOCAL_DIR", "./var/archives"),
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
	debugEnabled, err := boolFromEnv("MROLESCOPE_AI_DEBUG_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	cfg.AIDebugEnabled = debugEnabled
	if cfg.AIDebugEnabled && strings.EqualFold(cfg.Environment, "production") {
		return Config{}, fmt.Errorf("MROLESCOPE_AI_DEBUG_ENABLED cannot be enabled in production")
	}
	if value := os.Getenv("MROLESCOPE_AI_DEBUG_MAX_EVENTS"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < 1 || parsed > 2048 {
			return Config{}, fmt.Errorf("MROLESCOPE_AI_DEBUG_MAX_EVENTS must be between 1 and 2048")
		}
		cfg.AIDebugMaxEvents = parsed
	}
	if value := os.Getenv("MROLESCOPE_AI_DEBUG_MAX_BYTES"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < 1024 || parsed > 64<<20 {
			return Config{}, fmt.Errorf("MROLESCOPE_AI_DEBUG_MAX_BYTES must be between 1024 and 67108864")
		}
		cfg.AIDebugMaxBytes = parsed
	}
	if timeout, timeoutErr := durationFromEnv("MROLESCOPE_AI_DEBUG_TIMEOUT", cfg.AIDebugTimeout); timeoutErr != nil {
		return Config{}, timeoutErr
	} else {
		cfg.AIDebugTimeout = timeout
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func boolFromEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return parsed, nil
}

func (cfg Config) ValidateAccess() error {
	if cfg.InviteBcryptHash == "" {
		return fmt.Errorf("MROLESCOPE_INVITE_BCRYPT_HASH is required")
	}
	if cfg.SessionSigningSecret == "" {
		return fmt.Errorf("MROLESCOPE_SESSION_SIGNING_SECRET is required")
	}
	if len(cfg.SessionSigningSecret) < 64 || !sessionSecretPattern.MatchString(cfg.SessionSigningSecret) {
		return fmt.Errorf("MROLESCOPE_SESSION_SIGNING_SECRET must contain at least 32 bytes encoded as hexadecimal")
	}
	if len(cfg.InviteBcryptHash) < 59 || !strings.HasPrefix(cfg.InviteBcryptHash, "$2") {
		return fmt.Errorf("MROLESCOPE_INVITE_BCRYPT_HASH must be a bcrypt hash")
	}
	return nil
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
