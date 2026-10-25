package jdanalysis

import (
	"context"
	"fmt"
)

// AISemanticReviewInput names one exact persisted conclusion. It has no body
// text, provider response, evidence excerpt, or free-form reviewer rationale.
type AISemanticReviewInput struct {
	AIConfigurationID  string
	AISemanticResultID string
	ArtifactID         string
	Reviewer           string
	Verdict            string
	ErrorCategory      string
}

// AIReviewTierSummary is a redacted stratified-review progress count.
type AIReviewTierSummary struct {
	ArtifactCount         int
	ReviewTier            string
	ReviewedArtifactCount int
}

// AITrialReviewSummary is deliberately limited to configuration identity and
// counts. It is safe for remote status output and comparison reporting.
type AITrialReviewSummary struct {
	AIConfigurationID           string
	FullyReviewedTrialBodyCount int
	MajorErrorCount             int
	ReviewCount                 int
	ReviewTiers                 []AIReviewTierSummary
	TrialBodyCount              int
}

// AITrialComparisonConfigurationSummary is the safe, aggregate view of one
// trial arm. It intentionally excludes model text, semantic values, review
// targets, source coordinates, and all raw provider data.
type AITrialComparisonConfigurationSummary struct {
	AIConfigurationID string
	CompleteTaskCount int
	FailedTaskCount   int
	InputTokenCount   int64
	LogicalTaskCount  int
	OutputTokenCount  int64
	PendingTaskCount  int
}

// AITrialComparisonReport compares only the two pre-bound trial arms through
// opaque IDs, state counts, and returned usage totals.
type AITrialComparisonReport struct {
	Flash AITrialComparisonConfigurationSummary
	Plus  AITrialComparisonConfigurationSummary
}

