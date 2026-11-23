package evaluation

import (
	"testing"
	"time"
)

func completeVersionInput() VersionInput {
	return VersionInput{
		RunID: "run-001", Model: "qwen-test", Provider: "qwen",
		PromptVersion: "intent-v1", ToolSchemaVersion: "tools-v1",
		SnapshotVersion: "serving-evaluation-snapshot-v1", CodeRevision: "abc123",
		CasesVersion: BenchmarkVersion, ScorerVersion: "scorer-v1",
	}
}

func TestEvaluationVersionIsStableForCompleteMetadata(t *testing.T) {
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	first, err := NewEvaluationVersion(completeVersionInput(), now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewEvaluationVersion(completeVersionInput(), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID() != second.ID() {
		t.Fatalf("same metadata produced different IDs: %q %q", first.ID(), second.ID())
	}
	if !first.CreatedAt().Equal(now) {
		t.Fatalf("created at = %s, want %s", first.CreatedAt(), now)
	}
}

func TestEvaluationVersionRejectsMissingMetadata(t *testing.T) {
	input := completeVersionInput()
	input.ToolSchemaVersion = ""
	if _, err := NewEvaluationVersion(input, time.Now()); err == nil {
		t.Fatal("missing tool schema version unexpectedly accepted")
	}
}

func TestCaseRunValidatesBoundedOutcomes(t *testing.T) {
	run := CaseRun{CaseID: "E01", Repetition: 1, Status: CaseRunPassed, VersionID: "eval-001", ToolCalls: 3}
	if err := run.Validate(); err != nil {
		t.Fatal(err)
	}
	run.ToolCalls = 21
	if err := run.Validate(); err == nil {
		t.Fatal("tool call limit was not enforced")
	}
}
