package jdanalysis

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestPrepareAIPlusTrialFreezesFiveBodyIDsAndOneMemberBudget(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-ai-trial-reference")
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
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 6)

	plus := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-trial-plus")
	plus.PricingProfileVersion = QwenPlusTrialPricingProfile().Version
	plus.PricingProfileSHA256 = QwenPlusTrialPricingProfile().SHA256()
	prepared, err := repository.PrepareAIPlusTrial(ctx, AIPlusTrialPreparationInput{Plus: plus})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Plus.ID == "" || prepared.Plus.Status != "candidate" ||
		prepared.PlusQueued != aiTrialSampleSize || len(prepared.BodyContentVersionIDs) != aiTrialSampleSize {
		t.Fatalf("trial preparation = %#v", prepared)
	}
	assertTrialWorkExactlyMatchesSample(t, repository, prepared.Plus.ID, prepared.BodyContentVersionIDs)
	var budgetGroupCount, budgetMemberCount, maximumLogicalTasks, maximumAttempts int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*)
   FROM ai_analysis_trial_budget_groups AS budget_group
   JOIN ai_analysis_trial_budget_group_members AS member
     ON member.ai_analysis_trial_budget_group_id = budget_group.id
    AND member.ai_analysis_configuration_id = $1),
  (SELECT count(*)
   FROM ai_analysis_trial_budget_group_members
   WHERE ai_analysis_trial_budget_group_id = $2),
  (SELECT maximum_logical_task_count FROM ai_analysis_trial_budget_groups WHERE id = $2),
  (SELECT maximum_physical_attempt_count FROM ai_analysis_trial_budget_groups WHERE id = $2)`, prepared.Plus.ID, prepared.BudgetGroupID).Scan(&budgetGroupCount, &budgetMemberCount, &maximumLogicalTasks, &maximumAttempts); err != nil {
		t.Fatal(err)
	}
	if budgetGroupCount != 1 || budgetMemberCount != 1 || maximumLogicalTasks != 5 || maximumAttempts != 15 {
		t.Fatalf("Plus trial budget = groups:%d members:%d logical:%d attempts:%d, want 1/1/5/15", budgetGroupCount, budgetMemberCount, maximumLogicalTasks, maximumAttempts)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_publication_state (singleton)
VALUES (true)
ON CONFLICT (singleton) DO NOTHING`); err != nil {
		t.Fatal(err)
	}

	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 1)
	reused, err := repository.PrepareAIPlusTrial(ctx, AIPlusTrialPreparationInput{Plus: plus})
	if err != nil {
		t.Fatal(err)
	}
	if !reused.Reused || reused.Plus.ID != prepared.Plus.ID || reused.PlusQueued != 0 ||
		!equalStrings(reused.BodyContentVersionIDs, prepared.BodyContentVersionIDs) {
		t.Fatalf("reused trial preparation = %#v, initial = %#v", reused, prepared)
	}
	assertTrialWorkExactlyMatchesSample(t, repository, reused.Plus.ID, prepared.BodyContentVersionIDs)
}