// RecordAISemanticReview appends one immutable remote verdict. The INSERT is
// bound to the exact result/configuration/artifact triple so reviews cannot be
// copied into a later configuration or silently inherited by a new result.
func (repository *Repository) RecordAISemanticReview(ctx context.Context, input AISemanticReviewInput) error {
	if !validAISemanticReviewInput(input) {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	command, err := repository.pool.Exec(ctx, `
INSERT INTO ai_semantic_reviews (
    ai_semantic_result_id, semantic_artifact_id, reviewer, verdict, error_category
)
SELECT result.id, artifact.id, $4, $5::ai_semantic_review_verdict,
       NULLIF($6, '')::ai_semantic_review_error_category
FROM ai_semantic_results AS result
JOIN ai_semantic_artifacts AS artifact ON artifact.ai_semantic_result_id = result.id
WHERE result.id = $2
  AND result.ai_analysis_configuration_id = $1
  AND artifact.id = $3`,
		input.AIConfigurationID, input.AISemanticResultID, input.ArtifactID,
		input.Reviewer, input.Verdict, input.ErrorCategory)
	if err != nil || command.RowsAffected() != 1 {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return nil
}

// ReadAITrialReviewSummary returns all-review progress for the fixed trial
// scope plus counts per frozen review tier. It does not fetch review targets,
// field coordinates, snippets, candidate strings, or model output.
func (repository *Repository) ReadAITrialReviewSummary(ctx context.Context, configurationID string) (AITrialReviewSummary, error) {
	if configurationID == "" {
		return AITrialReviewSummary{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	summary := AITrialReviewSummary{
		AIConfigurationID: configurationID,
		ReviewTiers: []AIReviewTierSummary{
			{ReviewTier: "routine"},
			{ReviewTier: "spot_check"},
			{ReviewTier: "required_review"},
		},
	}
	err := repository.pool.QueryRow(ctx, `
WITH trial_results AS (
    SELECT DISTINCT result.id
    FROM ai_semantic_results AS result
    JOIN ai_analysis_work_items AS work
      ON work.ai_analysis_configuration_id = result.ai_analysis_configuration_id
     AND work.body_content_version_id = result.body_content_version_id
    WHERE result.ai_analysis_configuration_id = $1
      AND work.trigger = 'trial'
), trial_artifacts AS (
    SELECT artifact.id, artifact.ai_semantic_result_id, artifact.review_tier
    FROM ai_semantic_artifacts AS artifact
    JOIN trial_results ON trial_results.id = artifact.ai_semantic_result_id
), fully_reviewed_results AS (
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
)
SELECT
  (SELECT count(*) FROM trial_results),
  (SELECT count(*) FROM fully_reviewed_results),
  (SELECT count(*) FROM ai_semantic_reviews AS review
   JOIN trial_artifacts AS artifact ON artifact.id = review.semantic_artifact_id),
  (SELECT count(*) FROM ai_semantic_reviews AS review
   JOIN trial_artifacts AS artifact ON artifact.id = review.semantic_artifact_id
   WHERE review.verdict = 'major_error')`, configurationID).Scan(
		&summary.TrialBodyCount, &summary.FullyReviewedTrialBodyCount,
		&summary.ReviewCount, &summary.MajorErrorCount)
	if err != nil {
		return AITrialReviewSummary{}, fmt.Errorf("read AI trial review summary: %w", err)
	}
	rows, err := repository.pool.Query(ctx, `
WITH trial_artifacts AS (
    SELECT artifact.id, artifact.review_tier
    FROM ai_semantic_artifacts AS artifact
    JOIN ai_semantic_results AS result ON result.id = artifact.ai_semantic_result_id
    JOIN ai_analysis_work_items AS work
      ON work.ai_analysis_configuration_id = result.ai_analysis_configuration_id
     AND work.body_content_version_id = result.body_content_version_id
    WHERE result.ai_analysis_configuration_id = $1
      AND work.trigger = 'trial'
)
SELECT review_tier::text, count(*),
       count(*) FILTER (WHERE EXISTS (
           SELECT 1 FROM ai_semantic_reviews AS review
           WHERE review.semantic_artifact_id = trial_artifacts.id
       ))
FROM trial_artifacts
GROUP BY review_tier`, configurationID)
	if err != nil {
		return AITrialReviewSummary{}, fmt.Errorf("read AI trial review tiers: %w", err)
	}
	defer rows.Close()
	byTier := make(map[string]int, len(summary.ReviewTiers))
	for index, tier := range summary.ReviewTiers {
		byTier[tier.ReviewTier] = index
	}
	for rows.Next() {
		var tier AIReviewTierSummary
		if err := rows.Scan(&tier.ReviewTier, &tier.ArtifactCount, &tier.ReviewedArtifactCount); err != nil {
			return AITrialReviewSummary{}, fmt.Errorf("scan AI trial review tier: %w", err)
		}
		index, exists := byTier[tier.ReviewTier]
		if !exists {
			return AITrialReviewSummary{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		summary.ReviewTiers[index] = tier
	}
	if err := rows.Err(); err != nil {
		return AITrialReviewSummary{}, fmt.Errorf("iterate AI trial review tiers: %w", err)
	}
	return summary, nil
}

func (repository *Repository) ReadAITrialComparisonReport(ctx context.Context, plusConfigurationID, flashConfigurationID string) (AITrialComparisonReport, error) {
	if plusConfigurationID == "" || flashConfigurationID == "" || plusConfigurationID == flashConfigurationID {
		return AITrialComparisonReport{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if err := validateAITrialComparisonConfiguration(ctx, repository, plusConfigurationID, "qwen-plus", "candidate"); err != nil {
		return AITrialComparisonReport{}, err
	}
	if err := validateAITrialComparisonConfiguration(ctx, repository, flashConfigurationID, "qwen-flash", "comparison"); err != nil {
		return AITrialComparisonReport{}, err
	}
	plus, err := readAITrialComparisonConfigurationSummary(ctx, repository, plusConfigurationID)
	if err != nil {
		return AITrialComparisonReport{}, err
	}
	flash, err := readAITrialComparisonConfigurationSummary(ctx, repository, flashConfigurationID)
	if err != nil {
		return AITrialComparisonReport{}, err
	}
	return AITrialComparisonReport{Plus: plus, Flash: flash}, nil
}

func validateAITrialComparisonConfiguration(ctx context.Context, repository *Repository, configurationID, model, status string) error {
	var valid bool
	err := repository.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM ai_analysis_configurations
    WHERE id = $1 AND model = $2 AND status = $3::ai_analysis_configuration_status
)`, configurationID, model, status).Scan(&valid)
	if err != nil || !valid {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return nil
}

func readAITrialComparisonConfigurationSummary(ctx context.Context, repository *Repository, configurationID string) (AITrialComparisonConfigurationSummary, error) {
	summary := AITrialComparisonConfigurationSummary{AIConfigurationID: configurationID}
	err := repository.pool.QueryRow(ctx, `
SELECT
  count(*),
  count(*) FILTER (WHERE status = 'pending'),
  count(*) FILTER (WHERE status = 'complete'),
  count(*) FILTER (WHERE status = 'failed_safely'),
  COALESCE((
      SELECT sum(usage.input_tokens)::bigint
      FROM ai_analysis_usage_records AS usage
      JOIN ai_analysis_attempts AS attempt ON attempt.id = usage.attempt_id
      WHERE attempt.ai_analysis_configuration_id = $1
  ), 0),
  COALESCE((
      SELECT sum(usage.output_tokens)::bigint
      FROM ai_analysis_usage_records AS usage
      JOIN ai_analysis_attempts AS attempt ON attempt.id = usage.attempt_id
      WHERE attempt.ai_analysis_configuration_id = $1
  ), 0)
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1`, configurationID).Scan(
		&summary.LogicalTaskCount, &summary.PendingTaskCount,
		&summary.CompleteTaskCount, &summary.FailedTaskCount,
		&summary.InputTokenCount, &summary.OutputTokenCount)
	if err != nil {
		return AITrialComparisonConfigurationSummary{}, fmt.Errorf("read AI comparison summary: %w", err)
	}
	return summary, nil
}

func validAISemanticReviewInput(input AISemanticReviewInput) bool {
	if input.AIConfigurationID == "" || input.AISemanticResultID == "" || input.ArtifactID == "" ||
		!validBoundedLabel(input.Reviewer, 256) {
		return false
	}
	switch input.Verdict {
	case "accepted", "not_reviewable":
		return input.ErrorCategory == ""
	case "minor_error", "major_error":
		switch input.ErrorCategory {
		case "necessity_reversal", "alternative_reversal", "education_reversal", "experience_reversal", "other":
			return true
		default:
			return false
		}
	default:
		return false
	}
}
