package jdanalysis

import (
	"context"
	"strings"
)

const (
	AIControlledConfigurationCreator = "FAN"
	AIValidatorVersionV1             = "ai-semantic-validator-v1"
	AISpanContractVersionV1          = "ai-span-contract-v1"
)

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
	var configurationID string
	err = repository.pool.QueryRow(ctx, `
SELECT configuration.id::text
FROM ai_analysis_configurations AS configuration
JOIN ai_analysis_trial_budget_group_members AS membership
  ON membership.ai_analysis_configuration_id = configuration.id
WHERE configuration.configuration_sha256 = $1
  AND configuration.technology_analysis_configuration_id = $2
  AND configuration.model = $3`, input.ConfigurationSHA256, technologyConfigurationID, model).Scan(&configurationID)
	if err != nil {
		return "", NewSafeAIError(AISafeErrorConfigurationInvalid, err)
	}
	return configurationID, nil
}

func controlledAITrialConfigurationInput(technologyConfigurationID string, requestProfile QwenRequestProfile, pricingProfile AITrialPricingProfile) CandidateAIConfigurationInput {
	input := CandidateAIConfigurationInput{
		TechnologyConfigurationID: technologyConfigurationID,
		Provider:                  "bailian",
		Model:                     requestProfile.Model(),
		RequestProfileSHA256:      requestProfile.SHA256(),
		PromptSHA256:              AISystemPromptV1SHA256(),
		ResponseSchemaVersion:     AIResponseSchemaVersionV1,
		ResponseSchemaSHA256:      AIResponseSchemaV1SHA256(),
		InputContractVersion:      AIInputContractVersionV1,
		InputContractSHA256:       controlledAIInputContractSHA256(),
		ValidatorVersion:          AIValidatorVersionV1,
		ValidatorSHA256:           sha256Hex(AIValidatorVersionV1),
		PricingProfileVersion:     pricingProfile.Version,
		PricingProfileSHA256:      pricingProfile.SHA256(),
		CreatedBy:                 AIControlledConfigurationCreator,
	}
	input.ConfigurationSHA256 = candidateAIConfigurationSHA256(input)
	return input
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
	return sha256Hex(strings.Join([]string{
		AIInputContractVersionV1,
		AISpanContractVersionV1,
		AIInputTokenEstimatorVersionV1,
		AIInputTokenEstimatorSHA256(),
	}, "|"))
}
