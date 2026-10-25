package jdanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestAISemanticProcessorCompletesCandidateWorkOnceAndHonorsConfigurationLock(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-processor-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-processor-candidate"))
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

	release, acquired, err := repository.TryAcquireAIConfigurationLock(ctx, aiConfiguration.ID)
	if err != nil || !acquired {
		t.Fatalf("acquire AI configuration lock = acquired:%t err:%v", acquired, err)
	}
	processor := AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: &notConfiguredAIModelClient{}}}
	if _, err := processor.ProcessPending(ctx, aiConfiguration.ID); !errors.Is(err, ErrAIConfigurationBusy) {
		t.Fatalf("process while AI lock is held error = %v, want ErrAIConfigurationBusy", err)
	}
	release()

	first, err := processor.ProcessPending(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.RunID == "" || first.Claimed != 1 || first.Completed != 1 || first.Failed != 0 {
		t.Fatalf("first AI process summary = %#v", first)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "complete", "complete", "not_configured")
	var artifactCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM ai_semantic_artifacts AS artifact
JOIN ai_semantic_results AS result ON result.id = artifact.ai_semantic_result_id
WHERE result.ai_analysis_configuration_id = $1
  AND result.body_content_version_id = $2`, aiConfiguration.ID, bodyID).Scan(&artifactCount); err != nil {
		t.Fatal(err)
	}
	if artifactCount != 2 {
		t.Fatalf("persisted base semantic artifacts = %d, want 2", artifactCount)
	}

	second, err := processor.ProcessPending(ctx, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Claimed != 0 || second.Completed != 0 || second.Failed != 0 {
		t.Fatalf("duplicate AI process summary = %#v", second)
	}
}

func TestAISemanticProcessorRecoversExplicitlyRetriedSafeFailureWithoutDuplicatingWork(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-recovery-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-recovery-candidate"))
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

	failing := AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: failingAIModelClient{}}}
	if _, err := failing.ProcessPending(ctx, aiConfiguration.ID); !errors.Is(err, ErrAISemanticWorkFailed) {
		t.Fatalf("failed AI processing error = %v, want ErrAISemanticWorkFailed", err)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "failed_safely", "failed_safely", "not_analyzed")

	resumed, err := repository.ResumeFailedAIWork(ctx, aiConfiguration.ID)
	if err != nil || resumed != 1 {
		t.Fatalf("resume failed AI work = %d, %v; want 1, nil", resumed, err)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "pending", "pending", "not_analyzed")

	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: notConfiguredAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "complete", "complete", "not_configured")
	var attempts, workAttemptCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT (SELECT count(*) FROM ai_analysis_attempts),
       (SELECT attempt_count FROM ai_analysis_work_items WHERE ai_analysis_configuration_id = $1 AND body_content_version_id = $2)`, aiConfiguration.ID, bodyID).Scan(&attempts, &workAttemptCount); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || workAttemptCount != 2 {
		t.Fatalf("recovered AI attempts = rows:%d work:%d, want 2/2", attempts, workAttemptCount)
	}
}

func TestAISemanticProcessorPersistsValidatedTechnologyRequirementWithoutCreatingTechnology(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-requirement-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-requirement-candidate"))
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
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: oneVerifiedRequirementAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var groups, options, identities int
	var role, necessity, operator string
	var minSatisfied int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM ai_technology_requirement_groups),
  (SELECT count(*) FROM ai_technology_requirement_options),
  (SELECT count(*) FROM technology_identities),
  (SELECT role::text FROM ai_technology_requirement_groups),
  (SELECT necessity::text FROM ai_technology_requirement_groups),
  (SELECT operator::text FROM ai_technology_requirement_groups),
  (SELECT min_satisfied FROM ai_technology_requirement_groups)
