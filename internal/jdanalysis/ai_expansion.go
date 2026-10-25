package jdanalysis

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AIExpansionSummary exposes only a fixed candidate identity and safe queue
// counts. It never returns body IDs, text, semantic values, prompts, provider
// payloads, or credentials.
type AIExpansionSummary struct {
	AIConfigurationID          string
	ExistingExpansionWorkCount int
	Queued                     int
}

// ExpandAIPlusCandidate queues the remaining eligible body versions only for
// the same Plus candidate that passed its fully reviewed, error-free trial.
// It has no caller-supplied body list, model, contract, pricing, query, or
// execution parameter.
func (repository *Repository) ExpandAIPlusCandidate(ctx context.Context, configurationID string) (AIExpansionSummary, error) {
	if configurationID == "" {
		return AIExpansionSummary{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AIExpansionSummary{}, fmt.Errorf("begin AI Plus expansion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	summary := AIExpansionSummary{AIConfigurationID: configurationID}
	var model, status, technologyConfigurationID string
	err = tx.QueryRow(ctx, `
SELECT model, status::text, technology_analysis_configuration_id::text
FROM ai_analysis_configurations
WHERE id = $1
FOR UPDATE`, configurationID).Scan(&model, &status, &technologyConfigurationID)
	if err == pgx.ErrNoRows {
		return AIExpansionSummary{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if err != nil {
		return AIExpansionSummary{}, fmt.Errorf("lock AI candidate for expansion: %w", err)
	}
	if model != "qwen-plus" || status != "candidate" {
		return AIExpansionSummary{}, ErrAIReleaseGateFailed
	}
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1 AND trigger = 'expansion'`, configurationID).Scan(&summary.ExistingExpansionWorkCount); err != nil {
		return AIExpansionSummary{}, fmt.Errorf("count existing AI expansion work: %w", err)
	}
	if summary.ExistingExpansionWorkCount > 0 {
		return summary, nil
	}

	gates, err := readAIReleaseGateCounts(ctx, tx, configurationID)
	if err != nil {
		return AIExpansionSummary{}, err
	}
	if !gates.allowsTrialExpansion() {
		return summary, ErrAIReleaseGateFailed
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_semantic_results (body_content_version_id, ai_analysis_configuration_id)
SELECT DISTINCT deterministic_result.body_content_version_id, $1::uuid
FROM technology_analysis_results AS deterministic_result
WHERE deterministic_result.configuration_id = $2
  AND deterministic_result.status = 'complete'
ON CONFLICT (body_content_version_id, ai_analysis_configuration_id) DO NOTHING`, configurationID, technologyConfigurationID); err != nil {
		return AIExpansionSummary{}, fmt.Errorf("create AI expansion result slots: %w", err)
	}
	command, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_work_items (
    ai_analysis_configuration_id, body_content_version_id, trigger
)
SELECT $1::uuid, deterministic_result.body_content_version_id, 'expansion'
FROM technology_analysis_results AS deterministic_result
WHERE deterministic_result.configuration_id = $2
  AND deterministic_result.status = 'complete'
ON CONFLICT (body_content_version_id, ai_analysis_configuration_id) DO NOTHING`, configurationID, technologyConfigurationID)
	if err != nil {
		return AIExpansionSummary{}, fmt.Errorf("queue AI expansion work: %w", err)
	}
	summary.Queued = int(command.RowsAffected())
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_audit_events (
    ai_analysis_configuration_id, severity, code, logical_task_count, attempt_count,
    input_token_count, output_token_count, charge_rmb
) VALUES ($1, 'info', 'work_queued', $2, 0, 0, 0, 0)`, configurationID, summary.Queued); err != nil {
		return AIExpansionSummary{}, fmt.Errorf("write AI expansion audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AIExpansionSummary{}, fmt.Errorf("commit AI Plus expansion: %w", err)
	}
	return summary, nil
}

func (gates aiReleaseGateCounts) allowsTrialExpansion() bool {
	return gates.TrialBodyCount == aiTrialSampleSize &&
		gates.FullyReviewedTrialBodyCount == aiTrialSampleSize &&
		gates.MajorErrorCount == 0 &&
		gates.AllWorkCount == gates.TrialBodyCount &&
		gates.CompleteWorkCount == gates.AllWorkCount
}
