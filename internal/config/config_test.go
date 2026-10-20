package config

import "testing"

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
