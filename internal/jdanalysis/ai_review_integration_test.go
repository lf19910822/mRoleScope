package jdanalysis

import (
	"context"
	"testing"
)

func TestRecordAISemanticReviewIsImmutableBoundToExactResultAndSummarizedWithoutText(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-review-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-review-candidate"))
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

	var resultID, experienceArtifactID, educationArtifactID string
	if err := repository.pool.QueryRow(ctx, `
SELECT result.id::text,
       min(artifact.id::text) FILTER (WHERE artifact.artifact_kind = 'experience_summary'),
       min(artifact.id::text) FILTER (WHERE artifact.artifact_kind = 'education_summary')
FROM ai_semantic_results AS result
JOIN ai_semantic_artifacts AS artifact ON artifact.ai_semantic_result_id = result.id
WHERE result.ai_analysis_configuration_id = $1
  AND result.body_content_version_id = $2
GROUP BY result.id`, aiConfiguration.ID, bodyID).Scan(&resultID, &experienceArtifactID, &educationArtifactID); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordAISemanticReview(ctx, AISemanticReviewInput{
		AIConfigurationID:  aiConfiguration.ID,
		AISemanticResultID: resultID,
		ArtifactID:         experienceArtifactID,
		Reviewer:           "FAN",
		Verdict:            "accepted",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordAISemanticReview(ctx, AISemanticReviewInput{
		AIConfigurationID:  aiConfiguration.ID,
		AISemanticResultID: resultID,
		ArtifactID:         educationArtifactID,
		Reviewer:           "FAN",
		Verdict:            "major_error",
		ErrorCategory:      "education_reversal",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordAISemanticReview(ctx, AISemanticReviewInput{
		AIConfigurationID:  aiConfiguration.ID,
		AISemanticResultID: resultID,
		ArtifactID:         experienceArtifactID,
		Reviewer:           "FAN",
		Verdict:            "accepted",
	}); err == nil {
		t.Fatal("a second verdict rewrote an immutable review")
	}
	if err := repository.RecordAISemanticReview(ctx, AISemanticReviewInput{
		AIConfigurationID:  aiConfiguration.ID,
		AISemanticResultID: "00000000-0000-4000-8000-000000000999",
		ArtifactID:         experienceArtifactID,
		Reviewer:           "OTHER",
		Verdict:            "accepted",
	}); err == nil {
		t.Fatal("review accepted an artifact under a different semantic result")
	}

	summary, err := repository.ReadAITrialReviewSummary(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.AIConfigurationID != aiConfiguration.ID || summary.TrialBodyCount != 1 ||
		summary.FullyReviewedTrialBodyCount != 1 || summary.MajorErrorCount != 1 ||
		summary.ReviewCount != 2 || len(summary.ReviewTiers) != 3 {
		t.Fatalf("safe AI trial review summary = %#v", summary)
	}
}
