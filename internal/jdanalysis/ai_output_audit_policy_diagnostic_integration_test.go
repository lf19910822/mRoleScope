package jdanalysis

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAIOutputAuditPolicyDiagnosticReplaysThroughPersistenceAndRollsBack(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-output-audit-policy-replay-reference")
	configurationInput := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-output-audit-policy-replay")
	configurationInput.PromptSHA256 = AISystemPromptV3SHA256()
	configurationInput.ValidatorVersion = AIValidatorVersionV2
	configurationInput.ValidatorSHA256 = sha256Hex(AIValidatorVersionV2)
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, configurationInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var workID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')
RETURNING id::text`, aiConfiguration.ID, bodyID).Scan(&workID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_work_items
SET status = 'failed_safely', attempt_count = 1, completed_at = now(),
    last_safe_error_code = 'policy_rejected'
WHERE id = $1`, workID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'failed_safely', semantic_status = 'not_analyzed',
    safe_error_code = 'policy_rejected', completed_at = now()
WHERE ai_analysis_configuration_id = $1 AND body_content_version_id = $2`, aiConfiguration.ID, bodyID); err != nil {
		t.Fatal(err)
	}
	var runID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_runs (
    ai_analysis_configuration_id, trigger, status, completed_at,
    claimed_count, completed_count, failed_count
) VALUES ($1, 'trial', 'complete', now(), 1, 1, 0)
RETURNING id::text`, aiConfiguration.ID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_attempts (
    ai_analysis_configuration_id, work_item_id, run_id, attempt_number, status,
    input_token_limit, output_token_limit, provider_request_id, completed_at
) VALUES ($1, $2, $3, 1, 'succeeded', 10000, 2500, $4, now())`,
		aiConfiguration.ID, workID, runID, deterministicAIProviderRequestID(workID, 1)); err != nil {
		t.Fatal(err)
	}

	modelInput, err := repository.BuildAIModelInput(ctx, deterministicAIProviderRequestID(workID, 1), aiConfiguration.ID, bodyID)
	if err != nil {
		t.Fatal(err)
	}
	rawOutput, err := json.Marshal(AISemanticResponse{
		CandidateMentions:           []AICandidateMentionResponse{},
		Experience:                  AIExperienceResponse{Status: "not_configured", Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{}, EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		MinimumEducation:            AIMinimumEducationResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		RequestID:                   modelInput.RequestID,
		ResponseSchemaVersion:       AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{},
	})
	if err != nil {
		t.Fatal(err)
	}
	store := NewAIOutputAuditStore(t.TempDir())
	reservation, err := store.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reservation.Capture(AIOutputAuditIdentity{
		ConfigurationID:      aiConfiguration.ID,
		WorkItemID:           workID,
		AttemptNumber:        1,
		BodyContentVersionID: bodyID,
	}, rawOutput); err != nil {
		t.Fatal(err)
	}

	diagnostic, err := repository.DiagnoseCapturedAIOutputPolicyFailures(ctx, store, []string{aiConfiguration.ID}, "qwen-plus")
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.ProcessedCount != 1 || len(diagnostic.Counts) != 1 || diagnostic.Counts[0] != (AIOutputAuditPolicyDiagnosticCount{
		Category: AIOutputAuditPolicyDiagnosticReplaySucceeded,
		Count:    1,
	}) {
		t.Fatalf("policy diagnostic = %#v, want one successful rollback replay", diagnostic)
	}

	var workStatus, workSafeCode, resultStatus, resultSemanticStatus, resultSafeCode string
	if err := repository.pool.QueryRow(ctx, `
SELECT work.status::text, work.last_safe_error_code,
       result.execution_status::text, result.semantic_status::text, result.safe_error_code
FROM ai_analysis_work_items AS work
JOIN ai_semantic_results AS result
  ON result.ai_analysis_configuration_id = work.ai_analysis_configuration_id
 AND result.body_content_version_id = work.body_content_version_id
WHERE work.id = $1`, workID).Scan(&workStatus, &workSafeCode, &resultStatus, &resultSemanticStatus, &resultSafeCode); err != nil {
		t.Fatal(err)
	}
	if workStatus != "failed_safely" || workSafeCode != string(AISafeErrorPolicyRejected) ||
		resultStatus != "failed_safely" || resultSemanticStatus != "not_analyzed" || resultSafeCode != string(AISafeErrorPolicyRejected) {
		t.Fatalf("rollback diagnostic changed durable state: work=%s/%s result=%s/%s/%s", workStatus, workSafeCode, resultStatus, resultSemanticStatus, resultSafeCode)
	}
	var artifactCount int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM ai_semantic_artifacts`).Scan(&artifactCount); err != nil {
		t.Fatal(err)
	}
	if artifactCount != 0 {
		t.Fatalf("rollback diagnostic wrote semantic artifacts: %d", artifactCount)
	}
}
