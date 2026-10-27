package jdanalysis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	AIControlledConfigurationCreator         = "FAN"
	AIControlledOutputAuditDiagnosticCreator = "FAN/audit-diagnostic"
	// AIOutputAuditDiagnosticExecutionContractV1 records the first execution
	// environment that can durably reserve and capture diagnostic output in the
	// root-only CVM audit mount. It deliberately gives post-fix diagnostics a
	// new immutable candidate identity rather than retrying prior safe failures.
	AIOutputAuditDiagnosticExecutionContractV1 = "diagnostic_output_audit_execution=ai-output-audit-root-mount-v1"
	AIValidatorVersionV1                       = "ai-semantic-validator-v1"
	AIValidatorVersionV2                       = "ai-semantic-validator-v2"
	AIValidatorVersionV3                       = "ai-semantic-validator-v3"
	// AIValidatorVersionV4 identifies the current validator after candidate
	// capacity and soft-boundary handling became part of the frozen contract.
	// Earlier validator versions remain available only for immutable history.
	AIValidatorVersionV4    = "ai-semantic-validator-v4"
	AISpanContractVersionV1 = "ai-span-contract-v1"
)

// AIOutputAuditDiagnosticPreparation exposes only the Plus candidate and its
// frozen five-body membership. No Flash configuration is created, selected or
// budgeted by this preparation.
type AIOutputAuditDiagnosticPreparation struct {
	BodyContentVersionIDs []string
	Plus                  AIConfiguration
	PlusQueued            int
	Reused                bool
}

// PrepareControlledAITrial is the only preparation path intended for the
// remote worker. It has no model, body, prompt, schema, SQL, price, or
// configuration arguments: every non-secret contract value is resolved from
// reviewed in-image assets and the one published deterministic baseline.
func (repository *Repository) PrepareControlledAITrial(ctx context.Context, workspaceID string) (AIPlusTrialPreparation, error) {
	technologyConfigurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return AIPlusTrialPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return AIPlusTrialPreparation{}, err
	}
	return repository.PrepareAIPlusTrial(ctx, AIPlusTrialPreparationInput{
		Plus: controlledAITrialConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
	})
}

// PrepareControlledAIOutputAuditDiagnostic freezes a fresh, Plus-only
// five-body diagnostic. It is intentionally separate from the formal
// baseline trial and has no caller-controlled sample, model, prompt, or body
// selection.
func (repository *Repository) PrepareControlledAIOutputAuditDiagnostic(ctx context.Context, workspaceID string) (AIOutputAuditDiagnosticPreparation, error) {
	technologyConfigurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}
	return repository.prepareControlledAIOutputAuditDiagnostic(ctx,
		controlledAIOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
	)
}

// ControlledAITrialConfigurationID resolves only the exact in-image contract
// for the current published deterministic baseline. It deliberately has no
// "latest candidate" behavior, so an unrelated experiment cannot be executed,
// expanded, or published by a fixed operation.
func (repository *Repository) ControlledAIPlusTrialConfigurationID(ctx context.Context, workspaceID string) (string, error) {
	technologyConfigurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	profile, err := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return "", err
	}
	return repository.controlledAIConfigurationID(ctx,
		controlledAITrialConfigurationInput(technologyConfigurationID, profile, QwenPlusTrialPricingProfile()), "qwen-plus")
}

// ControlledAIOutputAuditDiagnosticConfigurationID resolves only the fixed
// Plus diagnostic identity. It cannot resolve an arbitrary candidate or the
// formal baseline trial.
func (repository *Repository) ControlledAIOutputAuditDiagnosticConfigurationID(ctx context.Context, workspaceID string) (string, error) {
	technologyConfigurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	profile, err := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return "", err
	}
	return repository.controlledAIConfigurationID(ctx,
		controlledAIOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, profile, QwenPlusTrialPricingProfile()), "qwen-plus")
}

