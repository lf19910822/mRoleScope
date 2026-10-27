package jdanalysis

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// historicalAIComparisonFixture represents rows that predate the Plus-only
// lifecycle. It is test-only: production code has no API that can create a
// new Flash comparison configuration. The fixture lets regression tests prove
// that old immutable rows remain readable and do not affect a new candidate.
type historicalAIComparisonFixture struct {
	Plus                  AIConfiguration
	Flash                 AIConfiguration
	BodyContentVersionIDs []string
	UnqueuedBodyID        string
}

func insertHistoricalAIComparisonFixture(t *testing.T, repository *Repository, plusInput, flashInput CandidateAIConfigurationInput) historicalAIComparisonFixture {
	t.Helper()
	ctx := context.Background()
	if !validAIConfigurationInput(plusInput, "qwen-plus") || !validAIConfigurationInput(flashInput, "qwen-flash") {
		t.Fatal("historical comparison fixture requires valid fixed Plus and Flash identities")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	plus := insertHistoricalAIConfigurationForTest(t, ctx, tx, plusInput, "candidate")
	flash := insertHistoricalAIConfigurationForTest(t, ctx, tx, flashInput, "comparison")
	metrics, err := loadAITrialSamplingMetrics(ctx, tx, plusInput.TechnologyConfigurationID)
	if err != nil {
		t.Fatal(err)
	}
	sampleSize := aiTrialSampleSize
	var sample []string
	if plusInput.InputContractSHA256 == legacyThirtyBodyControlledAIInputContractSHA256() &&
		flashInput.InputContractSHA256 == legacyThirtyBodyControlledAIInputContractSHA256() {
		sampleSize = aiLegacyTrialSampleSizeV1
		sample, err = SelectAITrialSampleV1(metrics)
	} else {
		sample, err = SelectAITrialSampleV2(metrics)
	}
	if err != nil {
		t.Fatal(err)
	}
	if queued, err := enqueueAITrialWork(ctx, tx, plus.ID, sample); err != nil || queued != sampleSize {
		t.Fatalf("queue historical Plus fixture = %d, %v; want %d, nil", queued, err, sampleSize)
	}
	if queued, err := insertHistoricalAITrialWorkForTest(ctx, tx, flash.ID, sample); err != nil || queued != sampleSize {
		t.Fatalf("queue historical Flash fixture = %d, %v; want %d, nil", queued, err, sampleSize)
	}
	queuedBodyIDs := make(map[string]struct{}, len(sample))
	for _, bodyID := range sample {
		queuedBodyIDs[bodyID] = struct{}{}
	}
	var unqueuedBodyID string
	for _, metric := range metrics {
		if _, queued := queuedBodyIDs[metric.BodyContentVersionID]; !queued {
			unqueuedBodyID = metric.BodyContentVersionID
			break
		}
	}
	if unqueuedBodyID == "" {
		t.Fatal("historical fixture requires one eligible body outside its frozen sample")
	}
	var budgetGroupID string
	if err := tx.QueryRow(ctx, `
INSERT INTO ai_analysis_trial_budget_groups DEFAULT VALUES
RETURNING id::text`).Scan(&budgetGroupID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_trial_budget_group_members (
    ai_analysis_trial_budget_group_id, ai_analysis_configuration_id
) VALUES ($1, $2), ($1, $3)`, budgetGroupID, plus.ID, flash.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return historicalAIComparisonFixture{Plus: plus, Flash: flash, BodyContentVersionIDs: sample, UnqueuedBodyID: unqueuedBodyID}
}

func TestHistoricalFlashComparisonIsReadOnlyAndCannotRunNewWork(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	technologyConfigurationID := preparePublishedTechnologyConfigurationForAI(t, repository, bodyID, dictionaryID, technologyID, ruleID, "synthetic-historical-flash-read-only")
	insertSyntheticTrialEligibleBodies(t, repository, technologyConfigurationID, dictionaryID, 6)
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

	if _, err := repository.ReadAIConfiguration(ctx, historical.Flash.ID); err != nil {
		t.Fatalf("historical Flash configuration should remain readable: %v", err)
	}
	if _, err := repository.EnsureAISemanticResult(ctx, historical.UnqueuedBodyID, historical.Flash.ID); err == nil {
		t.Fatal("historical Flash configuration accepted a new semantic-result write")
	}
	if _, err := repository.NewPersistentAITrialBudget(ctx, historical.Flash.ID); err == nil {
		t.Fatal("historical Flash configuration acquired a budget for new execution")
	}
	if _, err := (AISemanticProcessor{
		Repository: repository,
		Runner:     AIModelAttemptRunner{Client: notConfiguredAIModelClient{}, Estimator: fixedAIInputTokenEstimator{tokens: 100}},
	}).ProcessPending(ctx, historical.Flash.ID); err == nil {
		t.Fatal("historical Flash configuration started new processing work")
	}
}

func insertHistoricalAITrialWorkForTest(ctx context.Context, tx pgx.Tx, configurationID string, bodyContentVersionIDs []string) (int, error) {
	queued := 0
	for _, bodyContentVersionID := range bodyContentVersionIDs {
		if _, err := tx.Exec(ctx, `
INSERT INTO ai_semantic_results (body_content_version_id, ai_analysis_configuration_id)
VALUES ($1, $2)
ON CONFLICT (body_content_version_id, ai_analysis_configuration_id) DO NOTHING`, bodyContentVersionID, configurationID); err != nil {
			return 0, err
		}
		command, err := tx.Exec(ctx, `
INSERT INTO ai_analysis_work_items (
    ai_analysis_configuration_id, body_content_version_id, trigger
) VALUES ($1, $2, 'trial'::ai_analysis_trigger)
ON CONFLICT (body_content_version_id, ai_analysis_configuration_id) DO NOTHING`, configurationID, bodyContentVersionID)
		if err != nil {
			return 0, err
		}
		queued += int(command.RowsAffected())
	}
	return queued, nil
}

func insertHistoricalAIConfigurationForTest(t *testing.T, ctx context.Context, tx pgx.Tx, input CandidateAIConfigurationInput, status string) AIConfiguration {
	t.Helper()
	var configuration AIConfiguration
	err := tx.QueryRow(ctx, `
INSERT INTO ai_analysis_configurations (
    technology_analysis_configuration_id, provider, model,
    configuration_sha256, request_profile_sha256, prompt_sha256,
    response_schema_version, response_schema_sha256,
    input_contract_version, input_contract_sha256,
    validator_version, validator_sha256,
    pricing_profile_version, pricing_profile_sha256, status, created_by
) VALUES (
    $1, $2::ai_analysis_provider, $3,
    $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
    $15::ai_analysis_configuration_status, $16
)
RETURNING id::text, status::text`,
		input.TechnologyConfigurationID, input.Provider, input.Model,
		input.ConfigurationSHA256, input.RequestProfileSHA256, input.PromptSHA256,
		input.ResponseSchemaVersion, input.ResponseSchemaSHA256,
		input.InputContractVersion, input.InputContractSHA256,
		input.ValidatorVersion, input.ValidatorSHA256,
		input.PricingProfileVersion, input.PricingProfileSHA256, status, input.CreatedBy,
	).Scan(&configuration.ID, &configuration.Status)
	if err != nil {
		t.Fatal(err)
	}
	return configuration
}
