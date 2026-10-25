package jdanalysis

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunControlledAIReviewSessionDisplaysRemoteOnlyMaterialAndRecordsFixedVerdict(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-review-session-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-review-session-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')`, aiConfiguration.ID, bodyID); err != nil {
		t.Fatal(err)
	}
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: notConfiguredAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	reviewed, err := RunControlledAIReviewSession(ctx, repository, aiConfiguration.ID, "qwen-plus", strings.NewReader("accepted\n"), &output)
	if err != nil || !reviewed {
		t.Fatalf("controlled AI review session = reviewed:%t err:%v", reviewed, err)
	}
	if !strings.Contains(output.String(), "Go, Go") || !strings.Contains(output.String(), "structured_conclusion") || !strings.Contains(output.String(), "accepted") {
		t.Fatalf("remote review screen omitted source or structured conclusion: %q", output.String())
	}
	var reviews int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM ai_semantic_reviews AS review
JOIN ai_semantic_artifacts AS artifact ON artifact.id = review.semantic_artifact_id
JOIN ai_semantic_results AS result ON result.id = artifact.ai_semantic_result_id
WHERE result.ai_analysis_configuration_id = $1
  AND review.reviewer = 'FAN'
  AND review.verdict = 'accepted'`, aiConfiguration.ID).Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if reviews != 1 {
		t.Fatalf("controlled review records = %d, want 1", reviews)
	}
}

func TestRunControlledAIReviewSessionRejectsFreeFormVerdicts(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-review-session-rejection-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-review-session-rejection-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')`, aiConfiguration.ID, bodyID); err != nil {
		t.Fatal(err)
	}
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: notConfiguredAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if _, err := RunControlledAIReviewSession(ctx, repository, aiConfiguration.ID, "qwen-plus", strings.NewReader("please use this free-form rationale\n"), &output); err == nil {
		t.Fatal("controlled AI review accepted a free-form verdict")
	}
	var reviews int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM ai_semantic_reviews AS review
JOIN ai_semantic_artifacts AS artifact ON artifact.id = review.semantic_artifact_id
JOIN ai_semantic_results AS result ON result.id = artifact.ai_semantic_result_id
WHERE result.ai_analysis_configuration_id = $1`, aiConfiguration.ID).Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if reviews != 0 {
		t.Fatalf("free-form review created %d records", reviews)
	}
}
