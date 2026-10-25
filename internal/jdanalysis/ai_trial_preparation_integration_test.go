package jdanalysis

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestPrepareAITrialFreezesSameThirtyBodyIDsForPlusAndFlash(t *testing.T) {
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
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)

	plus := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-trial-plus")
	flash := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-ai-trial-flash")
	flash.Model = "qwen-flash"
	flash.RequestProfileSHA256 = testSHA256("synthetic-ai-trial-flash-request-profile")
	flash.PricingProfileSHA256 = testSHA256("synthetic-ai-trial-flash-pricing-profile")
	prepared, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{Plus: plus, Flash: flash})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Plus.ID == "" || prepared.Flash.ID == "" || prepared.Plus.ID == prepared.Flash.ID ||
		prepared.Plus.Status != "candidate" || prepared.Flash.Status != "comparison" ||
		prepared.PlusQueued != aiTrialSampleSize || prepared.FlashQueued != aiTrialSampleSize ||
		len(prepared.BodyContentVersionIDs) != aiTrialSampleSize {
		t.Fatalf("trial preparation = %#v", prepared)
	}
	assertTrialWorkExactlyMatchesSample(t, repository, prepared.Plus.ID, prepared.BodyContentVersionIDs)
	assertTrialWorkExactlyMatchesSample(t, repository, prepared.Flash.ID, prepared.BodyContentVersionIDs)
	var budgetGroupCount, budgetMemberCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*)
   FROM ai_analysis_trial_budget_groups AS budget_group
   JOIN ai_analysis_trial_budget_group_members AS plus_member
     ON plus_member.ai_analysis_trial_budget_group_id = budget_group.id
    AND plus_member.ai_analysis_configuration_id = $1
   JOIN ai_analysis_trial_budget_group_members AS flash_member
     ON flash_member.ai_analysis_trial_budget_group_id = budget_group.id
    AND flash_member.ai_analysis_configuration_id = $2),
  (SELECT count(*)
   FROM ai_analysis_trial_budget_group_members
   WHERE ai_analysis_configuration_id IN ($1, $2))`, prepared.Plus.ID, prepared.Flash.ID).Scan(&budgetGroupCount, &budgetMemberCount); err != nil {
		t.Fatal(err)
	}
	if budgetGroupCount != 1 || budgetMemberCount != 2 {
		t.Fatalf("trial budget membership = groups:%d members:%d, want 1/2", budgetGroupCount, budgetMemberCount)
	}
	report, err := repository.ReadAITrialComparisonReport(ctx, prepared.Plus.ID, prepared.Flash.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Plus.AIConfigurationID != prepared.Plus.ID || report.Flash.AIConfigurationID != prepared.Flash.ID ||
		report.Plus.LogicalTaskCount != aiTrialSampleSize || report.Flash.LogicalTaskCount != aiTrialSampleSize ||
		report.Plus.PendingTaskCount != aiTrialSampleSize || report.Flash.PendingTaskCount != aiTrialSampleSize ||
		report.Plus.CompleteTaskCount != 0 || report.Plus.FailedTaskCount != 0 ||
		len(report.Plus.SafeFailureCounts) != 0 || report.Flash.CompleteTaskCount != 0 ||
		report.Flash.FailedTaskCount != 0 || len(report.Flash.SafeFailureCounts) != 0 ||
		report.Plus.InputTokenCount != 0 || report.Flash.OutputTokenCount != 0 {
		t.Fatalf("text-free AI comparison report = %#v", report)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_work_items
SET status = 'failed_safely', last_safe_error_code = 'invalid_response_schema', completed_at = now()
WHERE id = (
  SELECT id
  FROM ai_analysis_work_items
  WHERE ai_analysis_configuration_id = $1
  ORDER BY id
  LIMIT 1
)`, prepared.Plus.ID); err != nil {
		t.Fatal(err)
	}
	report, err = repository.ReadAITrialComparisonReport(ctx, prepared.Plus.ID, prepared.Flash.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Plus.PendingTaskCount != aiTrialSampleSize-1 || report.Plus.FailedTaskCount != 1 ||
		len(report.Plus.SafeFailureCounts) != 1 ||
		report.Plus.SafeFailureCounts[0] != (AISafeFailureCount{Code: AISafeErrorInvalidResponseSchema, Count: 1}) {
		t.Fatalf("safe failure report = %#v", report.Plus)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO ai_analysis_publication_state (singleton)
VALUES (true)
ON CONFLICT (singleton) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
UPDATE ai_analysis_publication_state
SET ai_analysis_configuration_id = $1, updated_at = now()
WHERE singleton = true`, prepared.Flash.ID); err == nil {
		t.Fatal("Flash comparison configuration became the AI publication default")
	}

	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 1)
	reused, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{Plus: plus, Flash: flash})
	if err != nil {
		t.Fatal(err)
	}
	if !reused.Reused || reused.Plus.ID != prepared.Plus.ID || reused.Flash.ID != prepared.Flash.ID ||
		reused.PlusQueued != 0 || reused.FlashQueued != 0 ||
		!equalStrings(reused.BodyContentVersionIDs, prepared.BodyContentVersionIDs) {
		t.Fatalf("reused trial preparation = %#v, initial = %#v", reused, prepared)
	}
	assertTrialWorkExactlyMatchesSample(t, repository, reused.Plus.ID, prepared.BodyContentVersionIDs)
	assertTrialWorkExactlyMatchesSample(t, repository, reused.Flash.ID, prepared.BodyContentVersionIDs)
}

func TestPrepareControlledAITrialUsesOnlyFixedProfilesAndPublishedBaseline(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-controlled-ai-trial-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)

	first, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if first.Plus.Status != "candidate" || first.Flash.Status != "comparison" || first.PlusQueued != aiTrialSampleSize || first.FlashQueued != aiTrialSampleSize || len(first.BodyContentVersionIDs) != aiTrialSampleSize {
		t.Fatalf("controlled AI trial preparation = %#v", first)
	}
	plus, err := repository.ReadAIConfiguration(ctx, first.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	flash, err := repository.ReadAIConfiguration(ctx, first.Flash.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plus.TechnologyConfigurationID != technologyConfigurationID || plus.Model != "qwen-plus" || plus.PricingProfileVersion != QwenPlusTrialPricingProfile().Version || plus.PricingProfileSHA256 != QwenPlusTrialPricingProfile().SHA256() || flash.TechnologyConfigurationID != technologyConfigurationID || flash.Model != "qwen-flash" || flash.PricingProfileVersion != QwenFlashComparisonPricingProfile().Version || flash.PricingProfileSHA256 != QwenFlashComparisonPricingProfile().SHA256() {
		t.Fatalf("controlled AI trial configurations = plus:%#v flash:%#v", plus, flash)
	}

	second, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.Plus.ID != first.Plus.ID || second.Flash.ID != first.Flash.ID || second.PlusQueued != 0 || second.FlashQueued != 0 {
		t.Fatalf("controlled AI trial reuse = %#v, first = %#v", second, first)
	}
	plusID, err := repository.ControlledAITrialConfigurationID(ctx, "qwen-plus", "llm-mrolescope")
	if err != nil || plusID != first.Plus.ID {
		t.Fatalf("controlled Plus configuration = %q, %v; want %q, nil", plusID, err, first.Plus.ID)
	}
	flashID, err := repository.ControlledAITrialConfigurationID(ctx, "qwen-flash", "llm-mrolescope")
	if err != nil || flashID != first.Flash.ID {
		t.Fatalf("controlled Flash configuration = %q, %v; want %q, nil", flashID, err, first.Flash.ID)
	}
	if _, err := repository.ControlledAITrialConfigurationID(ctx, "uncontrolled-model", "llm-mrolescope"); err == nil {
		t.Fatal("uncontrolled AI model resolved to a trial configuration")
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

	var flashWorkCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM ai_analysis_trial_budget_group_members AS diagnostic_member
JOIN ai_analysis_trial_budget_group_members AS flash_member
  ON flash_member.ai_analysis_trial_budget_group_id = diagnostic_member.ai_analysis_trial_budget_group_id
JOIN ai_analysis_configurations AS flash
  ON flash.id = flash_member.ai_analysis_configuration_id
JOIN ai_analysis_work_items AS work
  ON work.ai_analysis_configuration_id = flash_member.ai_analysis_configuration_id
WHERE diagnostic_member.ai_analysis_configuration_id = $1
  AND flash.model = 'qwen-flash'`, prepared.Plus.ID).Scan(&flashWorkCount); err != nil {
		t.Fatal(err)
	}
	if flashWorkCount != 0 {
		t.Fatalf("output-audit diagnostic queued %d Flash work items, want 0", flashWorkCount)
	}

	standardID, err := repository.ControlledAITrialConfigurationID(ctx, "qwen-plus", "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	diagnosticID, err := repository.ControlledAIOutputAuditDiagnosticConfigurationID(ctx, "llm-mrolescope")
	if err != nil || diagnosticID != prepared.Plus.ID || diagnosticID == standardID {
		t.Fatalf("output-audit diagnostic configuration = %q, %v; standard = %q; want frozen distinct Plus candidate", diagnosticID, err, standardID)
	}
	auditPlusIDs, err := repository.ControlledAIOutputAuditConfigurationIDs(ctx, "qwen-plus", "llm-mrolescope")
	if err != nil || !equalStrings(auditPlusIDs, []string{diagnosticID, standardID}) {
		t.Fatalf("fixed Plus output-audit configuration IDs = %q, %v; want diagnostic then standard candidate", auditPlusIDs, err)
	}
	standardFlashID, err := repository.ControlledAITrialConfigurationID(ctx, "qwen-flash", "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	auditFlashIDs, err := repository.ControlledAIOutputAuditConfigurationIDs(ctx, "qwen-flash", "llm-mrolescope")
	if err != nil || !equalStrings(auditFlashIDs, []string{standardFlashID}) {
		t.Fatalf("fixed Flash output-audit configuration IDs = %q, %v; want standard comparison candidate", auditFlashIDs, err)
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
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := repository.prepareControlledAIOutputAuditDiagnostic(ctx,
		prePersistableOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		prePersistableOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
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
	legacy, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{
		Plus:  legacyByteEstimatorConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		Flash: legacyByteEstimatorConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	})
	if err != nil {
		t.Fatal(err)
	}
	preFixQwenBPE, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{
		Plus:  preFixQwenBPEConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		Flash: preFixQwenBPEConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	})
	if err != nil {
		t.Fatal(err)
	}

	current, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if current.Reused || current.Plus.ID == legacy.Plus.ID || current.Flash.ID == legacy.Flash.ID ||
		current.Plus.ID == preFixQwenBPE.Plus.ID || current.Flash.ID == preFixQwenBPE.Flash.ID {
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
	preAudit, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{
		Plus:  preAuditCaptureConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		Flash: preAuditCaptureConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err := repository.PrepareControlledAITrial(ctx, "llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	if current.Reused || current.Plus.ID == preAudit.Plus.ID || current.Flash.ID == preAudit.Flash.ID {
		t.Fatalf("current/pre-audit candidates = current:%#v pre-audit:%#v; want distinct candidates", current, preAudit)
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
