package jdanalysis

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAISemanticProcessorPersistsSupportedSummaryOnlyExperience(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "summary-only-experience-reference")
	aiConfiguration := createV1CandidateAIConfiguration(t, repository, technologyConfigurationID, "summary-only-experience-candidate")
	prepareAIWork(t, repository, bodyID, aiConfiguration.ID)

	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: summaryOnlyExperienceAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var status string
	var baseYear, itemCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT summary.status::text, summary.base_year::integer,
       (SELECT count(*) FROM ai_experience_requirement_items AS item WHERE item.experience_summary_id = summary.id)
FROM ai_experience_summaries AS summary`).Scan(&status, &baseYear, &itemCount); err != nil {
		t.Fatal(err)
	}
	if status != "configured" || baseYear != 3 || itemCount != 0 {
		t.Fatalf("summary-only experience = status:%s base:%d items:%d, want configured/3/0", status, baseYear, itemCount)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "complete", "complete", "configured")
}

func TestAISemanticProcessorPersistsSingleSourceDomainAlongsideOverallExperience(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "single-domain-experience-reference")
	aiConfiguration := createV1CandidateAIConfiguration(t, repository, technologyConfigurationID, "single-domain-experience-candidate")
	prepareAIWork(t, repository, bodyID, aiConfiguration.ID)

	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: singleDomainExperienceAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var overallItems, domainItems int
	var domainKey string
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*) FILTER (WHERE subject_kind = 'overall'),
       count(*) FILTER (WHERE subject_kind = 'domain'),
       COALESCE(max(domain_key) FILTER (WHERE subject_kind = 'domain'), '')
FROM ai_experience_requirement_items`).Scan(&overallItems, &domainItems, &domainKey); err != nil {
		t.Fatal(err)
	}
	if overallItems != 1 || domainItems != 1 || domainKey != "Go" {
		t.Fatalf("persisted experience scope = overall:%d domain:%d key:%q, want 1/1/Go", overallItems, domainItems, domainKey)
	}
}