func TestPrepareControlledAITrialUsesOnlyFixedProfilesAndPublishedBaseline(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-controlled-ai-trial-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 6)

	first, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if first.Plus.Status != "candidate" || first.PlusQueued != aiTrialSampleSize || len(first.BodyContentVersionIDs) != aiTrialSampleSize {
		t.Fatalf("controlled AI trial preparation = %#v", first)
	}
	plus, err := repository.ReadAIConfiguration(ctx, first.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plus.TechnologyConfigurationID != technologyConfigurationID || plus.Model != "qwen-plus" || plus.PromptSHA256 != AISystemPromptV4SHA256() || plus.ResponseSchemaVersion != AIResponseSchemaVersionV4 || plus.ResponseSchemaSHA256 != AIResponseSchemaV4SHA256() || plus.ValidatorVersion != AIValidatorVersionV4 || plus.PricingProfileVersion != QwenPlusTrialPricingProfile().Version || plus.PricingProfileSHA256 != QwenPlusTrialPricingProfile().SHA256() {
		t.Fatalf("controlled AI Plus configuration = %#v", plus)
	}
	var attempts int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM ai_analysis_attempts
WHERE ai_analysis_configuration_id = $1`, first.Plus.ID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("trial preparation made %d model attempt ledger row(s), want zero before a separately confirmed run", attempts)
	}

	second, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.Plus.ID != first.Plus.ID || second.PlusQueued != 0 {
		t.Fatalf("controlled AI trial reuse = %#v, first = %#v", second, first)
	}
	plusID, err := repository.ControlledAIPlusTrialConfigurationID(ctx, "llm-mrolescope")
	if err != nil || plusID != first.Plus.ID {
		t.Fatalf("controlled Plus configuration = %q, %v; want %q, nil", plusID, err, first.Plus.ID)
	}
	if _, err := repository.ControlledAIPlusTrialConfigurationID(ctx, "invalid workspace"); err == nil {
		t.Fatal("invalid workspace resolved to a trial configuration")
	}
}

func TestPrepareControlledAIOutputAuditDiagnosticFreezesFivePlusWorkItems(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-audit-diagnostic-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	if _, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope"); err != nil {
		t.Fatal(err)
	}

	prepared, err := repository.PrepareControlledAIOutputAuditDiagnostic(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Plus.ID == "" || prepared.Plus.Status != "candidate" || prepared.PlusQueued != aiOutputAuditDiagnosticSampleSize || len(prepared.BodyContentVersionIDs) != aiOutputAuditDiagnosticSampleSize {
		t.Fatalf("output-audit diagnostic preparation = %#v", prepared)
	}
	assertAIWorkMatchesSample(t, repository, prepared.Plus.ID, prepared.BodyContentVersionIDs)

	standardID, err := repository.ControlledAIPlusTrialConfigurationID(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	diagnosticID, err := repository.ControlledAIOutputAuditDiagnosticConfigurationID(ctx, "llm-mrolescope")
	if err != nil || diagnosticID != prepared.Plus.ID || diagnosticID == standardID {
		t.Fatalf("output-audit diagnostic configuration = %q, %v; standard = %q; want frozen distinct Plus candidate", diagnosticID, err, standardID)
	}
	auditPlusIDs, err := repository.ControlledAIOutputAuditConfigurationIDs(ctx, "llm-mrolescope")
	if err != nil || !equalStrings(auditPlusIDs, []string{diagnosticID, standardID}) {
		t.Fatalf("fixed Plus output-audit configuration IDs = %q, %v; want diagnostic then standard candidate", auditPlusIDs, err)
	}

	reused, err := repository.PrepareControlledAIOutputAuditDiagnostic(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if !reused.Reused || reused.Plus.ID != prepared.Plus.ID || reused.PlusQueued != 0 || !equalStrings(reused.BodyContentVersionIDs, prepared.BodyContentVersionIDs) {
		t.Fatalf("reused output-audit diagnostic = %#v, first = %#v", reused, prepared)
	}
}

func TestPrepareControlledAIOutputAuditDiagnosticCreatesNewCandidateAfterNonPersistableStorageContract(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-non-persistable-audit-diagnostic")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)

	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := repository.prepareControlledAIOutputAuditDiagnostic(ctx,
		prePersistableOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
	)
	if err != nil {
		t.Fatal(err)
	}

	current, err := repository.PrepareControlledAIOutputAuditDiagnostic(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if current.Reused || current.Plus.ID == legacy.Plus.ID || current.PlusQueued != aiOutputAuditDiagnosticSampleSize {
		t.Fatalf("persistable/legacy diagnostic candidates = current:%#v legacy:%#v; want a distinct new five-item candidate", current, legacy)
	}
	assertAIWorkMatchesSample(t, repository, current.Plus.ID, current.BodyContentVersionIDs)
}

func TestPrepareControlledAITrialCreatesNewCandidatesAfterLegacyByteEstimatorContract(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-legacy-token-estimator")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)

	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	legacy := insertHistoricalAIComparisonFixture(t, repository,
		legacyByteEstimatorConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		legacyByteEstimatorConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	)
	preFixQwenBPE := insertHistoricalAIComparisonFixture(t, repository,
		preFixQwenBPEConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		preFixQwenBPEConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	)

	current, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if current.Reused || current.Plus.ID == legacy.Plus.ID || current.Plus.ID == preFixQwenBPE.Plus.ID {
		t.Fatalf("current/legacy candidates = current:%#v byte:%#v qwen-v1:%#v; want distinct new candidates", current, legacy, preFixQwenBPE)
	}
	plus, err := repository.ReadAIConfiguration(ctx, current.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	legacyPlus, err := repository.ReadAIConfiguration(ctx, legacy.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plus.InputContractSHA256 == legacyPlus.InputContractSHA256 {
		t.Fatalf("current input contract digest = %q, legacy digest = %q; want estimator contract identity change", plus.InputContractSHA256, legacyPlus.InputContractSHA256)
	}
	preFixQwenBPEPlus, err := repository.ReadAIConfiguration(ctx, preFixQwenBPE.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plus.InputContractSHA256 == preFixQwenBPEPlus.InputContractSHA256 {
		t.Fatalf("current input contract digest = %q, pre-fix Qwen BPE digest = %q; want estimator contract identity change", plus.InputContractSHA256, preFixQwenBPEPlus.InputContractSHA256)
	}
}

func TestPrepareControlledAITrialCreatesNewCandidatesAfterPreAuditCaptureContract(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-pre-audit-contract")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	preAudit := insertHistoricalAIComparisonFixture(t, repository,
		preAuditCaptureConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		preAuditCaptureConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	)
	current, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if current.Reused || current.Plus.ID == preAudit.Plus.ID {
		t.Fatalf("current/pre-audit candidates = current:%#v pre-audit:%#v; want distinct candidates", current, preAudit)
	}
}

func TestPrepareControlledAITrialCreatesV4ConfigurationsWithoutReusingV2ResultsOrSending(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-v2-v3-contract")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	historical := insertHistoricalAIComparisonFixture(t, repository,
		legacyV2ConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		legacyV2ConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	)
	if _, err := repository.EnsureAISemanticResult(ctx, historical.BodyContentVersionIDs[0], historical.Plus.ID); err != nil {
		t.Fatal(err)
	}
	current, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if current.Reused || current.Plus.ID == historical.Plus.ID {
		t.Fatalf("v4 trial configuration reused a v2 identity: current:%#v historical:%#v", current, historical)
	}
	var resultCount, attempts int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM ai_semantic_results
   WHERE body_content_version_id = $1 AND ai_analysis_configuration_id IN ($2, $3)),
  (SELECT count(*) FROM ai_analysis_attempts
   WHERE ai_analysis_configuration_id = $3)`,
		historical.BodyContentVersionIDs[0], historical.Plus.ID, current.Plus.ID).Scan(&resultCount, &attempts); err != nil {
		t.Fatal(err)
	}
	if resultCount != 2 || attempts != 0 {
		t.Fatalf("v2/v3 result isolation = results:%d attempts:%d, want 2/0", resultCount, attempts)
	}
}