`).Scan(&groups, &options, &identities, &role, &necessity, &operator, &minSatisfied); err != nil {
		t.Fatal(err)
	}
	if groups != 1 || options != 1 || identities != 1 || role != "qualification" || necessity != "required" || operator != "at_least" || minSatisfied != 1 {
		t.Fatalf("persisted AI requirement = groups:%d options:%d identities:%d role:%s necessity:%s operator:%s min:%d", groups, options, identities, role, necessity, operator, minSatisfied)
	}
}

func TestAISemanticProcessorPersistsResponsibilityGroupWithoutCandidateConstraint(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-responsibility-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-responsibility-candidate"))
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
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: responsibilityGroupAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var role, necessity string
	var operator, minimumSatisfied *string
	var optionCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  group_record.role::text,
  group_record.necessity::text,
  group_record.operator::text,
  group_record.min_satisfied::text,
  (SELECT count(*) FROM ai_technology_requirement_options AS option_record WHERE option_record.requirement_group_id = group_record.id)
FROM ai_technology_requirement_groups AS group_record
WHERE group_record.role = 'responsibility'`).Scan(&role, &necessity, &operator, &minimumSatisfied, &optionCount); err != nil {
		t.Fatal(err)
	}
	if role != "responsibility" || necessity != "unspecified" || operator != nil || minimumSatisfied != nil || optionCount != 1 {
		t.Fatalf("persisted AI responsibility group = role:%s necessity:%s operator:%v minimum:%v options:%d", role, necessity, operator, minimumSatisfied, optionCount)
	}
}

func TestAISemanticProcessorPersistsExplicitOverallExperienceWithProgramOwnedEvidence(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-experience-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-experience-candidate"))
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
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: overallExperienceAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var summaryStatus, subjectKind string
	var baseYear, minimumYears int
	var evidenceCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT summary.status::text, summary.base_year::integer,
       item.subject_kind, item.minimum_years::integer,
       (SELECT count(*) FROM ai_semantic_evidence AS evidence WHERE evidence.semantic_artifact_id = item.id)
FROM ai_experience_summaries AS summary
JOIN ai_experience_requirement_items AS item ON item.experience_summary_id = summary.id
`).Scan(&summaryStatus, &baseYear, &subjectKind, &minimumYears, &evidenceCount); err != nil {
		t.Fatal(err)
	}
	if summaryStatus != "configured" || baseYear != 3 || subjectKind != "overall" || minimumYears != 3 || evidenceCount != 1 {
		t.Fatalf("persisted AI experience = status:%s base:%d subject:%s minimum:%d evidence:%d", summaryStatus, baseYear, subjectKind, minimumYears, evidenceCount)
	}
}

func TestAISemanticProcessorPersistsTechnologyScopedExperienceAgainstVerifiedIdentity(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-technology-experience-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-technology-experience-candidate"))
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
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: technologyExperienceAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var subjectKind, persistedTechnologyID string
	var minimumYears, evidenceCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT item.subject_kind, item.technology_identity_id::text, item.minimum_years::integer,
       (SELECT count(*) FROM ai_semantic_evidence AS evidence WHERE evidence.semantic_artifact_id = item.id)
FROM ai_experience_requirement_items AS item
WHERE item.subject_kind = 'technology'`).Scan(&subjectKind, &persistedTechnologyID, &minimumYears, &evidenceCount); err != nil {
		t.Fatal(err)
	}
	if subjectKind != "technology" || persistedTechnologyID != technologyID || minimumYears != 3 || evidenceCount != 1 {
		t.Fatalf("persisted AI technology experience = subject:%s identity:%s minimum:%d evidence:%d", subjectKind, persistedTechnologyID, minimumYears, evidenceCount)
	}
}

func TestAISemanticProcessorPersistsEducationExperienceConditionBranches(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-condition-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-condition-candidate"))
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
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: educationExperienceConditionAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var branches, bachelorThreeYears, masterTwoYears, evidenceCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
    count(*),
    count(*) FILTER (WHERE education_level = 'bachelor' AND minimum_experience_years = 3),
    count(*) FILTER (WHERE education_level = 'master' AND minimum_experience_years = 2),
    (SELECT count(*)
     FROM ai_semantic_evidence AS evidence
     JOIN ai_semantic_condition_branches AS branch ON branch.id = evidence.semantic_artifact_id)
FROM ai_semantic_condition_branches`).Scan(&branches, &bachelorThreeYears, &masterTwoYears, &evidenceCount); err != nil {
		t.Fatal(err)
	}
	if branches != 2 || bachelorThreeYears != 1 || masterTwoYears != 1 || evidenceCount != 2 {
		t.Fatalf("persisted AI condition branches = total:%d bachelor_3:%d master_2:%d evidence:%d", branches, bachelorThreeYears, masterTwoYears, evidenceCount)
	}
}

func TestAISemanticProcessorNormalizesJuniorCollegeConditionBranchBeforePersistence(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-ai-junior-college-condition-reference")
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-junior-college-condition-candidate"))
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
	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: juniorCollegeConditionAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var branchEducation, minimumEducation string
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT education_level::text FROM ai_semantic_condition_branches),
  (SELECT minimum_level::text FROM ai_minimum_education_requirements)
`).Scan(&branchEducation, &minimumEducation); err != nil {
		t.Fatal(err)
	}
	if branchEducation != "associate_or_lower" || minimumEducation != "associate_or_lower" {
		t.Fatalf("junior college education normalization = branch:%s minimum:%s", branchEducation, minimumEducation)
	}
}