func TestAISemanticProcessorPersistsIndependentCPlusPlusAndPythonRequirements(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, cppID, pythonID, cppRuleID, pythonRuleID := insertSyntheticRequirementRelationPrerequisites(t, repository)
	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "independent-cpp-python-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{
		{TechnologyIdentityID: cppID, Evidence: []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: cppRuleID, Start: 0, End: 3}}},
		{TechnologyIdentityID: pythonID, Evidence: []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: pythonRuleID, Start: 4, End: 10}}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	aiConfiguration := createV1CandidateAIConfiguration(t, repository, technologyConfigurationID, "independent-cpp-python-candidate")
	prepareAIWork(t, repository, bodyID, aiConfiguration.ID)

	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: independentCPlusPlusPythonAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	rows, err := repository.pool.Query(ctx, `
SELECT identity.technology_key, requirement.operator::text, requirement.min_satisfied, count(*) OVER (PARTITION BY requirement.id)
FROM ai_technology_requirement_groups AS requirement
JOIN ai_technology_requirement_options AS option ON option.requirement_group_id = requirement.id
JOIN technology_identities AS identity ON identity.id = option.technology_identity_id
ORDER BY identity.technology_key`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type requirementRow struct {
		technologyKey string
		operator      string
		minSatisfied  int
		optionCount   int
	}
	got := make([]requirementRow, 0, 2)
	for rows.Next() {
		var row requirementRow
		if err := rows.Scan(&row.technologyKey, &row.operator, &row.minSatisfied, &row.optionCount); err != nil {
			t.Fatal(err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (requirementRow{technologyKey: "cpp", operator: "at_least", minSatisfied: 1, optionCount: 2}) || got[1] != (requirementRow{technologyKey: "python", operator: "at_least", minSatisfied: 1, optionCount: 2}) {
		t.Fatalf("persisted model-supplied C++/Python group = %#v", got)
	}
}

func TestAISemanticProcessorCandidateIssuesKeepVerifiedFactsConfigured(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	const (
		description = "具备3年以上经验，要求Go和OriginalSDK。"
		jobRequire  = "本科及以上。"
		addition    = ""
	)
	if _, err := repository.pool.Exec(ctx, `
UPDATE job_body_content_versions
SET description_text = $2,
    job_require_text = $3,
    addition_text = $4,
    description_sha256 = $5,
    job_require_sha256 = $6,
    addition_sha256 = $7,
    body_sha256 = $8
WHERE id = $1`, bodyID, description, jobRequire, addition,
		testSHA256(description), testSHA256(jobRequire), testSHA256(addition), testSHA256(description+"|"+jobRequire+"|"+addition)); err != nil {
		t.Fatal(err)
	}
	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "candidate-issues-do-not-block-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 11, End: 13}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	aiConfiguration := createV3CandidateAIConfiguration(t, repository, technologyConfigurationID, "candidate-issues-do-not-block")
	prepareAIWork(t, repository, bodyID, aiConfiguration.ID)

	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: candidateIssueTolerantAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "complete", "complete", "configured")

	var baseYear, educationCount, requirementOptionCount, dictionaryCandidateCount, unresolvedCount int
	var sourceTerm string
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT base_year::integer FROM ai_experience_summaries),
  (SELECT count(*) FROM ai_minimum_education_requirements WHERE status = 'configured'),
  (SELECT count(*) FROM ai_technology_requirement_options),
  (SELECT count(*) FROM technology_dictionary_candidates),
  (SELECT count(*) FROM ai_pending_unresolved_source_terms),
  (SELECT source_term FROM ai_pending_unresolved_source_terms)
`).Scan(&baseYear, &educationCount, &requirementOptionCount, &dictionaryCandidateCount, &unresolvedCount, &sourceTerm); err != nil {
		t.Fatal(err)
	}
	if baseYear != 3 || educationCount != 1 || requirementOptionCount != 1 || dictionaryCandidateCount != 0 || unresolvedCount != 1 || sourceTerm != "OriginalSDK" {
		t.Fatalf("candidate tolerance persistence = base:%d education:%d options:%d dictionary:%d unresolved:%d source:%q", baseYear, educationCount, requirementOptionCount, dictionaryCandidateCount, unresolvedCount, sourceTerm)
	}
}

func TestAISemanticProcessorPersistsModelRelationWithoutSemanticPolicyGate(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "model-relation-without-policy-gate-reference")
	aiConfiguration := createV1CandidateAIConfiguration(t, repository, technologyConfigurationID, "model-relation-without-policy-gate-candidate")
	prepareAIWork(t, repository, bodyID, aiConfiguration.ID)

	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: structurallyValidModelRelationAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var role string
	var minimumSatisfied, optionCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT requirement.role::text, requirement.min_satisfied, count(option.id)
FROM ai_technology_requirement_groups AS requirement
LEFT JOIN ai_technology_requirement_options AS option ON option.requirement_group_id = requirement.id
GROUP BY requirement.role, requirement.min_satisfied`).Scan(&role, &minimumSatisfied, &optionCount); err != nil {
		t.Fatal(err)
	}
	if role != "responsibility" || minimumSatisfied != 2 || optionCount != 1 {
		t.Fatalf("persisted model relation = role:%s min:%d options:%d, want responsibility/2/1", role, minimumSatisfied, optionCount)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "complete", "complete", "configured")
}

func TestAISemanticProcessorSanitizesInvalidReferencesWithoutCrossBodyEvidence(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "local-reference-sanitation-reference")
	aiConfiguration := createV1CandidateAIConfiguration(t, repository, technologyConfigurationID, "local-reference-sanitation-candidate")
	prepareAIWork(t, repository, bodyID, aiConfiguration.ID)

	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: localReferenceSanitationAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "complete", "complete", "configured")

	var optionCount, foreignEvidenceCount, validEvidenceCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM ai_technology_requirement_options),
  (SELECT count(*)
     FROM ai_semantic_evidence
    WHERE span_id = 'foreign-span'
       OR normalized_field_sha256 <> $1),
  (SELECT count(*)
     FROM ai_semantic_evidence
    WHERE normalized_field_sha256 = $1)
