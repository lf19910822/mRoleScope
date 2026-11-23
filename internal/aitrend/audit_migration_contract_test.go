package aitrend

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTrendAuditMigrationStoresOnlyBoundedSummaries(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	migrationPath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "migrations", "000038_ai_trend_audit.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	schema := string(migration)
	for _, required := range []string{
		"CREATE TABLE ai_trend_request_audits",
		"CREATE TABLE ai_trend_request_audit_tools",
		"ai_trend_request_audits_expiry_idx",
		"expires_at",
		"tool_call_count",
		"draft_curve_count",
	} {
		if !strings.Contains(schema, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"raw_user_text",
		"prompt_text",
		"model_response",
		"jd_body",
		"connection_string",
		"password",
		"secret",
	} {
		if strings.Contains(strings.ToLower(schema), forbidden) {
			t.Fatalf("migration contains forbidden audit field %q", forbidden)
		}
	}
}

func TestAgentAuditMigrationAddsOnlyBoundedResponseSummary(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	migrationPath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "migrations", "000039_ai_trend_agent_audit.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	schema := string(migration)
	for _, required := range []string{"response_modes", "reply_warning_count BETWEEN 0 AND 3", "cardinality(response_modes) <= 2", "ARRAY['trend', 'guidance']"} {
		if !strings.Contains(schema, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"raw_user_text", "prompt_text", "model_response", "jd_body", "connection_string", "password", "secret"} {
		if strings.Contains(strings.ToLower(schema), forbidden) {
			t.Fatalf("migration contains forbidden audit field %q", forbidden)
		}
	}
}