func preparePublishedTechnologyConfigurationForAI(t *testing.T, repository *Repository, bodyID, dictionaryID, technologyID, ruleID, parserVersion string) string {
	t.Helper()
	ctx := context.Background()
	configurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, parserVersion)
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      configurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, configurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, configurationID); err != nil {
		t.Fatal(err)
	}
	return configurationID
}

func assertAISemanticWorkState(t *testing.T, repository *Repository, configurationID, bodyID, wantWorkStatus, wantExecutionStatus, wantSemanticStatus string) {
	t.Helper()
	var workStatus, executionStatus, semanticStatus string
	if err := repository.pool.QueryRow(context.Background(), `
SELECT work.status::text, result.execution_status::text, result.semantic_status::text
FROM ai_analysis_work_items AS work
JOIN ai_semantic_results AS result
  ON result.body_content_version_id = work.body_content_version_id
 AND result.ai_analysis_configuration_id = work.ai_analysis_configuration_id
WHERE work.ai_analysis_configuration_id = $1
  AND work.body_content_version_id = $2`, configurationID, bodyID).Scan(&workStatus, &executionStatus, &semanticStatus); err != nil {
		t.Fatal(err)
	}
	if workStatus != wantWorkStatus || executionStatus != wantExecutionStatus || semanticStatus != wantSemanticStatus {
		t.Fatalf("AI work/result state = work:%s execution:%s semantic:%s, want work:%s execution:%s semantic:%s", workStatus, executionStatus, semanticStatus, wantWorkStatus, wantExecutionStatus, wantSemanticStatus)
	}
}

type notConfiguredAIModelClient struct{}

func (notConfiguredAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions:           []AICandidateMentionResponse{},
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

type failingAIModelClient struct{}

func (failingAIModelClient) Analyze(context.Context, AIModelInput) (AIModelResponse, error) {
	return AIModelResponse{}, AIProviderFailure{Code: AISafeErrorProviderAuthentication}
}

type oneVerifiedRequirementAIModelClient struct{}

func (oneVerifiedRequirementAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	mention := input.VerifiedTechnologyMentions[0]
	minimumSatisfied := 1
	operator := "at_least"
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions:     []AICandidateMentionResponse{},
		Experience:            AIExperienceResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{}, ReviewRecommendation: "routine"},
		MinimumEducation:      AIMinimumEducationResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		RequestID:             input.RequestID,
		ResponseSchemaVersion: AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{{
			LocalID:              "required-go",
			Role:                 "qualification",
			Necessity:            "required",
			Operator:             &operator,
			MinSatisfied:         &minimumSatisfied,
			Options:              []AITechnologyRequirementOptionResponse{{TechnologyKey: mention.TechnologyKey, MentionEvidenceIDs: []string{mention.MentionEvidenceID}}},
			EvidenceSpanIDs:      []string{mention.SpanID},
			ReviewRecommendation: "routine",
		}},
	})
	if err != nil {
		return AIModelResponse{}, err
	}
	return AIModelResponse{Content: raw, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}

type responsibilityGroupAIModelClient struct{}

