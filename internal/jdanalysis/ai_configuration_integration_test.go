package jdanalysis

import (
	"context"
	"strings"
	"testing"
)

func TestCandidateAIConfigurationRequiresPublishedTechnologyConfiguration(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	publishedTechnologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-reference-published")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      publishedTechnologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, publishedTechnologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, publishedTechnologyConfigurationID); err != nil {
		t.Fatal(err)
	}

	created, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(publishedTechnologyConfigurationID, "candidate-ai-configuration"))
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Status != "candidate" {
		t.Fatalf("created AI configuration = %#v", created)
	}

	unpublishedTechnologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-reference-candidate")
	_, err = repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(unpublishedTechnologyConfigurationID, "unpublished-candidate-ai-configuration"))
	if err == nil {
		t.Fatal("candidate AI configuration accepted an unpublished technology configuration")
	}
}

func TestPrepareCandidateAIConfigurationReusesExactIdentityAndPreservesPriorResults(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-prepare-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}

	first, err := repository.PrepareCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-prepare-first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.PrepareCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-prepare-first"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || !second.Reused || first.Configuration.ID == "" || second.Configuration.ID != first.Configuration.ID {
		t.Fatalf("candidate preparation reuse = first:%#v second:%#v", first, second)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, first.Configuration.ID); err != nil {
		t.Fatal(err)
	}

	changed := testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-prepare-prompt-v2")
	changed.PromptSHA256 = testSHA256("candidate-ai-prompt-v2")
	third, err := repository.PrepareCandidateAIConfiguration(ctx, changed)
	if err != nil {
		t.Fatal(err)
	}
	if third.Reused || third.Configuration.ID == first.Configuration.ID || third.Configuration.Status != "candidate" {
		t.Fatalf("changed candidate preparation = %#v, want a distinct candidate", third)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, bodyID, third.Configuration.ID); err != nil {
		t.Fatal(err)
	}

	var resultCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM ai_semantic_results
WHERE body_content_version_id = $1
  AND ai_analysis_configuration_id IN ($2, $3)`, bodyID, first.Configuration.ID, third.Configuration.ID).Scan(&resultCount); err != nil {
		t.Fatal(err)
	}
	if resultCount != 2 {
		t.Fatalf("AI results retained across configurations = %d, want 2", resultCount)
	}
}

func TestReadAIConfigurationReturnsOnlyFrozenContractSummary(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-read-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	input := testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-read-configuration")
	prepared, err := repository.PrepareCandidateAIConfiguration(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	read, err := repository.ReadAIConfiguration(ctx, prepared.Configuration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != prepared.Configuration.ID || read.TechnologyConfigurationID != technologyConfigurationID ||
		read.Provider != input.Provider || read.Model != input.Model ||
		read.RequestProfileSHA256 != input.RequestProfileSHA256 || read.PromptSHA256 != input.PromptSHA256 ||
		read.ResponseSchemaVersion != input.ResponseSchemaVersion || read.ResponseSchemaSHA256 != input.ResponseSchemaSHA256 ||
		read.InputContractVersion != input.InputContractVersion || read.InputContractSHA256 != input.InputContractSHA256 ||
		read.ValidatorVersion != input.ValidatorVersion || read.ValidatorSHA256 != input.ValidatorSHA256 ||
		read.PricingProfileVersion != input.PricingProfileVersion || read.PricingProfileSHA256 != input.PricingProfileSHA256 {
		t.Fatalf("frozen AI configuration summary = %#v", read)
	}
}

func TestAISafeAuditPersistsOnlyWhitelistedCodeAndCounts(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-audit-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	prepared, err := repository.PrepareCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-audit-configuration"))
	if err != nil {
		t.Fatal(err)
	}

	if err := repository.RecordAISafeAuditEvent(ctx, AISafeAuditEventInput{
		AIConfigurationID: prepared.Configuration.ID,
		Code:              AIAuditCandidatePrepared,
		LogicalTaskCount:  30,
		AttemptCount:      0,
	}); err != nil {
		t.Fatal(err)
	}

	const untrustedDiagnostic = "synthetic JD body and secret must never enter audit"
	err = repository.RecordAISafeAuditEvent(ctx, AISafeAuditEventInput{
		AIConfigurationID: prepared.Configuration.ID,
		Code:              AIAuditCode(untrustedDiagnostic),
	})
	if err == nil {
		t.Fatal("AI audit accepted an untrusted code")
	}
	if strings.Contains(err.Error(), untrustedDiagnostic) {
		t.Fatalf("AI audit error leaked untrusted diagnostic: %q", err)
	}

	var code string
	var logicalTaskCount, attemptCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT code::text, logical_task_count, attempt_count
FROM ai_analysis_audit_events
WHERE ai_analysis_configuration_id = $1`, prepared.Configuration.ID).Scan(&code, &logicalTaskCount, &attemptCount); err != nil {
		t.Fatal(err)
	}
	if code != string(AIAuditCandidatePrepared) || logicalTaskCount != 30 || attemptCount != 0 {
		t.Fatalf("safe AI audit = code:%s logical:%d attempts:%d", code, logicalTaskCount, attemptCount)
	}
}

