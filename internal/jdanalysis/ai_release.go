package jdanalysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	AIReleaseGateVersion = "ai-semantic-release-gate-v1"
	AIReleasePublisher   = "FAN"
)

var ErrAIReleaseGateFailed = errors.New("AI semantic release gate failed")

// AIReleaseSummary contains only the immutable candidate identity and safe
// gate counts. It deliberately excludes JD text, semantic values, prompts,
// provider payloads, and credentials.
type AIReleaseSummary struct {
	AIConfigurationID           string
	FullyReviewedTrialBodyCount int
	MajorErrorCount             int
	Publisher                   string
	TrialBodyCount              int
}

// PublishAIConfiguration advances one fixed qwen-plus candidate only after
// every release gate passes. Publication has no caller-controlled model,
// prompt, query, body list, pricing or publisher argument.
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
	summary.FullyReviewedTrialBodyCount = gates.FullyReviewedTrialBodyCount
	summary.MajorErrorCount = gates.MajorErrorCount
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
    trial_body_count, fully_reviewed_trial_body_count, major_error_count
) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		configurationID, AIReleaseGateVersion, AIReleasePublisher, attestationSHA256,
		summary.TrialBodyCount, summary.FullyReviewedTrialBodyCount, summary.MajorErrorCount); err != nil {
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
	AllWorkCount                   int
	CompleteWorkCount              int
	FullyReviewedTrialBodyCount    int
	MajorErrorCount                int
	RequiredReviewArtifactCount    int
	ReviewedRequiredArtifactCount  int
	ReviewedRoutineArtifactCount   int
	ReviewedSpotCheckArtifactCount int
	RoutineArtifactCount           int
	SpotCheckArtifactCount         int
	TrialBodyCount                 int
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
), trial_results AS (
    SELECT DISTINCT result_id AS id
    FROM scoped_work
    WHERE trigger = 'trial' AND result_id IS NOT NULL
), trial_artifacts AS (
    SELECT artifact.id, artifact.ai_semantic_result_id
    FROM ai_semantic_artifacts AS artifact
    JOIN trial_results ON trial_results.id = artifact.ai_semantic_result_id
), fully_reviewed_trial_results AS (
    SELECT trial_results.id
    FROM trial_results
    WHERE EXISTS (
        SELECT 1 FROM trial_artifacts AS artifact
        WHERE artifact.ai_semantic_result_id = trial_results.id
    )
      AND NOT EXISTS (
        SELECT 1
        FROM trial_artifacts AS artifact
        WHERE artifact.ai_semantic_result_id = trial_results.id
          AND NOT EXISTS (
              SELECT 1 FROM ai_semantic_reviews AS review
              WHERE review.semantic_artifact_id = artifact.id
          )
    )
), all_artifacts AS (
    SELECT artifact.id, artifact.review_tier
    FROM ai_semantic_artifacts AS artifact
    JOIN ai_semantic_results AS result ON result.id = artifact.ai_semantic_result_id
    WHERE result.ai_analysis_configuration_id = $1
)
SELECT
  (SELECT count(*) FROM scoped_work),
  (SELECT count(*) FROM scoped_work WHERE work_status = 'complete' AND execution_status = 'complete'),
  (SELECT count(*) FROM scoped_work WHERE trigger = 'trial'),
  (SELECT count(*) FROM fully_reviewed_trial_results),
  (SELECT count(*)
   FROM ai_semantic_reviews AS review
   JOIN trial_artifacts AS artifact ON artifact.id = review.semantic_artifact_id
   WHERE review.verdict = 'major_error'),
  (SELECT count(*) FROM all_artifacts WHERE review_tier = 'required_review'),
  (SELECT count(*) FROM all_artifacts AS artifact
   WHERE review_tier = 'required_review'
     AND EXISTS (SELECT 1 FROM ai_semantic_reviews AS review WHERE review.semantic_artifact_id = artifact.id)),
  (SELECT count(*) FROM all_artifacts WHERE review_tier = 'spot_check'),
  (SELECT count(*) FROM all_artifacts AS artifact
   WHERE review_tier = 'spot_check'
     AND EXISTS (SELECT 1 FROM ai_semantic_reviews AS review WHERE review.semantic_artifact_id = artifact.id)),
  (SELECT count(*) FROM all_artifacts WHERE review_tier = 'routine'),
  (SELECT count(*) FROM all_artifacts AS artifact
   WHERE review_tier = 'routine'
     AND EXISTS (SELECT 1 FROM ai_semantic_reviews AS review WHERE review.semantic_artifact_id = artifact.id))`, configurationID).Scan(
		&gates.AllWorkCount,
		&gates.CompleteWorkCount,
		&gates.TrialBodyCount,
		&gates.FullyReviewedTrialBodyCount,
		&gates.MajorErrorCount,
		&gates.RequiredReviewArtifactCount,
		&gates.ReviewedRequiredArtifactCount,
		&gates.SpotCheckArtifactCount,
		&gates.ReviewedSpotCheckArtifactCount,
		&gates.RoutineArtifactCount,
		&gates.ReviewedRoutineArtifactCount,
	)
	if err != nil {
		return aiReleaseGateCounts{}, fmt.Errorf("read AI publication gate counts: %w", err)
	}
	return gates, nil
}

func (gates aiReleaseGateCounts) allowsPublication() bool {
	return gates.TrialBodyCount == aiTrialSampleSize &&
		gates.FullyReviewedTrialBodyCount == aiTrialSampleSize &&
		gates.MajorErrorCount == 0 &&
		gates.AllWorkCount > 0 &&
		gates.CompleteWorkCount == gates.AllWorkCount &&
		gates.ReviewedRequiredArtifactCount == gates.RequiredReviewArtifactCount &&
		gates.ReviewedSpotCheckArtifactCount >= percentageCeiling(gates.SpotCheckArtifactCount, 20) &&
		gates.ReviewedRoutineArtifactCount >= percentageCeiling(gates.RoutineArtifactCount, 5)
}

func percentageCeiling(total, percentage int) int {
	if total <= 0 || percentage <= 0 {
		return 0
	}
	return (total*percentage + 99) / 100
}

func aiReleaseAttestationSHA256(summary AIReleaseSummary, gates aiReleaseGateCounts) string {
	return sha256Hex(fmt.Sprintf(
		"%s|%s|%s|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d|%d",
		AIReleaseGateVersion, summary.AIConfigurationID, summary.Publisher,
		gates.AllWorkCount, gates.CompleteWorkCount, summary.TrialBodyCount,
		summary.FullyReviewedTrialBodyCount, summary.MajorErrorCount,
		gates.RequiredReviewArtifactCount, gates.ReviewedRequiredArtifactCount,
		gates.SpotCheckArtifactCount, gates.ReviewedSpotCheckArtifactCount,
		gates.RoutineArtifactCount, gates.ReviewedRoutineArtifactCount,
	))
}