// ControlledAIOutputAuditConfigurationIDs resolves the closed set of current
// Plus-only audit-capable configurations. It prefers the five-body diagnostic,
// then includes the ordinary controlled candidate if it exists. No caller can
// select a configuration, and a missing candidate is simply absent.
func (repository *Repository) ControlledAIOutputAuditConfigurationIDs(ctx context.Context, workspaceID string) ([]string, error) {
	technologyConfigurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	profile, err := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	inputs := []CandidateAIConfigurationInput{
		controlledAIOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, profile, QwenPlusTrialPricingProfile()),
		controlledAITrialConfigurationInput(technologyConfigurationID, profile, QwenPlusTrialPricingProfile()),
	}
	configurationIDs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		configurationID, found, err := repository.findControlledAIConfigurationID(ctx, input, "qwen-plus")
		if err != nil {
			return nil, err
		}
		if found {
			configurationIDs = append(configurationIDs, configurationID)
		}
	}
	return configurationIDs, nil
}

func (repository *Repository) controlledAIConfigurationID(ctx context.Context, input CandidateAIConfigurationInput, model string) (string, error) {
	configurationID, found, err := repository.findControlledAIConfigurationID(ctx, input, model)
	if err != nil || !found {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return configurationID, nil
}

func (repository *Repository) findControlledAIConfigurationID(ctx context.Context, input CandidateAIConfigurationInput, model string) (string, bool, error) {
	var configurationID string
	err := repository.pool.QueryRow(ctx, `
SELECT configuration.id::text
FROM ai_analysis_configurations AS configuration
JOIN ai_analysis_trial_budget_group_members AS membership
  ON membership.ai_analysis_configuration_id = configuration.id
WHERE configuration.configuration_sha256 = $1
  AND configuration.technology_analysis_configuration_id = $2
  AND configuration.model = $3`, input.ConfigurationSHA256, input.TechnologyConfigurationID, model).Scan(&configurationID)
	if err == pgx.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return configurationID, true, nil
}

func controlledAITrialConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	return controlledAIConfigurationInput(technologyConfigurationID, requestProfile, pricingProfile, controlledAIInputContractSHA256(), AIControlledConfigurationCreator)
}

func controlledAIOutputAuditDiagnosticConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	return controlledAIConfigurationInput(technologyConfigurationID, requestProfile, pricingProfile, controlledAIOutputAuditDiagnosticInputContractSHA256(), AIControlledOutputAuditDiagnosticCreator)
}

func controlledAIConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile, inputContractSHA256, createdBy string) CandidateAIConfigurationInput {
	input := CandidateAIConfigurationInput{
		TechnologyConfigurationID: technologyConfigurationID,
		Provider:                  "bailian",
		Model:                     requestProfile.Model(),
		RequestProfileSHA256:      requestProfile.SHA256(),
		PromptSHA256:              AISystemPromptV4SHA256(),
		ResponseSchemaVersion:     AIResponseSchemaVersionV4,
		ResponseSchemaSHA256:      AIResponseSchemaV4SHA256(),
		InputContractVersion:      AIInputContractVersionV1,
		InputContractSHA256:       inputContractSHA256,
		ValidatorVersion:          AIValidatorVersionV4,
		ValidatorSHA256:           sha256Hex(AIValidatorVersionV4),
		PricingProfileVersion:     pricingProfile.Version,
		PricingProfileSHA256:      pricingProfile.SHA256(),
		CreatedBy:                 createdBy,
	}
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
}