func TestAISemanticResultKeepsExecutionAndBusinessStatesSeparate(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-state-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}

	notConfiguredConfiguration, err := repository.PrepareCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-state-not-configured"))
	if err != nil {
		t.Fatal(err)
	}
	notConfiguredResult, err := repository.EnsureAISemanticResult(ctx, bodyID, notConfiguredConfiguration.Configuration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteAISemanticResult(ctx, notConfiguredResult.ID, "not_configured"); err != nil {
		t.Fatal(err)
	}

	uncertainConfiguration, err := repository.PrepareCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-state-uncertain"))
	if err != nil {
		t.Fatal(err)
	}
	uncertainResult, err := repository.EnsureAISemanticResult(ctx, bodyID, uncertainConfiguration.Configuration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteAISemanticResult(ctx, uncertainResult.ID, "uncertain"); err != nil {
		t.Fatal(err)
	}

	notAnalyzedConfiguration, err := repository.PrepareCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-state-not-analyzed"))
	if err != nil {
		t.Fatal(err)
	}
	notAnalyzedResult, err := repository.EnsureAISemanticResult(ctx, bodyID, notAnalyzedConfiguration.Configuration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.FailAISemanticResult(ctx, notAnalyzedResult.ID, AISafeErrorInputTooLarge); err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteAISemanticResult(ctx, notAnalyzedResult.ID, "not_analyzed"); err == nil {
		t.Fatal("completed AI result accepted the not_analyzed business state")
	}

	rows, err := repository.pool.Query(ctx, `
SELECT execution_status::text, semantic_status::text, safe_error_code
FROM ai_semantic_results
WHERE id IN ($1, $2, $3)
ORDER BY id`, notConfiguredResult.ID, uncertainResult.ID, notAnalyzedResult.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var executionStatus, semanticStatus string
		var safeErrorCode *string
		if err := rows.Scan(&executionStatus, &semanticStatus, &safeErrorCode); err != nil {
			t.Fatal(err)
		}
		seen[executionStatus+":"+semanticStatus] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !seen["complete:not_configured"] || !seen["complete:uncertain"] || !seen["failed_safely:not_analyzed"] {
		t.Fatalf("separate AI result states = %#v", seen)
	}
}

func TestAISemanticResultHasOneIdentityAndLeavesDeterministicResultsUntouched(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-result-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-result-configuration"))
	if err != nil {
		t.Fatal(err)
	}

	first, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || !second.Reused || first.ID == "" || second.ID != first.ID ||
		first.ExecutionStatus != "pending" || first.SemanticStatus != "not_analyzed" {
		t.Fatalf("AI result reuse = first:%#v second:%#v", first, second)
	}

	var deterministicResults, semanticResults int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM technology_analysis_results),
  (SELECT count(*) FROM ai_semantic_results)`).Scan(&deterministicResults, &semanticResults); err != nil {
		t.Fatal(err)
	}
	if deterministicResults != 1 || semanticResults != 1 {
		t.Fatalf("result families = deterministic:%d semantic:%d", deterministicResults, semanticResults)
	}
}

func TestAIConfigurationFreezesContractInputsAndRestrictsLifecycleTransitions(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-lifecycle-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-lifecycle-configuration"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_configurations
SET prompt_sha256 = $2
WHERE id = $1`, aiConfiguration.ID, testSHA256("replacement-prompt")); err == nil {
		t.Fatal("AI configuration accepted a changed prompt digest")
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_configurations
SET status = 'comparison'
WHERE id = $1`, aiConfiguration.ID); err == nil {
		t.Fatal("AI configuration accepted an invalid candidate-to-comparison transition")
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_configurations
SET status = 'published', published_at = now()
WHERE id = $1`, aiConfiguration.ID); err != nil {
		t.Fatalf("publish qwen-plus candidate: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_configurations
SET status = 'candidate', published_at = NULL
WHERE id = $1`, aiConfiguration.ID); err == nil {
		t.Fatal("AI configuration accepted a published-to-candidate transition")
	}
	if _, err := repository.pool.Exec(ctx, `DELETE FROM ai_analysis_configurations WHERE id = $1`, aiConfiguration.ID); err == nil {
		t.Fatal("AI configuration accepted deletion")
	}
}

func TestAIWorkLedgerBindsRunsAttemptsAndUsageToOneConfiguration(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-work-ledger-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-work-ledger-configuration"))
	if err != nil {
		t.Fatal(err)
	}

	var runID, workID, attemptID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_runs (ai_analysis_configuration_id, trigger)
VALUES ($1, 'trial')
RETURNING id::text`, aiConfiguration.ID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')
RETURNING id::text`, aiConfiguration.ID, bodyID).Scan(&workID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_work_items (ai_analysis_configuration_id, body_content_version_id, trigger)
VALUES ($1, $2, 'trial')`, aiConfiguration.ID, bodyID); err == nil {
		t.Fatal("AI work ledger accepted a duplicate body/configuration identity")
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_attempts (
    ai_analysis_configuration_id, work_item_id, run_id, attempt_number,
    input_token_limit, output_token_limit
) VALUES ($1, $2, $3, 1, 10000, 2500)
RETURNING id::text`, aiConfiguration.ID, workID, runID).Scan(&attemptID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_usage_records (
    attempt_id, input_tokens, output_tokens, estimated_charge_rmb, actual_charge_rmb
) VALUES ($1, 100, 20, 0.100000, 0.050000)`, attemptID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_usage_records (
    attempt_id, input_tokens, output_tokens, estimated_charge_rmb, actual_charge_rmb
) VALUES ($1, -1, 0, 0, 0)`, attemptID); err == nil {
		t.Fatal("AI usage ledger accepted a negative input token count")
	}
}

func TestAISemanticArtifactsKeepEvidenceBranchesRequirementsCandidatesReviewsAndAttestationsLinked(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)

	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-artifact-reference")
	if _, err := repository.PublishCompleteResult(ctx, ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      technologyConfigurationID,
	}, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2}},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := (Processor{Repository: repository}).ProcessPending(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.PromoteCandidateConfiguration(ctx, technologyConfigurationID); err != nil {
		t.Fatal(err)
	}
	aiConfiguration, err := repository.CreateCandidateAIConfiguration(ctx, testCandidateAIConfigurationInput(technologyConfigurationID, "candidate-ai-artifact-configuration"))
	if err != nil {
		t.Fatal(err)
	}
	semanticResult, err := repository.EnsureAISemanticResult(ctx, bodyID, aiConfiguration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_semantic_results
SET execution_status = 'complete', semantic_status = 'configured', completed_at = now()
WHERE id = $1`, semanticResult.ID); err != nil {
		t.Fatal(err)
	}

	var experienceArtifactID, branchArtifactID, groupArtifactID, optionArtifactID, candidateArtifactID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'experience_summary', 'configured', 'routine')
RETURNING id::text`, semanticResult.ID).Scan(&experienceArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_semantic_evidence (
    semantic_artifact_id, span_id, source_field, normalization_version, normalized_field_sha256,
    codepoint_start, codepoint_end
) VALUES ($1, 'description:s01', 'description', 'jd-body-v1', $2, 0, 2)`, experienceArtifactID, testSHA256("Go, Go")); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_experience_summaries (id, status, base_year)
VALUES ($1, 'configured', 3)`, experienceArtifactID); err != nil {
		t.Fatal(err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'condition_branch', 'configured', 'required_review')
RETURNING id::text`, semanticResult.ID).Scan(&branchArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_semantic_condition_branches (
    id, experience_summary_id, education_level, minimum_experience_years, branch_order
) VALUES ($1, $2, 'bachelor', 3, 1)`, branchArtifactID, experienceArtifactID); err != nil {
		t.Fatal(err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'requirement_group', 'configured', 'routine')
RETURNING id::text`, semanticResult.ID).Scan(&groupArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_technology_requirement_groups (
    id, role, necessity, operator, min_satisfied
) VALUES ($1, 'qualification', 'required', 'at_least', 1)`, groupArtifactID); err != nil {
		t.Fatal(err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'requirement_option', 'routine', 'routine')
RETURNING id::text`, semanticResult.ID).Scan(&optionArtifactID); err == nil {
		t.Fatal("AI artifact accepted an invalid semantic status")
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'requirement_option', 'configured', 'routine')
RETURNING id::text`, semanticResult.ID).Scan(&optionArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_technology_requirement_options (
    id, requirement_group_id, technology_identity_id, proficiency
) VALUES ($1, $2, $3, 'p3')`, optionArtifactID, groupArtifactID, technologyID); err != nil {
		t.Fatal(err)
	}

	var dictionaryCandidateID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO technology_dictionary_candidates (
    dictionary_version_id, candidate_text, candidate_summary
) VALUES ($1, 'synthetic-candidate', 'synthetic candidate')
RETURNING id::text`, dictionaryID).Scan(&dictionaryCandidateID); err != nil {
		t.Fatal(err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_artifacts (ai_semantic_result_id, artifact_kind, semantic_status, review_tier)
VALUES ($1, 'candidate_technology', 'configured', 'required_review')
RETURNING id::text`, semanticResult.ID).Scan(&candidateArtifactID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_candidate_technology_mentions (id, dictionary_candidate_id)
VALUES ($1, $2)`, candidateArtifactID, dictionaryCandidateID); err != nil {
		t.Fatal(err)
	}
	var reviewID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_semantic_reviews (
    ai_semantic_result_id, semantic_artifact_id, reviewer, verdict
) VALUES ($1, $2, 'remote-reviewer', 'accepted')
RETURNING id::text`, semanticResult.ID, groupArtifactID).Scan(&reviewID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_configurations
SET status = 'published', published_at = now()
WHERE id = $1`, aiConfiguration.ID); err != nil {
		t.Fatal(err)
	}
	var attestationID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO ai_analysis_release_attestations (
    ai_analysis_configuration_id, gate_version, publisher, attestation_sha256,
    trial_body_count, fully_reviewed_trial_body_count, major_error_count
) VALUES ($1, 'jd-ai-release-gate-v1', 'FAN', $2, 30, 30, 0)
RETURNING id::text`, aiConfiguration.ID, testSHA256("synthetic-attestation")).Scan(&attestationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `UPDATE ai_semantic_reviews SET verdict = 'minor_error' WHERE id = $1`, reviewID); err == nil {
		t.Fatal("AI semantic review accepted mutation")
	}
	if _, err := repository.pool.Exec(ctx, `UPDATE ai_analysis_release_attestations SET publisher = 'other' WHERE id = $1`, attestationID); err == nil {
		t.Fatal("AI analysis release attestation accepted mutation")
	}
}

func testCandidateAIConfigurationInput(technologyConfigurationID, configurationSeed string) CandidateAIConfigurationInput {
	pricing := QwenPlusTrialPricingProfile()
	return CandidateAIConfigurationInput{
		TechnologyConfigurationID: technologyConfigurationID,
		Provider:                  "bailian",
		Model:                     "qwen-plus",
		ConfigurationSHA256:       testSHA256(configurationSeed),
		RequestProfileSHA256:      testSHA256("candidate-ai-request-profile"),
		PromptSHA256:              testSHA256("candidate-ai-prompt"),
		ResponseSchemaVersion:     "jd-ai-semantic-schema-v1",
		ResponseSchemaSHA256:      testSHA256("candidate-ai-response-schema"),
		InputContractVersion:      "jd-ai-input-contract-v1",
		InputContractSHA256:       testSHA256("candidate-ai-input-contract"),
		ValidatorVersion:          "jd-ai-validator-v1",
		ValidatorSHA256:           testSHA256("candidate-ai-validator"),
		PricingProfileVersion:     pricing.Version,
		PricingProfileSHA256:      pricing.SHA256(),
		CreatedBy:                 "test",
	}
}