`, testSHA256("Go, Go")).Scan(&optionCount, &foreignEvidenceCount, &validEvidenceCount); err != nil {
		t.Fatal(err)
	}
	if optionCount != 1 || foreignEvidenceCount != 0 || validEvidenceCount == 0 {
		t.Fatalf("local reference sanitation = options:%d foreign_evidence:%d valid_evidence:%d, want 1/0/nonzero", optionCount, foreignEvidenceCount, validEvidenceCount)
	}
}

func TestAISemanticProcessorPersistsStructurallyValidModelStatesWithoutSemanticGate(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "model-state-without-policy-gate-reference")
	aiConfiguration := createV1CandidateAIConfiguration(t, repository, technologyConfigurationID, "model-state-without-policy-gate-candidate")
	prepareAIWork(t, repository, bodyID, aiConfiguration.ID)

	if _, err := (AISemanticProcessor{Repository: repository, Runner: AIModelAttemptRunner{Client: structurallyValidModelStateAIModelClient{}}}).ProcessPending(ctx, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}

	var experienceStatus, educationStatus, educationLevel string
	var baseYear int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT status::text FROM ai_experience_summaries),
  (SELECT base_year::integer FROM ai_experience_summaries),
  (SELECT status::text FROM ai_minimum_education_requirements),
  (SELECT minimum_level::text FROM ai_minimum_education_requirements)
`).Scan(&experienceStatus, &baseYear, &educationStatus, &educationLevel); err != nil {
		t.Fatal(err)
	}
	if experienceStatus != "not_configured" || baseYear != 3 || educationStatus != "uncertain" || educationLevel != "bachelor" {
		t.Fatalf("persisted model states = experience:%s/%d education:%s/%s, want not_configured/3 uncertain/bachelor", experienceStatus, baseYear, educationStatus, educationLevel)
	}
	assertAISemanticWorkState(t, repository, aiConfiguration.ID, bodyID, "complete", "complete", "uncertain")
}

func createV1CandidateAIConfiguration(t *testing.T, repository *Repository, technologyConfigurationID, seed string) AIConfiguration {
	t.Helper()
	input := testCandidateAIConfigurationInput(technologyConfigurationID, seed)
	input.ResponseSchemaVersion = AIResponseSchemaVersionV1
	input.ResponseSchemaSHA256 = AIResponseSchemaV1SHA256()
	configuration, err := repository.CreateCandidateAIConfiguration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return configuration
}

func createV3CandidateAIConfiguration(t *testing.T, repository *Repository, technologyConfigurationID, seed string) AIConfiguration {
	t.Helper()
	input := testCandidateAIConfigurationInput(technologyConfigurationID, seed)
	input.PromptSHA256 = AISystemPromptV3SHA256()
	input.ResponseSchemaVersion = AIResponseSchemaVersionV3
	input.ResponseSchemaSHA256 = AIResponseSchemaV3SHA256()
	input.ValidatorVersion = "ai-semantic-validator-v3"
	input.ValidatorSHA256 = sha256Hex(input.ValidatorVersion)
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	configuration, err := repository.CreateCandidateAIConfiguration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return configuration
}

func prepareAIWork(t *testing.T, repository *Repository, bodyID, configurationID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, configurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')`, configurationID, bodyID); err != nil {
		t.Fatal(err)
	}
}