func TestPrepareControlledAITrialDoesNotReuseHistoricalThirtyBodyV3Candidate(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-v3-thirty-to-five-contract")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	historical := insertHistoricalAIComparisonFixture(t, repository,
		legacyThirtyBodyV3ConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		legacyThirtyBodyV3ConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	)
	if len(historical.BodyContentVersionIDs) != aiLegacyTrialSampleSizeV1 {
		t.Fatalf("historical v3 trial body count = %d, want %d", len(historical.BodyContentVersionIDs), aiLegacyTrialSampleSizeV1)
	}

	current, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if current.Reused || current.Plus.ID == historical.Plus.ID || len(current.BodyContentVersionIDs) != aiTrialSampleSize {
		t.Fatalf("current five-body candidate reused historical trial: current:%#v historical:%#v", current, historical)
	}
	var historicalPlusWork, currentPlusWork, currentAttempts int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM ai_analysis_work_items WHERE ai_analysis_configuration_id = $1),
  (SELECT count(*) FROM ai_analysis_work_items WHERE ai_analysis_configuration_id = $2),
  (SELECT count(*) FROM ai_analysis_attempts WHERE ai_analysis_configuration_id = $2)`,
		historical.Plus.ID, current.Plus.ID,
	).Scan(&historicalPlusWork, &currentPlusWork, &currentAttempts); err != nil {
		t.Fatal(err)
	}
	if historicalPlusWork != aiLegacyTrialSampleSizeV1 || currentPlusWork != aiTrialSampleSize || currentAttempts != 0 {
		t.Fatalf("historical/current work and attempts = %d/%d/%d, want 30/5/0", historicalPlusWork, currentPlusWork, currentAttempts)
	}
}

func legacyByteEstimatorConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	input := controlledAITrialConfigurationInput(technologyConfigurationID, requestProfile, pricingProfile)
	input.InputContractSHA256 = sha256Hex(AIInputContractVersionV1 + "|" + AISpanContractVersionV1)
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
}

func preFixQwenBPEConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	input := controlledAITrialConfigurationInput(technologyConfigurationID, requestProfile, pricingProfile)
	input.InputContractSHA256 = aiInputContractSHA256(AIInputTokenEstimatorVersionV1, aiInputTokenEstimatorSHA256(AIInputTokenEstimatorVersionV1))
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
}

func preAuditCaptureConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	input := controlledAITrialConfigurationInput(technologyConfigurationID, requestProfile, pricingProfile)
	input.InputContractSHA256 = aiInputContractSHA256(AIInputTokenEstimatorVersionV2, AIInputTokenEstimatorSHA256())
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
}

func legacyV2ConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	input := controlledAITrialConfigurationInput(technologyConfigurationID, requestProfile, pricingProfile)
	input.PromptSHA256 = AISystemPromptV2SHA256()
	input.ResponseSchemaVersion = AIResponseSchemaVersionV2
	input.ResponseSchemaSHA256 = AIResponseSchemaV2SHA256()
	input.InputContractSHA256 = sha256Hex(strings.Join([]string{
		AIInputContractVersionV1,
		AISpanContractVersionV1,
		AIInputTokenEstimatorVersionV2,
		aiInputTokenEstimatorSHA256(AIInputTokenEstimatorVersionV2),
		AIOutputAuditPolicyVersionV1,
		"retention=168h",
		"capacity_bytes=67108864",
		"response_reservation_bytes=1052672",
	}, "|"))
	input.ValidatorVersion = AIValidatorVersionV2
	input.ValidatorSHA256 = sha256Hex(AIValidatorVersionV2)
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
}

func legacyThirtyBodyV3ConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	input := controlledAITrialConfigurationInput(technologyConfigurationID, requestProfile, pricingProfile)
	input.InputContractSHA256 = legacyThirtyBodyControlledAIInputContractSHA256()
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
}

func prePersistableOutputAuditDiagnosticConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	input := controlledAIOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, requestProfile, pricingProfile)
	input.InputContractSHA256 = sha256Hex(strings.Join([]string{
		controlledAIInputContractSHA256(),
		"diagnostic_execution=plus_only",
		"diagnostic_sample_size=5",
	}, "|"))
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
}

func assertTrialWorkExactlyMatchesSample(t *testing.T, repository *Repository, configurationID string, sample []string) {
	assertAIWorkMatchesSample(t, repository, configurationID, sample)
	if len(sample) != aiTrialSampleSize {
		t.Fatalf("trial sample size = %d, want %d", len(sample), aiTrialSampleSize)
	}
}

func assertAIWorkMatchesSample(t *testing.T, repository *Repository, configurationID string, sample []string) {
	t.Helper()
	ctx := context.Background()
	var workCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1`, configurationID).Scan(&workCount); err != nil {
		t.Fatal(err)
	}
	if workCount != len(sample) {
		t.Fatalf("work count for configuration %s = %d, want %d", configurationID, workCount, len(sample))
	}
	for _, bodyContentVersionID := range sample {
		var matchingWork int
		if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1
  AND body_content_version_id = $2`, configurationID, bodyContentVersionID).Scan(&matchingWork); err != nil {
			t.Fatal(err)
		}
		if matchingWork != 1 {
			t.Fatalf("trial work for configuration %s and body %s = %d, want 1", configurationID, bodyContentVersionID, matchingWork)
		}
	}
}

func insertSyntheticTrialEligibleBodies(t *testing.T, repository *Repository, technologyConfigurationID, dictionaryID string, count int) {
	t.Helper()
	ctx := context.Background()
	var existing int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM job_body_content_versions`).Scan(&existing); err != nil {
		t.Fatal(err)
	}
	for offset := 1; offset <= count; offset++ {
		index := existing + offset
		jobID := fmt.Sprintf("00000000-0000-4000-8000-%012d", 600+index*2)
		bodyID := fmt.Sprintf("00000000-0000-4000-8000-%012d", 601+index*2)
		description := strings.Repeat("d", 100+index*75)
		jobRequire := ""
		addition := ""
		if index%4 >= 2 {
			jobRequire = strings.Repeat("r", 80+index*20)
		}
		if index%4 == 3 {
			addition = strings.Repeat("a", 40+index*10)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'synthetic-source', $2)`, jobID, fmt.Sprintf("synthetic-trial-job-%d", index)); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, 'jd-body-v1', $4, $5, $6, $7, $8, $9)`,
			bodyID, jobID, testSHA256(description+"|"+jobRequire+"|"+addition),
			description, jobRequire, addition,
			testSHA256(description), testSHA256(jobRequire), testSHA256(addition)); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_analysis_results (
    body_content_version_id, parser_version, dictionary_version_id, configuration_id,
    status, completed_at
) VALUES ($1, 'synthetic-ai-trial-reference', $2, $3, 'complete', now())`,
			bodyID, dictionaryID, technologyConfigurationID); err != nil {
			t.Fatal(err)
		}
	}
}