func (repository *Repository) prepareControlledAIOutputAuditDiagnostic(ctx context.Context, plusInput CandidateAIConfigurationInput) (AIOutputAuditDiagnosticPreparation, error) {
	if !validAIConfigurationInput(plusInput, "qwen-plus") {
		return AIOutputAuditDiagnosticPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, fmt.Errorf("begin AI output audit diagnostic preparation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	plus, plusReused, err := prepareAITrialConfigurationTx(ctx, tx, plusInput, "candidate")
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}
	budgetGroupID, err := ensureAIPlusTrialBudgetGroupTx(ctx, tx, plus.ID)
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}

	plusSample, err := loadAITrialWorkSample(ctx, tx, plus.ID)
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}
	preparation := AIOutputAuditDiagnosticPreparation{Plus: plus}
	if len(plusSample) != 0 {
		if len(plusSample) != aiOutputAuditDiagnosticSampleSize {
			return AIOutputAuditDiagnosticPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		preparation.BodyContentVersionIDs = plusSample
		preparation.Reused = plusReused
	} else {
		metrics, err := loadAITrialSamplingMetrics(ctx, tx, plusInput.TechnologyConfigurationID)
		if err != nil {
			return AIOutputAuditDiagnosticPreparation{}, err
		}
		sample, err := SelectAIOutputAuditDiagnosticSampleV1(metrics)
		if err != nil {
			return AIOutputAuditDiagnosticPreparation{}, err
		}
		sort.Strings(sample)
		queued, err := enqueueAITrialWork(ctx, tx, plus.ID, sample)
		if err != nil {
			return AIOutputAuditDiagnosticPreparation{}, err
		}
		if queued != aiOutputAuditDiagnosticSampleSize {
			return AIOutputAuditDiagnosticPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		preparation.BodyContentVersionIDs = sample
		preparation.PlusQueued = queued
		preparation.Reused = plusReused
	}
	_ = budgetGroupID // The opaque group is bound before any diagnostic work is eligible.
	if err := tx.Commit(ctx); err != nil {
		return AIOutputAuditDiagnosticPreparation{}, fmt.Errorf("commit AI output audit diagnostic preparation: %w", err)
	}
	return preparation, nil
}

func candidateAIConfigurationSHA256(input CandidateAIConfigurationInput) string {
	return sha256Hex(strings.Join([]string{
		input.TechnologyConfigurationID,
		input.Provider,
		input.Model,
		input.RequestProfileSHA256,
		input.PromptSHA256,
		input.ResponseSchemaVersion,
		input.ResponseSchemaSHA256,
		input.InputContractVersion,
		input.InputContractSHA256,
		input.ValidatorVersion,
		input.ValidatorSHA256,
		input.PricingProfileVersion,
		input.PricingProfileSHA256,
		input.CreatedBy,
	}, "\n"))
}

// controlledAIInputContractSHA256 records every non-secret rule that affects
// the provider-visible input or the pre-send decision. Changing the estimator
// produces a new candidate rather than reusing historical trial results that
// were accepted or rejected under a different counting contract.
func controlledAIInputContractSHA256() string {
	components := []string{
		AIInputContractVersionV1,
		AISpanContractVersionV1,
		AIInputTokenEstimatorVersionV3,
		AIInputTokenEstimatorSHA256(),
		"formal_trial_sampling_version=" + aiTrialSamplingVersionV2,
		"formal_trial_sample_size=5",
		"formal_trial_max_logical_tasks=5",
		"formal_trial_max_attempts=15",
		"formal_trial_max_charge_nano_rmb=2500000000",
	}
	components = append(components, aiOutputAuditInputContractComponents()...)
	return sha256Hex(strings.Join(components, "|"))
}

// legacyThirtyBodyControlledAIInputContractSHA256 preserves the exact input
// contract digest used by the already-created v3 30-body trials. It is kept
// only so their CVM-resident audit artifacts remain readable; fixed lifecycle
// operations resolve the current five-body digest and therefore cannot reuse
// that historical candidate.
func legacyThirtyBodyControlledAIInputContractSHA256() string {
	components := []string{
		AIInputContractVersionV1,
		AISpanContractVersionV1,
		AIInputTokenEstimatorVersionV3,
		AIInputTokenEstimatorSHA256(),
	}
	components = append(components, aiOutputAuditInputContractComponents()...)
	return sha256Hex(strings.Join(components, "|"))
}

func controlledAIOutputAuditDiagnosticInputContractSHA256() string {
	return sha256Hex(strings.Join([]string{
		controlledAIInputContractSHA256(),
		"diagnostic_execution=plus_only",
		"diagnostic_sample_size=5",
		AIOutputAuditDiagnosticExecutionContractV1,
	}, "|"))
}

func legacyThirtyBodyControlledAIOutputAuditDiagnosticInputContractSHA256() string {
	return sha256Hex(strings.Join([]string{
		legacyThirtyBodyControlledAIInputContractSHA256(),
		"diagnostic_execution=plus_only",
		"diagnostic_sample_size=5",
	}, "|"))
}

func aiInputContractSHA256(estimatorVersion, estimatorSHA256 string) string {
	return sha256Hex(strings.Join([]string{
		AIInputContractVersionV1,
		AISpanContractVersionV1,
		estimatorVersion,
		estimatorSHA256,
	}, "|"))
}
