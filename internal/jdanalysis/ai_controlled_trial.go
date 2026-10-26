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
	AIValidatorVersionV1                     = "ai-semantic-validator-v1"
	AIValidatorVersionV2                     = "ai-semantic-validator-v2"
	AIValidatorVersionV3                     = "ai-semantic-validator-v3"
	AISpanContractVersionV1                  = "ai-span-contract-v1"
)

// AIOutputAuditDiagnosticPreparation exposes only the Plus candidate and its
// frozen five-body membership. The internal Flash comparison identity exists
// solely to retain the existing shared persistent-budget invariant; no Flash
// work is queued or callable from this diagnostic preparation.
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
func (repository *Repository) PrepareControlledAITrial(ctx context.Context, workspaceID string) (AITrialPreparation, error) {
	technologyConfigurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return AITrialPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return AITrialPreparation{}, err
	}
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return AITrialPreparation{}, err
	}
	return repository.PrepareAITrial(ctx, AITrialPreparationInput{
		Plus:  controlledAITrialConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		Flash: controlledAITrialConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
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
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace(workspaceID)
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}
	return repository.prepareControlledAIOutputAuditDiagnostic(ctx,
		controlledAIOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, plusProfile, QwenPlusTrialPricingProfile()),
		controlledAIOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, flashProfile, QwenFlashComparisonPricingProfile()),
	)
}

// ControlledAITrialConfigurationID resolves only the exact in-image contract
// for the current published deterministic baseline. It deliberately has no
// "latest candidate" behavior, so an unrelated experiment cannot be executed,
// expanded, or published by a fixed operation.
func (repository *Repository) ControlledAITrialConfigurationID(ctx context.Context, model, workspaceID string) (string, error) {
	technologyConfigurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	var input CandidateAIConfigurationInput
	switch model {
	case "qwen-plus":
		profile, profileErr := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
		if profileErr != nil {
			return "", profileErr
		}
		input = controlledAITrialConfigurationInput(technologyConfigurationID, profile, QwenPlusTrialPricingProfile())
	case "qwen-flash":
		profile, profileErr := QwenFlashComparisonProfileForBeijingWorkspace(workspaceID)
		if profileErr != nil {
			return "", profileErr
		}
		input = controlledAITrialConfigurationInput(technologyConfigurationID, profile, QwenFlashComparisonPricingProfile())
	default:
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	return repository.controlledAIConfigurationID(ctx, input, model)
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
// audit-capable configurations for one fixed review arm. Plus prefers the
// five-body diagnostic, then includes the ordinary controlled candidate if it
// exists; Flash includes only its ordinary comparison candidate. No caller can
// select a configuration, and a missing not-yet-prepared arm is simply empty.
func (repository *Repository) ControlledAIOutputAuditConfigurationIDs(ctx context.Context, model, workspaceID string) ([]string, error) {
	technologyConfigurationID, err := repository.PublishedConfigurationID(ctx)
	if err != nil {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	var inputs []CandidateAIConfigurationInput
	switch model {
	case "qwen-plus":
		profile, profileErr := QwenPlusTrialProfileForBeijingWorkspace(workspaceID)
		if profileErr != nil {
			return nil, profileErr
		}
		inputs = []CandidateAIConfigurationInput{
			controlledAIOutputAuditDiagnosticConfigurationInput(technologyConfigurationID, profile, QwenPlusTrialPricingProfile()),
			controlledAITrialConfigurationInput(technologyConfigurationID, profile, QwenPlusTrialPricingProfile()),
		}
	case "qwen-flash":
		profile, profileErr := QwenFlashComparisonProfileForBeijingWorkspace(workspaceID)
		if profileErr != nil {
			return nil, profileErr
		}
		inputs = []CandidateAIConfigurationInput{
			controlledAITrialConfigurationInput(technologyConfigurationID, profile, QwenFlashComparisonPricingProfile()),
		}
	default:
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	configurationIDs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		configurationID, found, err := repository.findControlledAIConfigurationID(ctx, input, model)
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
		PromptSHA256:              AISystemPromptV3SHA256(),
		ResponseSchemaVersion:     AIResponseSchemaVersionV3,
		ResponseSchemaSHA256:      AIResponseSchemaV3SHA256(),
		InputContractVersion:      AIInputContractVersionV1,
		InputContractSHA256:       inputContractSHA256,
		ValidatorVersion:          AIValidatorVersionV3,
		ValidatorSHA256:           sha256Hex(AIValidatorVersionV3),
		PricingProfileVersion:     pricingProfile.Version,
		PricingProfileSHA256:      pricingProfile.SHA256(),
		CreatedBy:                 createdBy,
	}
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
}

func (repository *Repository) prepareControlledAIOutputAuditDiagnostic(ctx context.Context, plusInput, flashInput CandidateAIConfigurationInput) (AIOutputAuditDiagnosticPreparation, error) {
	if !validAITrialPreparationInput(AITrialPreparationInput{Plus: plusInput, Flash: flashInput}) {
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
	flash, flashReused, err := prepareAITrialConfigurationTx(ctx, tx, flashInput, "comparison")
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}
	if _, err := ensureAITrialBudgetGroupTx(ctx, tx, plus.ID, flash.ID); err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}

	plusSample, err := loadAITrialWorkSample(ctx, tx, plus.ID)
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}
	flashSample, err := loadAITrialWorkSample(ctx, tx, flash.ID)
	if err != nil {
		return AIOutputAuditDiagnosticPreparation{}, err
	}
	preparation := AIOutputAuditDiagnosticPreparation{Plus: plus}
	if len(plusSample) != 0 || len(flashSample) != 0 {
		if len(plusSample) != aiOutputAuditDiagnosticSampleSize || len(flashSample) != 0 {
			return AIOutputAuditDiagnosticPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		preparation.BodyContentVersionIDs = plusSample
		preparation.Reused = plusReused && flashReused
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
		queued, err := enqueueAITrialWork(ctx, tx, plus.ID, sample, "candidate")
		if err != nil {
			return AIOutputAuditDiagnosticPreparation{}, err
		}
		if queued != aiOutputAuditDiagnosticSampleSize {
			return AIOutputAuditDiagnosticPreparation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
		}
		preparation.BodyContentVersionIDs = sample
		preparation.PlusQueued = queued
		preparation.Reused = plusReused && flashReused
	}
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
		"formal_trial_max_logical_tasks=10",
		"formal_trial_max_attempts=30",
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
