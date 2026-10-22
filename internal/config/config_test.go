package config

import (
	"testing"
	"time"
)

func TestDurationFromEnvRejectsNonPositiveDuration(t *testing.T) {
	t.Setenv("COLLECTOR_TIMEOUT", "0s")
	if _, err := durationFromEnv("COLLECTOR_TIMEOUT", 1); err == nil {
		t.Fatal("expected a validation error")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected a missing DATABASE_URL error")
	}
}

func TestValidateSyncRejectsUnsafeCOSConfiguration(t *testing.T) {
	cfg := Config{
		DatabaseURL: "postgres://example",
		COS: COSConfig{
			Endpoint: "http://not-https.example.com", Region: "ap-shanghai", Bucket: "valid-bucket", Prefix: "mrolescope",
			AccessKeyID: "id", SecretAccessKey: "secret",
		},
		SyncRetryDelays: []time.Duration{time.Minute, 2 * time.Minute},
		SyncTempRoot:    "/run/mrolescope-sync",
		SyncTimeout:     30 * time.Minute,
	}
	if err := cfg.ValidateSync(); err == nil {
		t.Fatal("expected unsafe COS endpoint to be rejected")
	}
}

func TestValidateSyncRejectsCOSPath(t *testing.T) {
	cfg := Config{
		DatabaseURL: "postgres://example",
		COS: COSConfig{
			Endpoint: "https://cos.ap-shanghai.myqcloud.com/not-a-service-endpoint", Region: "ap-shanghai", Bucket: "valid-bucket", Prefix: "mrolescope",
			AccessKeyID: "id", SecretAccessKey: "secret",
		},
		SyncRetryDelays: []time.Duration{time.Minute, 2 * time.Minute},
		SyncTempRoot:    "/run/mrolescope-sync",
		SyncTimeout:     30 * time.Minute,
	}
	if err := cfg.ValidateSync(); err == nil {
		t.Fatal("expected a COS endpoint path to be rejected")
	}
}

func TestValidateSyncAcceptsRootOnlyRuntimeValues(t *testing.T) {
	cfg := Config{
		DatabaseURL: "postgres://example",
		COS: COSConfig{
			Endpoint: "https://cos.ap-shanghai.myqcloud.com", Region: "ap-shanghai", Bucket: "mrolescope-1234567890", Prefix: "raw/mihoyo",
			AccessKeyID: "id", SecretAccessKey: "secret",
		},
		SyncRetryDelays: []time.Duration{time.Minute, 2 * time.Minute},
		SyncTempRoot:    "/run/mrolescope-sync",
		SyncTimeout:     30 * time.Minute,
	}
	if err := cfg.ValidateSync(); err != nil {
		t.Fatalf("ValidateSync() error = %v", err)
	}
}

func TestValidateSyncBoundsWorkerTimeout(t *testing.T) {
	cfg := Config{
		DatabaseURL: "postgres://example",
		COS: COSConfig{
			Endpoint: "https://cos.ap-shanghai.myqcloud.com", Region: "ap-shanghai", Bucket: "mrolescope-1234567890", Prefix: "raw/mihoyo",
			AccessKeyID: "id", SecretAccessKey: "secret",
		},
		SyncRetryDelays: []time.Duration{time.Minute, 2 * time.Minute},
		SyncTempRoot:    "/run/mrolescope-sync",
		SyncTimeout:     maxSyncTimeout + time.Second,
	}
	if err := cfg.ValidateSync(); err == nil {
		t.Fatal("expected an excessive worker timeout to be rejected")
	}
}
