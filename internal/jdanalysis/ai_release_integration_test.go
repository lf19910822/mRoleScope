package jdanalysis

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestPublishAIConfigurationRejectsIncompleteCandidateWithoutMovingDefault(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-release-incomplete-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-release-incomplete-candidate"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repository.PublishAIConfiguration(ctx, aiConfiguration.ID); !errors.Is(err, ErrAIReleaseGateFailed) {
		t.Fatalf("publish incomplete AI candidate error = %v, want ErrAIReleaseGateFailed", err)
	}

	var configurationStatus string
	var defaultConfigurationID *string
	var attestations int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT status::text FROM ai_analysis_configurations WHERE id = $1),
  (SELECT ai_analysis_configuration_id::text FROM ai_analysis_publication_state WHERE singleton = true),
  (SELECT count(*) FROM ai_analysis_release_attestations WHERE ai_analysis_configuration_id = $1)
`, aiConfiguration.ID).Scan(&configurationStatus, &defaultConfigurationID, &attestations); err != nil {
		t.Fatal(err)
	}
	if configurationStatus != "candidate" || defaultConfigurationID != nil || attestations != 0 {
		t.Fatalf("incomplete AI publication mutated state = status:%s default:%v attestations:%d", configurationStatus, defaultConfigurationID, attestations)
	}
}

func TestPublishAIConfigurationPublishesFullyReviewedPlusTrialAndWritesAttestation(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-release-success-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-release-success-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	seedCompletedReviewedAITrialForRelease(t, repository, aiConfiguration.ID, aiTrialSampleSize)

	summary, err := repository.PublishAIConfiguration(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.AIConfigurationID != aiConfiguration.ID || summary.Publisher != AIReleasePublisher || summary.TrialBodyCount != aiTrialSampleSize || summary.TotalWorkCount != aiTrialSampleSize || summary.TerminalWorkCount != aiTrialSampleSize || summary.StructuralSuccessCount != aiTrialSampleSize || summary.SafeFailureCount != 0 {
		t.Fatalf("AI release summary = %#v", summary)
	}

	var configurationStatus, defaultConfigurationID, publisher string
	var attestations int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT status::text FROM ai_analysis_configurations WHERE id = $1),
  (SELECT ai_analysis_configuration_id::text FROM ai_analysis_publication_state WHERE singleton = true),
  (SELECT publisher FROM ai_analysis_release_attestations WHERE ai_analysis_configuration_id = $1),
  (SELECT count(*) FROM ai_analysis_release_attestations WHERE ai_analysis_configuration_id = $1)
`, aiConfiguration.ID).Scan(&configurationStatus, &defaultConfigurationID, &publisher, &attestations); err != nil {
		t.Fatal(err)
	}
	if configurationStatus != "published" || defaultConfigurationID != aiConfiguration.ID || publisher != AIReleasePublisher || attestations != 1 {
		t.Fatalf("published AI state = status:%s default:%s publisher:%s attestations:%d", configurationStatus, defaultConfigurationID, publisher, attestations)
	}
}

func TestPublishAIConfigurationAllowsMajorTrialReviewErrorAndWritesMechanicalAttestation(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-release-major-error-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-release-major-error-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	seedCompletedAITrialForRelease(t, repository, aiConfiguration.ID, aiTrialSampleSize, 0)

	summary, err := repository.PublishAIConfiguration(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatalf("major review error must not gate mechanical publication: %v", err)
	}
	var configurationStatus string
	var defaultConfigurationID *string
	var attestations int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT status::text FROM ai_analysis_configurations WHERE id = $1),
  (SELECT ai_analysis_configuration_id::text FROM ai_analysis_publication_state WHERE singleton = true),
  (SELECT count(*) FROM ai_analysis_release_attestations WHERE ai_analysis_configuration_id = $1)
`, aiConfiguration.ID).Scan(&configurationStatus, &defaultConfigurationID, &attestations); err != nil {
		t.Fatal(err)
	}
	if configurationStatus != "published" || defaultConfigurationID == nil || *defaultConfigurationID != aiConfiguration.ID || attestations != 1 || summary.StructuralSuccessCount != aiTrialSampleSize || summary.SafeFailureCount != 0 {
		t.Fatalf("major-error mechanical publication = status:%s default:%v attestations:%d summary:%#v", configurationStatus, defaultConfigurationID, attestations, summary)
	}
}

func TestPublishAIConfigurationAllowsTerminalSafeFailureWithoutSemanticReviews(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-release-safe-failure-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-release-safe-failure-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	seedTerminalAITrialForMechanicalPublication(t, repository, aiConfiguration.ID, aiTrialSampleSize, true)

	summary, err := repository.PublishAIConfiguration(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatalf("terminal safe failure must remain attestable, not block publication: %v", err)
	}
	if summary.TrialBodyCount != aiTrialSampleSize {
		t.Fatalf("mechanical publication summary = %#v", summary)
	}
	var totalWork, terminalWork, structuralSuccess, safeFailures, attempts int
	var inputTokens, outputTokens, chargeNanoRMB int64
	if err := repository.pool.QueryRow(ctx, `
