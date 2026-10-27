package jdanalysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	AIReleaseGateVersion = "ai-semantic-release-gate-v2"
	AIReleasePublisher   = "FAN"
)

var ErrAIReleaseGateFailed = errors.New("AI semantic release gate failed")

// AIReleaseSummary contains only the immutable candidate identity and safe
// gate counts. It deliberately excludes JD text, semantic values, prompts,
// provider payloads, and credentials.
type AIReleaseSummary struct {
	AIConfigurationID      string
	Publisher              string
	TrialBodyCount         int
	TotalWorkCount         int
	TerminalWorkCount      int
	StructuralSuccessCount int
	SafeFailureCount       int
	AttemptCount           int
	InputTokenCount        int64
	OutputTokenCount       int64
	ChargeNanoRMB          int64
}

// PublishAIConfiguration advances one fixed qwen-plus candidate through the
// manual, mechanical release boundary. It never judges semantic correctness,
// review coverage, review tiers, or review verdicts. Publication has no
// caller-controlled model, prompt, query, body list, pricing or publisher.
func (repository *Repository) PublishAIConfiguration(ctx context.Context, configurationID string) (AIReleaseSummary, error) {
	if configurationID == "" {
		return AIReleaseSummary{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AIReleaseSummary{}, fmt.Errorf("begin AI configuration publication: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var model, status string
	err = tx.QueryRow(ctx, `
SELECT model, status::text
FROM ai_analysis_configurations
WHERE id = $1
FOR UPDATE`, configurationID).Scan(&model, &status)
	if err == pgx.ErrNoRows {
		return AIReleaseSummary{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if err != nil {
		return AIReleaseSummary{}, fmt.Errorf("lock AI configuration for publication: %w", err)
	}
	if model != "qwen-plus" || status != "candidate" {
		return AIReleaseSummary{}, ErrAIReleaseGateFailed
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_publication_state (singleton)
VALUES (true)
ON CONFLICT (singleton) DO NOTHING`); err != nil {
		return AIReleaseSummary{}, fmt.Errorf("ensure AI publication state: %w", err)
	}

	summary := AIReleaseSummary{AIConfigurationID: configurationID, Publisher: AIReleasePublisher}
	gates, err := readAIReleaseGateCounts(ctx, tx, configurationID)
	if err != nil {
		return AIReleaseSummary{}, err
	}
	summary.TrialBodyCount = gates.TrialBodyCount
	summary.TotalWorkCount = gates.AllWorkCount
	summary.TerminalWorkCount = gates.TerminalWorkCount
	summary.StructuralSuccessCount = gates.StructuralSuccessCount
	summary.SafeFailureCount = gates.SafeFailureCount
	summary.AttemptCount = gates.AttemptCount
	summary.InputTokenCount = gates.InputTokenCount
	summary.OutputTokenCount = gates.OutputTokenCount
	summary.ChargeNanoRMB = gates.ChargeNanoRMB
	if !gates.allowsPublication() {
		return summary, ErrAIReleaseGateFailed
	}

	var currentDefaultConfigurationID *string
	if err := tx.QueryRow(ctx, `
SELECT ai_analysis_configuration_id::text
FROM ai_analysis_publication_state
WHERE singleton = true
FOR UPDATE`).Scan(&currentDefaultConfigurationID); err != nil {
		return AIReleaseSummary{}, fmt.Errorf("lock AI publication state: %w", err)
	}
	if currentDefaultConfigurationID != nil {
		return summary, ErrAIReleaseGateFailed
	}

	attestationSHA256 := aiReleaseAttestationSHA256(summary, gates)
	command, err := tx.Exec(ctx, `
UPDATE ai_analysis_configurations
SET status = 'published', published_at = now(), failed_at = NULL, retired_at = NULL, safe_failure_code = NULL
WHERE id = $1 AND status = 'candidate' AND model = 'qwen-plus'`, configurationID)
	if err != nil || command.RowsAffected() != 1 {
		return AIReleaseSummary{}, fmt.Errorf("publish AI configuration: %w", err)
	}
	command, err = tx.Exec(ctx, `
UPDATE ai_analysis_publication_state
SET ai_analysis_configuration_id = $1, updated_at = now()
WHERE singleton = true`, configurationID)
	if err != nil || command.RowsAffected() != 1 {
		return AIReleaseSummary{}, fmt.Errorf("advance AI publication state: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_release_attestations (
    ai_analysis_configuration_id, gate_version, publisher, attestation_sha256,
    trial_body_count, fully_reviewed_trial_body_count, major_error_count,
    total_work_count, terminal_work_count, structural_success_count,
    safe_failure_count, attempt_count, input_token_count, output_token_count,
    charge_nano_rmb
) VALUES ($1, $2, $3, $4, $5, 0, 0, $6, $7, $8, $9, $10, $11, $12, $13)`,
		configurationID, AIReleaseGateVersion, AIReleasePublisher, attestationSHA256,
		summary.TrialBodyCount, summary.TotalWorkCount, summary.TerminalWorkCount,
		summary.StructuralSuccessCount, summary.SafeFailureCount, summary.AttemptCount,
		summary.InputTokenCount, summary.OutputTokenCount, summary.ChargeNanoRMB); err != nil {
		return AIReleaseSummary{}, fmt.Errorf("write AI release attestation: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_audit_events (
    ai_analysis_configuration_id, severity, code, logical_task_count, attempt_count,
    input_token_count, output_token_count, charge_rmb
) VALUES ($1, 'info', 'configuration_published', $2, 0, 0, 0, 0)`, configurationID, gates.AllWorkCount); err != nil {
		return AIReleaseSummary{}, fmt.Errorf("write AI publication audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AIReleaseSummary{}, fmt.Errorf("commit AI configuration publication: %w", err)
	}
	return summary, nil
}

type aiReleaseGateCounts struct {
	AllWorkCount           int
	TerminalWorkCount      int
	StructuralSuccessCount int
	SafeFailureCount       int
	TrialBodyCount         int
	AttemptCount           int
	InputTokenCount        int64
	OutputTokenCount       int64
	ChargeNanoRMB          int64
}

func readAIReleaseGateCounts(ctx context.Context, tx pgx.Tx, configurationID string) (aiReleaseGateCounts, error) {
	var gates aiReleaseGateCounts
	err := tx.QueryRow(ctx, `
WITH scoped_work AS (
    SELECT work.id, work.body_content_version_id, work.trigger, work.status AS work_status,
           result.id AS result_id, result.execution_status
    FROM ai_analysis_work_items AS work
    LEFT JOIN ai_semantic_results AS result
      ON result.ai_analysis_configuration_id = work.ai_analysis_configuration_id
     AND result.body_content_version_id = work.body_content_version_id
    WHERE work.ai_analysis_configuration_id = $1
)
SELECT
  (SELECT count(*) FROM scoped_work),
  (SELECT count(*) FROM scoped_work WHERE work_status IN ('complete', 'failed_safely')),
  (SELECT count(*) FROM scoped_work WHERE work_status = 'complete' AND execution_status = 'complete'),
  (SELECT count(*) FROM scoped_work WHERE work_status = 'failed_safely' AND execution_status = 'failed_safely'),
  (SELECT count(*) FROM scoped_work WHERE trigger = 'trial'),
  (SELECT count(*) FROM ai_analysis_attempts WHERE ai_analysis_configuration_id = $1),
  (SELECT COALESCE(sum(usage.input_tokens), 0)
   FROM ai_analysis_usage_records AS usage
   JOIN ai_analysis_attempts AS attempt ON attempt.id = usage.attempt_id
   WHERE attempt.ai_analysis_configuration_id = $1),
  (SELECT COALESCE(sum(usage.output_tokens), 0)
   FROM ai_analysis_usage_records AS usage
   JOIN ai_analysis_attempts AS attempt ON attempt.id = usage.attempt_id
   WHERE attempt.ai_analysis_configuration_id = $1),
  (SELECT COALESCE(sum((usage.actual_charge_rmb * 1000000000)::bigint), 0)
   FROM ai_analysis_usage_records AS usage
   JOIN ai_analysis_attempts AS attempt ON attempt.id = usage.attempt_id
   WHERE attempt.ai_analysis_configuration_id = $1)`, configurationID).Scan(
		&gates.AllWorkCount,
		&gates.TerminalWorkCount,
		&gates.StructuralSuccessCount,
		&gates.SafeFailureCount,
		&gates.TrialBodyCount,
		&gates.AttemptCount,
		&gates.InputTokenCount,
		&gates.OutputTokenCount,
		&gates.ChargeNanoRMB,
	)
	if err != nil {
		return aiReleaseGateCounts{}, fmt.Errorf("read AI publication gate counts: %w", err)
	}
	return gates, nil
}

func (gates aiReleaseGateCounts) allowsPublication() bool {
	return gates.AllWorkCount > 0 &&
		gates.TerminalWorkCount == gates.AllWorkCount &&
		gates.StructuralSuccessCount+gates.SafeFailureCount == gates.AllWorkCount
}

func aiReleaseAttestationSHA256(summary AIReleaseSummary, gates aiReleaseGateCounts) string {
	return sha256Hex(fmt.Sprintf(
		"%s|%s|%s|%d|%d|%d|%d|%d|%d|%d|%d|%d",
		AIReleaseGateVersion, summary.AIConfigurationID, summary.Publisher,
		summary.TotalWorkCount, summary.TerminalWorkCount, summary.TrialBodyCount,
		summary.StructuralSuccessCount, summary.SafeFailureCount,
		summary.AttemptCount, summary.InputTokenCount, summary.OutputTokenCount,
		summary.ChargeNanoRMB,
	))
}
