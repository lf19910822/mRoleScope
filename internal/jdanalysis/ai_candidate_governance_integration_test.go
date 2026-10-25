package jdanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
)

func TestAISemanticProcessorSendsVerifiedExternalCandidateToPendingDictionaryGovernanceOnly(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	baseBodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, baseBodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-candidate-governance-reference")
	bodyID := insertSyntheticExternalCandidateBody(t, repository, technologyConfigurationID, dictionaryID, technologyID, ruleID)
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-candidate-governance"))
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
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: externalCandidateAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var candidateState string
	var identityCount, candidateArtifactCount, requirementOptionCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT state::text FROM technology_dictionary_candidates WHERE dictionary_version_id = $1 AND candidate_text = 'UncataloguedTool'),
  (SELECT count(*) FROM technology_identities),
  (SELECT count(*) FROM ai_candidate_technology_mentions AS candidate
   JOIN ai_semantic_artifacts AS artifact ON artifact.id = candidate.id
   JOIN ai_semantic_results AS result ON result.id = artifact.ai_semantic_result_id
   WHERE result.ai_analysis_configuration_id = $2 AND result.body_content_version_id = $3),
  (SELECT count(*) FROM ai_technology_requirement_options)
`, dictionaryID, aiConfiguration.ID, bodyID).Scan(&candidateState, &identityCount, &candidateArtifactCount, &requirementOptionCount); err != nil {
		t.Fatal(err)
	}
	if candidateState != "candidate" || identityCount != 1 || candidateArtifactCount != 1 || requirementOptionCount != 0 {
		t.Fatalf("AI candidate governance = state:%s identities:%d artifacts:%d requirement-options:%d", candidateState, identityCount, candidateArtifactCount, requirementOptionCount)
	}
}

func TestAISemanticProcessorRejectsEnabledAliasCandidateWithoutCreatingGovernanceRecords(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-enabled-alias-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-enabled-alias-candidate"))
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
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: enabledAliasCandidateAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); !errors.Is(err, ErrAISemanticWorkFailed) {
		t.Fatalf("enabled alias candidate processing error = %v, want ErrAISemanticWorkFailed", err)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "failed_safely", "failed_safely", "not_analyzed")

	var dictionaryCandidates, identities, candidateArtifacts, requirementOptions int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM technology_dictionary_candidates WHERE dictionary_version_id = $1),
  (SELECT count(*) FROM technology_identities),
  (SELECT count(*) FROM ai_candidate_technology_mentions),
  (SELECT count(*) FROM ai_technology_requirement_options)
`, dictionaryID).Scan(&dictionaryCandidates, &identities, &candidateArtifacts, &requirementOptions); err != nil {
		t.Fatal(err)
	}
	if dictionaryCandidates != 0 || identities != 1 || candidateArtifacts != 0 || requirementOptions != 0 {
		t.Fatalf("enabled alias candidate left governance records = candidates:%d identities:%d artifacts:%d options:%d", dictionaryCandidates, identities, candidateArtifacts, requirementOptions)
	}
}

func TestAISemanticProcessorV2SkipsEnabledAliasCandidateAndCompletesOtherSemantics(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-v2-enabled-alias-reference")
	configurationInput := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-v2-enabled-alias-candidate")
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
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')`, aiConfiguration.ID, bodyID); err != nil {
		t.Fatal(err)
	}
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: enabledAliasCandidateAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatalf("V2 alias candidate processing error = %v; want local skip", err)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "complete", "complete", "not_configured")

	var dictionaryCandidates, candidateArtifacts int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM technology_dictionary_candidates WHERE dictionary_version_id = $1),
  (SELECT count(*) FROM ai_candidate_technology_mentions AS candidate
   JOIN ai_semantic_artifacts AS artifact ON artifact.id = candidate.id
   JOIN ai_semantic_results AS result ON result.id = artifact.ai_semantic_result_id
   WHERE result.ai_analysis_configuration_id = $2 AND result.body_content_version_id = $3)
`, dictionaryID, aiConfiguration.ID, bodyID).Scan(&dictionaryCandidates, &candidateArtifacts); err != nil {
		t.Fatal(err)
	}
	if dictionaryCandidates != 0 || candidateArtifacts != 0 {
		t.Fatalf("V2 enabled alias created candidate records: candidates:%d artifacts:%d", dictionaryCandidates, candidateArtifacts)
	}
}

func insertSyntheticExternalCandidateBody(t *testing.T, repository *Repository, technologyConfigurationID, dictionaryID, technologyID, ruleID string) string {
	t.Helper()
	ctx := context.Background()
	const (
		bodyID      = "00000000-0000-4000-8000-000000000911"
		jobID       = "00000000-0000-4000-8000-000000000910"
		description = "Go, UncataloguedTool"
	)
	descriptionSHA := sha256HexForCandidateTest(description)
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'synthetic-source', 'synthetic-ai-candidate-job')`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, 'jd-body-v1', $4, '', '', $5, $6, $6)`,
		bodyID, jobID, sha256HexForCandidateTest(description+"||"), description,
		descriptionSHA, sha256HexForCandidateTest("")); err != nil {
		t.Fatal(err)
	}
	var deterministicResultID, mentionID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO technology_analysis_results (
    body_content_version_id, parser_version, dictionary_version_id, configuration_id, status, completed_at
) VALUES ($1, 'synthetic-ai-candidate-governance-reference', $2, $3, 'complete', now())
RETURNING id::text`, bodyID, dictionaryID, technologyConfigurationID).Scan(&deterministicResultID); err != nil {
		t.Fatal(err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO technology_mentions (analysis_result_id, technology_identity_id)
VALUES ($1, $2)
RETURNING id::text`, deterministicResultID, technologyID).Scan(&mentionID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_mention_evidence (
    technology_mention_id, source_field, normalization_version, normalized_field_sha256,
    dictionary_rule_id, matched_alias, codepoint_start, codepoint_end
) VALUES ($1, 'description', 'jd-body-v1', $2, $3, 'Go', 0, 2)`, mentionID, descriptionSHA, ruleID); err != nil {
		t.Fatal(err)
	}
	return bodyID
}

func sha256HexForCandidateTest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type externalCandidateAIModelClient struct{}

func (externalCandidateAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions: []AICandidateMentionResponse{{
			CandidateText:        "UncataloguedTool",
			Necessity:            "preferred",
			ReviewRecommendation: "required_review",
			Role:                 "qualification",
			SpanID:               "description:s02",
		}},
		Experience:                  AIExperienceResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{}, ReviewRecommendation: "routine"},
		MinimumEducation:            AIMinimumEducationResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		RequestID:                   input.RequestID,
		ResponseSchemaVersion:       AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{},
	})
	if err != nil {
		return AIModelResponse{}, err
	}
	return AIModelResponse{Content: raw, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}

type enabledAliasCandidateAIModelClient struct{}

func (enabledAliasCandidateAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions: []AICandidateMentionResponse{{
			CandidateText:        "Go",
			Necessity:            "required",
			ReviewRecommendation: "required_review",
			Role:                 "qualification",
			SpanID:               "description:s01",
		}},
		Experience:                  AIExperienceResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{}, ReviewRecommendation: "routine"},
		MinimumEducation:            AIMinimumEducationResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		RequestID:                   input.RequestID,
		ResponseSchemaVersion:       AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{},
	})
	if err != nil {
		return AIModelResponse{}, err
	}
	return AIModelResponse{Content: raw, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}
