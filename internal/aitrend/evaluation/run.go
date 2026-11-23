package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// VersionInput contains every input that can change benchmark meaning. It is
// intentionally explicit so a report cannot silently compare incompatible
// prompts, tools, snapshots, or scorers.
type VersionInput struct {
	RunID             string `json:"run_id"`
	Model             string `json:"model"`
	Provider          string `json:"provider"`
	PromptVersion     string `json:"prompt_version"`
	ToolSchemaVersion string `json:"tool_schema_version"`
	SnapshotVersion   string `json:"snapshot_version"`
	CodeRevision      string `json:"code_revision"`
	CasesVersion      string `json:"cases_version"`
	ScorerVersion     string `json:"scorer_version"`
}

// EvaluationVersion is immutable after construction. The ID is a digest of
// the complete metadata, while CreatedAt is retained only for the report.
type EvaluationVersion struct {
	metadata  VersionInput
	id        string
	createdAt time.Time
}

func NewEvaluationVersion(input VersionInput, now time.Time) (EvaluationVersion, error) {
	if err := validateVersionInput(input); err != nil {
		return EvaluationVersion{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return EvaluationVersion{}, fmt.Errorf("marshal evaluation version: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return EvaluationVersion{metadata: input, id: "eval-" + hex.EncodeToString(digest[:8]), createdAt: now.UTC()}, nil
}

func validateVersionInput(input VersionInput) error {
	fields := []struct {
		name  string
		value string
	}{
		{"run_id", input.RunID}, {"model", input.Model}, {"provider", input.Provider},
		{"prompt_version", input.PromptVersion}, {"tool_schema_version", input.ToolSchemaVersion},
		{"snapshot_version", input.SnapshotVersion}, {"code_revision", input.CodeRevision},
		{"cases_version", input.CasesVersion}, {"scorer_version", input.ScorerVersion},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("evaluation version %s is empty", field.name)
		}
	}
	return nil
}

func (version EvaluationVersion) ID() string { return version.id }

func (version EvaluationVersion) Metadata() VersionInput { return version.metadata }

func (version EvaluationVersion) CreatedAt() time.Time { return version.createdAt }

// CaseRunStatus separates functional outcomes from infrastructure outcomes.
type CaseRunStatus string

const (
	CaseRunPassed             CaseRunStatus = "passed"
	CaseRunSemanticFailure    CaseRunStatus = "ai_semantic_failure"
	CaseRunInfrastructureFail CaseRunStatus = "infrastructure_failure"
	CaseRunIncomplete         CaseRunStatus = "incomplete"
	CaseRunLimitExceeded      CaseRunStatus = "tool_limit_exceeded"
)

type CaseRun struct {
	CaseID       string        `json:"case_id"`
	Repetition   int           `json:"repetition"`
	Status       CaseRunStatus `json:"status"`
	VersionID    string        `json:"evaluation_version_id"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	RetryCount   int           `json:"retry_count"`
	ToolCalls    int           `json:"tool_calls"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	CostNanoRMB  int64         `json:"cost_nano_rmb"`
}

func (run CaseRun) Validate() error {
	if strings.TrimSpace(run.CaseID) == "" || strings.TrimSpace(run.VersionID) == "" || run.Repetition < 1 || run.Repetition > 3 {
		return fmt.Errorf("invalid evaluation case run identity")
	}
	switch run.Status {
	case CaseRunPassed, CaseRunSemanticFailure, CaseRunInfrastructureFail, CaseRunIncomplete, CaseRunLimitExceeded:
	default:
		return fmt.Errorf("invalid evaluation case run status %q", run.Status)
	}
	if run.RetryCount < 0 || run.RetryCount > 2 || run.ToolCalls < 0 || run.ToolCalls > 20 || run.InputTokens < 0 || run.OutputTokens < 0 || run.CostNanoRMB < 0 {
		return fmt.Errorf("evaluation case run exceeds bounded limits")
	}
	return nil
}
