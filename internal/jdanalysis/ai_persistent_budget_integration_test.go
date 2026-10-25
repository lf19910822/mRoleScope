package jdanalysis

import (
	"context"
	"errors"
	"testing"
)

func TestPersistentAITrialBudgetSharesReservationsAndSettlesReturnedUsage(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-persistent-budget-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)

	plus := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-budget-plus")
	flash := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-budget-flash")
	flash.Model = "qwen-flash"
	flash.RequestProfileSHA256 = testSHA256("synthetic-persistent-budget-flash-request-profile")
	flash.PricingProfileVersion = QwenFlashComparisonPricingProfile().Version
	flash.PricingProfileSHA256 = QwenFlashComparisonPricingProfile().SHA256()
	plus.PricingProfileVersion = QwenPlusTrialPricingProfile().Version
	plus.PricingProfileSHA256 = QwenPlusTrialPricingProfile().SHA256()
	prepared, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{Plus: plus, Flash: flash})
	if err != nil {
		t.Fatal(err)
	}

	plusWorkID := firstPendingAIWorkID(t, repository, prepared.Plus.ID)
	flashWorkID := firstPendingAIWorkID(t, repository, prepared.Flash.ID)
	plusBudget, err := repository.NewPersistentAITrialBudget(ctx, prepared.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	flashBudget, err := repository.NewPersistentAITrialBudget(ctx, prepared.Flash.ID)
	if err != nil {
		t.Fatal(err)
	}
	plusReservation, err := plusBudget.Reserve(ctx, plusWorkID, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := flashBudget.Reserve(ctx, flashWorkID, 1, 100); err != nil {
		t.Fatal(err)
	}
	if err := plusBudget.RecordUsage(ctx, plusReservation, AIModelUsage{InputTokens: 100, OutputTokens: 20, TotalTokens: 120}); err != nil {
		t.Fatal(err)
	}

	snapshot, err := flashBudget.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LogicalTaskCount != 2 || snapshot.AttemptCount != 2 || snapshot.ChargedNanoRMB != 120000 {
		t.Fatalf("persistent shared budget snapshot = %#v, want logical:2 attempts:2 charged:120000", snapshot)
	}
	if snapshot.ReservedNanoRMB <= snapshot.ChargedNanoRMB {
		t.Fatalf("persistent shared budget did not retain the other worker's reservation: %#v", snapshot)
	}
}

func TestAISemanticProcessorPersistentTrialBudgetRecordsReturnedUsage(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-persistent-processor-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	plus := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-processor-plus")
	flash := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-processor-flash")
	flash.Model = "qwen-flash"
	flash.RequestProfileSHA256 = testSHA256("synthetic-persistent-processor-flash-request-profile")
	flash.PricingProfileVersion = QwenFlashComparisonPricingProfile().Version
	flash.PricingProfileSHA256 = QwenFlashComparisonPricingProfile().SHA256()
	prepared, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{Plus: plus, Flash: flash})
	if err != nil {
		t.Fatal(err)
	}

	processor := AISemanticProcessor{
		Repository:                   repository,
		RequirePersistentTrialBudget: true,
		Runner: AIModelAttemptRunner{
			Client:    notConfiguredAIModelClient{},
			Estimator: fixedAIInputTokenEstimator{tokens: 100},
		},
	}
	summary, err := processor.ProcessPending(ctx, prepared.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Claimed != aiTrialSampleSize || summary.Completed != aiTrialSampleSize || summary.Failed != 0 {
		t.Fatalf("persistent trial processing summary = %#v", summary)
	}

	var reservations, settledReservations, attempts, usageRecords int
	var chargedNanoRMB int64
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM ai_analysis_trial_budget_reservations WHERE ai_analysis_configuration_id = $1),
  (SELECT count(*) FROM ai_analysis_trial_budget_reservations WHERE ai_analysis_configuration_id = $1 AND status = 'settled'),
  (SELECT count(*) FROM ai_analysis_attempts WHERE ai_analysis_configuration_id = $1),
  (SELECT count(*)
   FROM ai_analysis_usage_records AS usage
   JOIN ai_analysis_attempts AS attempt ON attempt.id = usage.attempt_id
   WHERE attempt.ai_analysis_configuration_id = $1),
  (SELECT COALESCE(sum(actual_charge_nano_rmb), 0)
   FROM ai_analysis_trial_budget_reservations
   WHERE ai_analysis_configuration_id = $1 AND status = 'settled')`, prepared.Plus.ID).Scan(
		&reservations, &settledReservations, &attempts, &usageRecords, &chargedNanoRMB,
	); err != nil {
		t.Fatal(err)
	}
	if reservations != aiTrialSampleSize || settledReservations != aiTrialSampleSize || attempts != aiTrialSampleSize || usageRecords != aiTrialSampleSize || chargedNanoRMB != int64(aiTrialSampleSize)*28000 {
		t.Fatalf("persistent trial ledger = reservations:%d settled:%d attempts:%d usage:%d charged:%d", reservations, settledReservations, attempts, usageRecords, chargedNanoRMB)
	}
}

func TestAISemanticProcessorRecoversReservedAttemptWithoutReusingItsNumber(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-persistent-recovery-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)

	plus := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-recovery-plus")
	flash := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-recovery-flash")
	flash.Model = "qwen-flash"
	flash.RequestProfileSHA256 = testSHA256("synthetic-persistent-recovery-flash-request-profile")
	flash.PricingProfileVersion = QwenFlashComparisonPricingProfile().Version
	flash.PricingProfileSHA256 = QwenFlashComparisonPricingProfile().SHA256()
	plus.PricingProfileVersion = QwenPlusTrialPricingProfile().Version
	plus.PricingProfileSHA256 = QwenPlusTrialPricingProfile().SHA256()
	prepared, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{Plus: plus, Flash: flash})
	if err != nil {
		t.Fatal(err)
	}

	interruptedRunID, err := repository.createAIAnalysisRun(ctx, aiProcessingConfiguration{ID: prepared.Plus.ID, Model: "qwen-plus", Status: "candidate"})
	if err != nil {
		t.Fatal(err)
	}
	interrupted, found, err := repository.claimNextAIWork(ctx, prepared.Plus.ID, interruptedRunID)
	if err != nil || !found {
		t.Fatalf("claim interrupted AI work = found:%t err:%v", found, err)
	}
	budget, err := repository.NewPersistentAITrialBudget(ctx, prepared.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := budget.Reserve(ctx, interrupted.ID, 1, 100); err != nil {
		t.Fatal(err)
	}

	summary, err := (AISemanticProcessor{
		Repository:                   repository,
		RequirePersistentTrialBudget: true,
		Runner: AIModelAttemptRunner{
			Client:    notConfiguredAIModelClient{},
			Estimator: fixedAIInputTokenEstimator{tokens: 100},
		},
	}).ProcessPending(ctx, prepared.Plus.ID)
	if err != nil {
		t.Fatalf("process after an unrecorded reserved attempt: %v", err)
	}
	if summary.Failed != 0 || summary.Completed != aiTrialSampleSize {
		t.Fatalf("recovered processing summary = %#v", summary)
	}

	var firstStatus, firstCode, secondStatus string
	var attemptCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT status::text FROM ai_analysis_attempts WHERE work_item_id = $1 AND attempt_number = 1),
  (SELECT safe_error_code::text FROM ai_analysis_attempts WHERE work_item_id = $1 AND attempt_number = 1),
  (SELECT status::text FROM ai_analysis_attempts WHERE work_item_id = $1 AND attempt_number = 2),
  (SELECT attempt_count FROM ai_analysis_work_items WHERE id = $1)`, interrupted.ID).Scan(
		&firstStatus, &firstCode, &secondStatus, &attemptCount,
	); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "failed" || firstCode != string(AISafeErrorProviderTimeout) || secondStatus != "succeeded" || attemptCount != 2 {
		t.Fatalf("recovered attempt ledger = first:%s/%s second:%s count:%d", firstStatus, firstCode, secondStatus, attemptCount)
	}
}