SELECT total_work_count, terminal_work_count, structural_success_count,
       safe_failure_count, attempt_count, input_token_count,
       output_token_count, charge_nano_rmb
FROM ai_analysis_release_attestations
WHERE ai_analysis_configuration_id = $1`, aiConfiguration.ID).Scan(
		&totalWork, &terminalWork, &structuralSuccess, &safeFailures,
		&attempts, &inputTokens, &outputTokens, &chargeNanoRMB,
	); err != nil {
		t.Fatal(err)
	}
	if totalWork != aiTrialSampleSize || terminalWork != aiTrialSampleSize || structuralSuccess != aiTrialSampleSize-1 || safeFailures != 1 || attempts != 0 || inputTokens != 0 || outputTokens != 0 || chargeNanoRMB != 0 {
		t.Fatalf("mechanical release attestation = work:%d/%d success:%d failure:%d attempts:%d usage:%d/%d/%d", totalWork, terminalWork, structuralSuccess, safeFailures, attempts, inputTokens, outputTokens, chargeNanoRMB)
	}
}

func seedCompletedReviewedAITrialForRelease(t *testing.T, repository *Repository, configurationID string, count int) {
	seedCompletedAITrialForRelease(t, repository, configurationID, count, -1)
}

func seedCompletedAITrialForRelease(t *testing.T, repository *Repository, configurationID string, count, majorErrorIndex int) {
	t.Helper()
	ctx := context.Background()
	for index := 0; index < count; index++ {
		jobID := fmt.Sprintf("00000000-0000-4000-8001-%012d", index+1)
		bodyID := fmt.Sprintf("00000000-0000-4000-8002-%012d", index+1)
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'synthetic-release-source', $2)`, jobID, fmt.Sprintf("synthetic-release-job-%d", index+1)); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, 'jd-body-v1', '', '', '', $4, $4, $4)`, bodyID, jobID, testSHA256("release-body-"+bodyID), testSHA256("")); err != nil {
			t.Fatal(err)
		}
		var resultID, artifactID string
		if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_results (
    body_content_version_id, ai_analysis_configuration_id, execution_status, semantic_status, completed_at
) VALUES ($1, $2, 'complete', 'not_configured', now())
RETURNING id::text`, bodyID, configurationID).Scan(&resultID); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (
    ai_analysis_configuration_id, body_content_version_id, trigger, status, completed_at
) VALUES ($1, $2, 'trial', 'complete', now())`, configurationID, bodyID); err != nil {
			t.Fatal(err)
		}
		if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (
    ai_semantic_result_id, artifact_kind, semantic_status, review_tier
) VALUES ($1, 'experience_summary', 'not_configured', 'routine')
RETURNING id::text`, resultID).Scan(&artifactID); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_experience_summaries (id, status, base_year)
VALUES ($1, 'not_configured', NULL)`, artifactID); err != nil {
			t.Fatal(err)
		}
		verdict, errorCategory := "accepted", any(nil)
		if index == majorErrorIndex {
			verdict, errorCategory = "major_error", "experience_reversal"
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_semantic_reviews (
    ai_semantic_result_id, semantic_artifact_id, reviewer, verdict, error_category
) VALUES ($1, $2, 'FAN', $3::ai_semantic_review_verdict, $4::ai_semantic_review_error_category)`, resultID, artifactID, verdict, errorCategory); err != nil {
			t.Fatal(err)
		}
	}
}

func seedTerminalAITrialForMechanicalPublication(t *testing.T, repository *Repository, configurationID string, count int, includeSafeFailure bool) {
	t.Helper()
	ctx := context.Background()
	for index := 0; index < count; index++ {
		jobID := fmt.Sprintf("00000000-0000-4000-8101-%012d", index+1)
		bodyID := fmt.Sprintf("00000000-0000-4000-8102-%012d", index+1)
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'synthetic-mechanical-release-source', $2)`, jobID, fmt.Sprintf("synthetic-mechanical-release-job-%d", index+1)); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, 'jd-body-v1', '', '', '', $4, $4, $4)`, bodyID, jobID, testSHA256("mechanical-release-body-"+bodyID), testSHA256("")); err != nil {
			t.Fatal(err)
		}
		workStatus, executionStatus, semanticStatus, safeError := "complete", "complete", "not_configured", any(nil)
		if includeSafeFailure && index == 0 {
			workStatus, executionStatus, semanticStatus, safeError = "failed_safely", "failed_safely", "not_analyzed", "provider_timeout"
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_semantic_results (
    body_content_version_id, ai_analysis_configuration_id, execution_status,
    semantic_status, safe_error_code, completed_at
) VALUES ($1, $2, $3::ai_semantic_execution_status, $4::ai_semantic_value_status,
          NULLIF($5, '')::ai_safe_error_code, now())`, bodyID, configurationID, executionStatus, semanticStatus, safeError); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (
    ai_analysis_configuration_id, body_content_version_id, trigger, status,
    last_safe_error_code, completed_at
) VALUES ($1, $2, 'trial', $3::ai_analysis_work_status,
          NULLIF($4, '')::ai_safe_error_code, now())`, configurationID, bodyID, workStatus, safeError); err != nil {
			t.Fatal(err)
		}
	}
}