func insertSyntheticRequirementRelationPrerequisites(t *testing.T, repository *Repository) (bodyID, dictionaryID, cppID, pythonID, cppRuleID, pythonRuleID string) {
	t.Helper()
	ctx := context.Background()
	bodyID, dictionaryID, _, _ = insertSyntheticAnalysisPrerequisites(t, repository)
	const description = "C++。Python。"
	if _, err := repository.pool.Exec(ctx, `
UPDATE job_body_content_versions
SET description_text = $2,
    description_sha256 = $3,
    body_sha256 = $4
WHERE id = $1`, bodyID, description, testSHA256(description), testSHA256(description+"||")); err != nil {
		t.Fatal(err)
	}
	cppID = "00000000-0000-4000-8000-000000000506"
	pythonID = "00000000-0000-4000-8000-000000000507"
	cppRuleID = "00000000-0000-4000-8000-000000000508"
	pythonRuleID = "00000000-0000-4000-8000-000000000509"
	for _, identity := range []struct {
		id, key, canonical string
	}{
		{cppID, "cpp", "C++"},
		{pythonID, "python", "Python"},
	} {
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_identities (id, technology_key, canonical_name, technology_kind)
VALUES ($1, $2, $3, 'programming-language')`, identity.id, identity.key, identity.canonical); err != nil {
			t.Fatal(err)
		}
	}
	for _, rule := range []struct {
		id, identityID, key, alias string
	}{
		{cppRuleID, cppID, "cpp.01", "C++"},
		{pythonRuleID, pythonID, "python.01", "Python"},
	} {
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_dictionary_rules (
    id, dictionary_version_id, technology_identity_id, rule_key, alias, state,
    case_mode, boundary_mode, rule_summary, approved_by, approved_at
) VALUES ($1, $2, $3, $4, $5, 'enabled', 'sensitive', 'exact', 'synthetic test rule', 'reviewer', now())`, rule.id, dictionaryID, rule.identityID, rule.key, rule.alias); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_dictionary_rule_decisions (rule_id, decision, reviewer, rationale)
VALUES ($1, 'approved_enabled', 'reviewer', 'synthetic approval')`, rule.id); err != nil {
			t.Fatal(err)
		}
	}
	return bodyID, dictionaryID, cppID, pythonID, cppRuleID, pythonRuleID
}

type summaryOnlyExperienceAIModelClient struct{}

func (summaryOnlyExperienceAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	baseYear := 3
	return syntheticSemanticAIModelResponse(input, AIExperienceResponse{
		Status:                    "configured",
		BaseYear:                  &baseYear,
		Items:                     []AIExperienceItemResponse{},
		ConditionalQualifications: []AIConditionalQualificationResponse{},
		EvidenceSpanIDs:           []string{"description:s01"},
		ReviewRecommendation:      "routine",
	}, nil)
}

type singleDomainExperienceAIModelClient struct{}

func (singleDomainExperienceAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	baseYear := 3
	return syntheticSemanticAIModelResponse(input, AIExperienceResponse{
		Status:   "configured",
		BaseYear: &baseYear,
		Items: []AIExperienceItemResponse{
			{ScopeType: "base", ScopeKey: "overall", MinYears: 3, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine"},
			{ScopeType: "domain", ScopeKey: "Go", MinYears: 3, EvidenceSpanIDs: []string{"description:s01"}, ReviewRecommendation: "routine"},
		},
		ConditionalQualifications: []AIConditionalQualificationResponse{},
		EvidenceSpanIDs:           []string{"description:s01"},
		ReviewRecommendation:      "routine",
	}, nil)
}

type independentCPlusPlusPythonAIModelClient struct{}

func (independentCPlusPlusPythonAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	mentions := make(map[string]VerifiedTechnologyMention, len(input.VerifiedTechnologyMentions))
	for _, mention := range input.VerifiedTechnologyMentions {
		mentions[mention.TechnologyKey] = mention
	}
	cpp, hasCPP := mentions["cpp"]
	python, hasPython := mentions["python"]
	if !hasCPP || !hasPython {
		return AIModelResponse{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	operator := "at_least"
	minimumOne := 1
	return syntheticSemanticAIModelResponse(input, AIExperienceResponse{Status: "not_configured", Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{}, EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"}, []AITechnologyRequirementGroupResponse{{
		LocalID: "merged-independent-requirements", Role: "qualification", Necessity: "required", Operator: &operator, MinSatisfied: &minimumOne,
		Options: []AITechnologyRequirementOptionResponse{
			{TechnologyKey: cpp.TechnologyKey, MentionEvidenceIDs: []string{cpp.MentionEvidenceID}},
			{TechnologyKey: python.TechnologyKey, MentionEvidenceIDs: []string{python.MentionEvidenceID}},
		},
		EvidenceSpanIDs: []string{cpp.SpanID, python.SpanID}, ReviewRecommendation: "routine",
	}})
}

type structurallyValidModelRelationAIModelClient struct{}

func (structurallyValidModelRelationAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	if len(input.VerifiedTechnologyMentions) != 1 {
		return AIModelResponse{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	minimumTwo := 2
	atLeast := "at_least"
	mention := input.VerifiedTechnologyMentions[0]
	return syntheticSemanticAIModelResponse(input,
		AIExperienceResponse{Status: "not_configured", Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{}, EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		[]AITechnologyRequirementGroupResponse{{
			LocalID: "model-relation", Role: "responsibility", Necessity: "required", Operator: &atLeast, MinSatisfied: &minimumTwo,
			Options:         []AITechnologyRequirementOptionResponse{{TechnologyKey: mention.TechnologyKey, MentionEvidenceIDs: []string{mention.MentionEvidenceID}}},
			EvidenceSpanIDs: []string{mention.SpanID}, ReviewRecommendation: "routine",
		}},
	)
}

type localReferenceSanitationAIModelClient struct{}

func (localReferenceSanitationAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	if len(input.Fields) == 0 || len(input.Fields[0].Spans) == 0 || len(input.VerifiedTechnologyMentions) != 1 {
		return AIModelResponse{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	baseYear, minimumOne := 3, 1
	atLeast := "at_least"
	spanID := input.Fields[0].Spans[0].SpanID
	mention := input.VerifiedTechnologyMentions[0]
	return syntheticSemanticAIModelResponse(input,
		AIExperienceResponse{
			Status: "configured", BaseYear: &baseYear,
			Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{},
			EvidenceSpanIDs: []string{"foreign-span", spanID}, ReviewRecommendation: "routine",
		},
		[]AITechnologyRequirementGroupResponse{{
			LocalID: "locally-sanitized", Role: "qualification", Necessity: "required", Operator: &atLeast, MinSatisfied: &minimumOne,
			Options: []AITechnologyRequirementOptionResponse{{
				TechnologyKey: mention.TechnologyKey, MentionEvidenceIDs: []string{"foreign-evidence", mention.MentionEvidenceID},
			}},
			EvidenceSpanIDs: []string{"foreign-span", spanID}, ReviewRecommendation: "routine",
		}},
	)
}

type structurallyValidModelStateAIModelClient struct{}

func (structurallyValidModelStateAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	baseYear, bachelorCode := 3, 2
	bachelor := "bachelor"
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions: []AICandidateMentionResponse{},
		Experience: AIExperienceResponse{
			Status: "not_configured", BaseYear: &baseYear,
			Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{},
			EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine",
		},
		MinimumEducation: AIMinimumEducationResponse{
			Status: "uncertain", Level: &bachelor, LevelCode: &bachelorCode,
			EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine",
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

type candidateIssueTolerantAIModelClient struct{}

func (candidateIssueTolerantAIModelClient) Analyze(_ context.Context, input AIModelInput) (AIModelResponse, error) {
	var experienceSpanID, educationSpanID, candidateSpanID string
	for _, field := range input.Fields {
		for _, span := range field.Spans {
			switch {
			case field.SourceField == SourceFieldDescription && strings.Contains(span.Text, "3年以上"):
				experienceSpanID = span.SpanID
			case field.SourceField == SourceFieldDescription && strings.Contains(span.Text, "OriginalSDK"):
				candidateSpanID = span.SpanID
			case field.SourceField == SourceFieldJobRequire && strings.Contains(span.Text, "本科"):
				educationSpanID = span.SpanID
			}
		}
	}
	if experienceSpanID == "" || educationSpanID == "" || candidateSpanID == "" || len(input.VerifiedTechnologyMentions) != 1 {
		return AIModelResponse{}, NewSafeAIError(AISafeErrorInvalidEvidence, nil)
	}
	baseYear, bachelorCode, minimumOne := 3, 2, 1
	bachelor, operator := "bachelor", "at_least"
	mention := input.VerifiedTechnologyMentions[0]
	raw, err := json.Marshal(AISemanticResponse{
		CandidateMentions: []AICandidateMentionResponse{{
			CandidateText: "Original SDK", SourceTerm: "OriginalSDK", SpanID: candidateSpanID,
			Role: "qualification", Necessity: "required", ReviewRecommendation: "required_review",
		}},
		Experience: AIExperienceResponse{
			Status: "configured", BaseYear: &baseYear, Items: []AIExperienceItemResponse{}, ConditionalQualifications: []AIConditionalQualificationResponse{},
			EvidenceSpanIDs: []string{experienceSpanID}, ReviewRecommendation: "routine",
		},
		MinimumEducation: AIMinimumEducationResponse{
			Status: "configured", Level: &bachelor, LevelCode: &bachelorCode, EvidenceSpanIDs: []string{educationSpanID}, ReviewRecommendation: "routine",
		},
		RequestID:             input.RequestID,
		ResponseSchemaVersion: AIResponseSchemaVersionV3,
		TechnologyRequirementGroups: []AITechnologyRequirementGroupResponse{{
			LocalID: "required-go", Role: "qualification", Necessity: "required", Operator: &operator, MinSatisfied: &minimumOne,
			Options:         []AITechnologyRequirementOptionResponse{{TechnologyKey: mention.TechnologyKey, MentionEvidenceIDs: []string{mention.MentionEvidenceID}}},
			EvidenceSpanIDs: []string{mention.SpanID}, ReviewRecommendation: "routine",
		}},
	})
	if err != nil {
		return AIModelResponse{}, err
	}
	return AIModelResponse{Content: raw, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}

func syntheticSemanticAIModelResponse(input AIModelInput, experience AIExperienceResponse, groups []AITechnologyRequirementGroupResponse) (AIModelResponse, error) {
	if groups == nil {
		groups = []AITechnologyRequirementGroupResponse{}
	}
	response := AISemanticResponse{
		CandidateMentions:           []AICandidateMentionResponse{},
		Experience:                  experience,
		MinimumEducation:            AIMinimumEducationResponse{Status: "not_configured", EvidenceSpanIDs: []string{}, ReviewRecommendation: "routine"},
		RequestID:                   input.RequestID,
		ResponseSchemaVersion:       AIResponseSchemaVersionV1,
		TechnologyRequirementGroups: groups,
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return AIModelResponse{}, err
	}
	return AIModelResponse{Content: raw, Usage: AIModelUsage{InputTokens: 10, OutputTokens: 10, TotalTokens: 20}}, nil
}