func TestAISemanticProcessorSafelyFailsInterruptedWorkAfterThirdReservedAttempt(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-persistent-exhausted-recovery-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)

	plus := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-exhausted-recovery-plus")
	flash := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-exhausted-recovery-flash")
	flash.Model = "qwen-flash"
	flash.RequestProfileSHA256 = testSHA256("synthetic-persistent-exhausted-recovery-flash-request-profile")
	flash.PricingProfileVersion = QwenFlashComparisonPricingProfile().Version
	flash.PricingProfileSHA256 = QwenFlashComparisonPricingProfile().SHA256()
	plus.PricingProfileVersion = QwenPlusTrialPricingProfile().Version
	plus.PricingProfileSHA256 = QwenPlusTrialPricingProfile().SHA256()
	prepared, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{Plus: plus, Flash: flash})
	if err != nil {
		t.Fatal(err)
	}

	interruptedRunID, err := repository.createAIAnalysisRun(ctx, aiProcessingConfiguration{ID: prepared.Plus.ID, Model: "qwen-plus", Status: "candidate"})
	if err != nil {
		t.Fatal(err)
	}
	interrupted, found, err := repository.claimNextAIWork(ctx, prepared.Plus.ID, interruptedRunID)
	if err != nil || !found {
		t.Fatalf("claim interrupted AI work = found:%t err:%v", found, err)
	}
	budget, err := repository.NewPersistentAITrialBudget(ctx, prepared.Plus.ID)
	if err != nil {
		t.Fatal(err)
	}
	for attemptNumber := 1; attemptNumber <= aiModelMaximumAttempts; attemptNumber++ {
		if _, err := budget.Reserve(ctx, interrupted.ID, attemptNumber, 100); err != nil {
			t.Fatal(err)
		}
	}

	summary, err := (AISemanticProcessor{
		Repository:                   repository,
		RequirePersistentTrialBudget: true,
		Runner: AIModelAttemptRunner{
			Client:    notConfiguredAIModelClient{},
			Estimator: fixedAIInputTokenEstimator{tokens: 100},
		},
	}).ProcessPending(ctx, prepared.Plus.ID)
	if !errors.Is(err, ErrAISemanticWorkFailed) {
		t.Fatalf("process after the third unrecorded reserved attempt error = %v, want safe failure", err)
	}
	if summary.Failed != 1 || summary.Completed != aiTrialSampleSize-1 {
		t.Fatalf("exhausted recovery summary = %#v", summary)
	}

	var workStatus, resultStatus, safeCode, interruptedRunStatus string
	var attemptCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT status::text FROM ai_analysis_work_items WHERE id = $1),
  (SELECT execution_status::text FROM ai_semantic_results WHERE ai_analysis_configuration_id = $2 AND body_content_version_id = $3),
  (SELECT last_safe_error_code::text FROM ai_analysis_work_items WHERE id = $1),
  (SELECT attempt_count FROM ai_analysis_work_items WHERE id = $1),
  (SELECT status::text FROM ai_analysis_runs WHERE id = $4)`, interrupted.ID, prepared.Plus.ID, interrupted.BodyContentVersionID, interruptedRunID).Scan(
		&workStatus, &resultStatus, &safeCode, &attemptCount, &interruptedRunStatus,
	); err != nil {
		t.Fatal(err)
	}
	if workStatus != "failed_safely" || resultStatus != "failed_safely" || safeCode != string(AISafeErrorProviderTimeout) || attemptCount != aiModelMaximumAttempts || interruptedRunStatus != "failed_safely" {
		t.Fatalf("exhausted recovery state = work:%s result:%s code:%s attempts:%d interrupted-run:%s", workStatus, resultStatus, safeCode, attemptCount, interruptedRunStatus)
	}
}

func TestAISemanticProcessorPersistentTrialBudgetPreservesNanoRMBUsage(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-persistent-nano-reference")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 34)
	plus := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-nano-plus")
	flash := testCandidateAIConfigurationInput(technologyConfigurationID, "synthetic-persistent-nano-flash")
	flash.Model = "qwen-flash"
	flash.RequestProfileSHA256 = testSHA256("synthetic-persistent-nano-flash-request-profile")
	flash.PricingProfileVersion = QwenFlashComparisonPricingProfile().Version
	flash.PricingProfileSHA256 = QwenFlashComparisonPricingProfile().SHA256()
	prepared, err := repository.PrepareAITrial(ctx, AITrialPreparationInput{Plus: plus, Flash: flash})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (AISemanticProcessor{
		Repository:                   repository,
		RequirePersistentTrialBudget: true,
		Runner: AIModelAttemptRunner{
			Client:    tinyUsageNotConfiguredAIModelClient{},
			Estimator: fixedAIInputTokenEstimator{tokens: 100},
		},
	}).ProcessPending(ctx, prepared.Plus.ID); err != nil {
		t.Fatal(err)
	}

	var actualNanoRMB int64
	if err := repository.pool.QueryRow(ctx, `