func (responsibilityGroupAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	mention := input.VerifiedTechnologyMentions[0]
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions:     []AICandidateMentionResponse{},
		Experience:            AIExperienceResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{}, ReviewRecommendation: "routine"},
		MinimumEducation:      AIMinimumEducationResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		RequestID:             input.RequestID,
		ResponseSchemaVersion: AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{{
			LocalID:              "responsibility-go",
			Role:                 "responsibility",
			Necessity:            "unspecified",
			Operator:             nil,
			MinSatisfied:         nil,
			Options:              []AITechnologyRequirementOptionResponse{{TechnologyKey: mention.TechnologyKey, MentionEvidenceIDs: []string{mention.MentionEvidenceID}}},
			EvidenceSpanIDs:      []string{mention.SpanID},
			ReviewRecommendation: "routine",
		}},
	})
	if err != nil {
		return AIModelResponse{}, err
	}
	return AIModelResponse{Content: raw, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}

type overallExperienceAIModelClient struct{}

func (overallExperienceAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	baseYear := 3
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions: []AICandidateMentionResponse{},
		Experience: AIExperienceResponse{
			Status:                    "configured",
			BaseYear:                  &baseYear,
			EvidenceSpanIDs:           []string{"description:s01"},
			Items:                     []AIExperienceItemResponse{{ScopeType: "base", ScopeKey: "overall", MinYears: 3, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine"}},
			ConditionalQualifications: []AIConditionalQualificationResponse{},
			ReviewRecommendation:      "routine",
		},
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

type educationExperienceConditionAIModelClient struct{}

func (educationExperienceConditionAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	baseYear := 3
	bachelor, bachelorCode := "bachelor", 2
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions: []AICandidateMentionResponse{},
		Experience: AIExperienceResponse{
			Status:          "configured",
			BaseYear:        &baseYear,
			EvidenceSpanIDs: []string{"description:s01"},
			Items: []AIExperienceItemResponse{{
				ScopeType: "base", ScopeKey: "overall", MinYears: 3, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
			}},
			ConditionalQualifications: []AIConditionalQualificationResponse{
				{WhenEducation: "bachelor", ExperienceScope: "base", ExperienceScopeKey: "overall", MinYears: 3, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine"},
				{WhenEducation: "master", ExperienceScope: "base", ExperienceScopeKey: "overall", MinYears: 2, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine"},
			},
			ReviewRecommendation: "routine",
		},
		MinimumEducation: AIMinimumEducationResponse{
			Status: "configured", Level: &bachelor, LevelCode: &bachelorCode, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
		},
		RequestID:                   input.RequestID,
		ResponseSchemaVersion:       AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{},
	})
	if err != nil {
		return AIModelResponse{}, err
	}
	return AIModelResponse{Content: raw, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}

type technologyExperienceAIModelClient struct{}

func (technologyExperienceAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	baseYear := 3
	mention := input.VerifiedTechnologyMentions[0]
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions: []AICandidateMentionResponse{},
		Experience: AIExperienceResponse{
			Status:                    "configured",
			BaseYear:                  &baseYear,
			EvidenceSpanIDs:           []string{mention.SpanID},
			Items:                     []AIExperienceItemResponse{{ScopeType: "technology", ScopeKey: mention.TechnologyKey, MinYears: 3, EvidenceSpanIDs: []string{mention.SpanID}, ReviewRecommendation: "routine"}},
			ConditionalQualifications: []AIConditionalQualificationResponse{},
			ReviewRecommendation:      "routine",
		},
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

type juniorCollegeConditionAIModelClient struct{}

func (juniorCollegeConditionAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	baseYear := 2
	juniorCollege, juniorCollegeCode := "junior_college_or_below", 1
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions: []AICandidateMentionResponse{},
		Experience: AIExperienceResponse{
			Status:          "configured",
			BaseYear:        &baseYear,
			EvidenceSpanIDs: []string{"description:s01"},
			Items: []AIExperienceItemResponse{{
				ScopeType: "base", ScopeKey: "overall", MinYears: 2, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
			}},
			ConditionalQualifications: []AIConditionalQualificationResponse{{
				WhenEducation: "junior_college_or_below", ExperienceScope: "base", ExperienceScopeKey: "overall", MinYears: 2, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
			}},
			ReviewRecommendation: "routine",
		},
		MinimumEducation: AIMinimumEducationResponse{
			Status: "configured", Level: &juniorCollege, LevelCode: &juniorCollegeCode, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine",
		},
		RequestID:                   input.RequestID,
		ResponseSchemaVersion:       AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{},
	})
	if err != nil {
		return AIModelResponse{}, err
	}
	return AIModelResponse{Content: raw, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}