SELECT COALESCE(sum(usage.actual_charge_rmb * 1000000000), 0)::bigint
FROM ai_analysis_usage_records AS usage
JOIN ai_analysis_attempts AS attempt ON attempt.id = usage.attempt_id
WHERE attempt.ai_analysis_configuration_id = $1`, prepared.Plus.ID).Scan(&actualNanoRMB); err != nil {
		t.Fatal(err)
	}
	if want := int64(aiTrialSampleSize * 800); actualNanoRMB != want {
		t.Fatalf("persisted actual usage nano RMB = %d, want %d", actualNanoRMB, want)
	}
}

func firstPendingAIWorkID(t *testing.T, repository *Repository, configurationID string) string {
	t.Helper()
	var workID string
	if err := repository.pool.QueryRow(context.Background(), `
SELECT id::text
FROM ai_analysis_work_items
WHERE ai_analysis_configuration_id = $1
  AND status = 'pending'
ORDER BY queued_at, id
LIMIT 1`, configurationID).Scan(&workID); err != nil {
		t.Fatal(err)
	}
	return workID
}

type tinyUsageNotConfiguredAIModelClient struct{}

func (tinyUsageNotConfiguredAIModelClient) Analyze(ctx context.Context, input AIModelInput) (AIModelResponse, error) {
	response, err := (notConfiguredAIModelClient{}).Analyze(ctx, input)
	if err != nil {
		return AIModelResponse{}, err
	}
	response.Usage = AIModelUsage{InputTokens: 1, OutputTokens: 0, TotalTokens: 1}
	return response, nil
}
