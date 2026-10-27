package jdanalysis

import (
	"encoding/json"
)

const (
	aiTrialMaximumAttempts     = 15
	aiTrialMaximumLogicalTasks = 5
	aiTrialMaximumNanoRMB      = int64(2_500_000_000)
)

// AIInputTokenEstimator provides a conservative pre-send estimate. It is a
// separate seam because provider tokenizers may be updated independently of
// request construction; an estimate that cannot be made must stop the call.
type AIInputTokenEstimator interface {
	Estimate(AIModelInput) (int, error)
}

// AITrialBudgetGate serializes the fixed trial limits and pricing reservations
// for one candidate configuration. Its summary contains only counts and money,
// never text, prompts, responses, or secrets.
type AITrialBudgetGate struct {
	chargedNanoRMB     int64
	logicalTaskIDs     map[string]struct{}
	maxNanoRMB         int64
	profile            AITrialPricingProfile
	reservations       map[string]int64
	reservedNanoRMB    int64
	seenAssociationIDs map[string]struct{}
	attemptCount       int
}

// AITrialBudgetReservation is an opaque record for correcting a worst-case
// reservation with returned provider usage after one physical request.
type AITrialBudgetReservation struct {
	associationID string
}

// AITrialBudgetSnapshot is a safe observability value for tests, ledgers, and
// operations output.
type AITrialBudgetSnapshot struct {
	AttemptCount     int
	ChargedNanoRMB   int64
	LogicalTaskCount int
	ReservedNanoRMB  int64
}

// AITrialPricingProfile is a reviewed, frozen pricing/usage capability
// record. The unexported verification flags prevent an operations caller from
// supplying an arbitrary unverified price as a budget bypass.
type AITrialPricingProfile struct {
	MaxInputTokens  int
	MaxOutputTokens int
	Model           string
	Version         string

	inputNanoRMBPerToken  int64
	outputNanoRMBPerToken int64
	usageSupported        bool
	verified              bool
}

// SHA256 is the stable digest persisted in an immutable AI configuration. It
// covers every non-secret value that affects a budget decision, so changing a
// model, limit, price, or usage capability requires a new configuration.
func (profile AITrialPricingProfile) SHA256() string {
	encoded, err := json.Marshal(struct {
		InputNanoRMBPerToken  int64  `json:"input_nano_rmb_per_token"`
		MaxInputTokens        int    `json:"max_input_tokens"`
		MaxOutputTokens       int    `json:"max_output_tokens"`
		Model                 string `json:"model"`
		OutputNanoRMBPerToken int64  `json:"output_nano_rmb_per_token"`
		UsageSupported        bool   `json:"usage_supported"`
		Version               string `json:"version"`
	}{
		InputNanoRMBPerToken:  profile.inputNanoRMBPerToken,
		MaxInputTokens:        profile.MaxInputTokens,
		MaxOutputTokens:       profile.MaxOutputTokens,
		Model:                 profile.Model,
		OutputNanoRMBPerToken: profile.outputNanoRMBPerToken,
		UsageSupported:        profile.usageSupported,
		Version:               profile.Version,
	})
	if err != nil {
		return ""
	}
	return sha256Hex(string(encoded))
}

// ConservativeAIInputTokenEstimator counts the model-visible system prompt,
// serialized model input, and strict response schema with the pinned Qwen BPE
// vocabulary plus a fixed chat envelope allowance. It intentionally adds safe
// segmentation boundaries rather than treating UTF-8 bytes as tokens. It never
// truncates text to meet a token budget; an over-limit estimate is handled by
// the budget gate.
type ConservativeAIInputTokenEstimator struct{}

// Estimate returns a conservative request-size estimate without persisting or
// reporting the serialized input.
func (estimator ConservativeAIInputTokenEstimator) Estimate(input AIModelInput) (int, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	responseSchemaVersion := input.responseContract.responseSchemaVersion
	if responseSchemaVersion == "" {
		// Synthetic callers that exercise only the preflight seam still model
		// the current controlled request rather than an historical contract.
		responseSchemaVersion = AIResponseSchemaVersionV4
	}
	assets, err := responseContractAssets(responseSchemaVersion)
	if err != nil {
		return 0, err
	}
	promptTokens, err := conservativeQwenTokenCount(input.reviewedSystemPrompt())
	if err != nil {
		return 0, err
	}
	inputTokens, err := conservativeQwenTokenCount(string(encoded))
	if err != nil {
		return 0, err
	}
	schemaTokens, err := conservativeQwenTokenCount(assets.schema)
	if err != nil {
		return 0, err
	}
	return promptTokens + inputTokens + schemaTokens + qwenChatEnvelopeTokens, nil
}

// NewAITrialBudgetGate accepts only a complete verified frozen pricing record.
func NewAITrialBudgetGate(profile AITrialPricingProfile) (*AITrialBudgetGate, error) {
	if !validAITrialPricingProfile(profile) {
		return nil, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	return &AITrialBudgetGate{
		logicalTaskIDs:     make(map[string]struct{}),
		maxNanoRMB:         aiTrialMaximumNanoRMB,
		profile:            profile,
		reservations:       make(map[string]int64),
		seenAssociationIDs: make(map[string]struct{}),
	}, nil
}

// QwenFlashComparisonPricingProfile is the frozen comparison pricing profile.
func QwenFlashComparisonPricingProfile() AITrialPricingProfile {
	return AITrialPricingProfile{
		MaxInputTokens:        10000,
		MaxOutputTokens:       5000,
		Model:                 "qwen-flash",
		Version:               "bailian-beijing-short-context-2026-10-23-v2",
		inputNanoRMBPerToken:  150,
		outputNanoRMBPerToken: 1500,
		usageSupported:        true,
		verified:              true,
	}
}

// QwenPlusTrialPricingProfile is the frozen v0.1 baseline pricing profile.
func QwenPlusTrialPricingProfile() AITrialPricingProfile {
	return AITrialPricingProfile{
		MaxInputTokens:        10000,
		MaxOutputTokens:       5000,
		Model:                 "qwen-plus",
		Version:               "bailian-beijing-short-context-2026-10-23-v2",
		inputNanoRMBPerToken:  800,
		outputNanoRMBPerToken: 2000,
		usageSupported:        true,
		verified:              true,
	}
}

// RecordUsage replaces one reservation's worst-case charge with returned
// usage. A provider response above the frozen profile remains a safe budget
// failure rather than a reason to issue another request.
func (gate *AITrialBudgetGate) RecordUsage(reservation AITrialBudgetReservation, usage AIModelUsage) error {
	if gate == nil || !usage.valid() || usage.InputTokens > gate.profile.MaxInputTokens || usage.OutputTokens > gate.profile.MaxOutputTokens {
		return NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}
	estimatedCharge, exists := gate.reservations[reservation.associationID]
	if !exists {
		return NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	actualCharge := gate.quote(usage.InputTokens, usage.OutputTokens)
	newReservedCharge := gate.reservedNanoRMB - estimatedCharge + actualCharge
	if newReservedCharge > gate.maxNanoRMB {
		return NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}
	gate.reservedNanoRMB = newReservedCharge
	gate.chargedNanoRMB += actualCharge
	delete(gate.reservations, reservation.associationID)
	return nil
}

// Reserve performs the pre-send gate. A successful reservation counts a
// physical attempt even if transport later fails and never yields usage.
func (gate *AITrialBudgetGate) Reserve(logicalTaskID, associationID string, estimatedInputTokens int) (AITrialBudgetReservation, error) {
	if gate == nil || logicalTaskID == "" || associationID == "" || estimatedInputTokens < 0 {
		return AITrialBudgetReservation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	if estimatedInputTokens > gate.profile.MaxInputTokens {
		return AITrialBudgetReservation{}, NewSafeAIError(AISafeErrorInputTooLarge, nil)
	}
	if _, duplicate := gate.seenAssociationIDs[associationID]; duplicate {
		return AITrialBudgetReservation{}, NewSafeAIError(AISafeErrorConfigurationInvalid, nil)
	}
	_, knownLogicalTask := gate.logicalTaskIDs[logicalTaskID]
	if (!knownLogicalTask && len(gate.logicalTaskIDs) >= aiTrialMaximumLogicalTasks) || gate.attemptCount >= aiTrialMaximumAttempts {
		return AITrialBudgetReservation{}, NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}
	estimatedCharge := gate.quote(estimatedInputTokens, gate.profile.MaxOutputTokens)
	if gate.reservedNanoRMB+estimatedCharge > gate.maxNanoRMB {
		return AITrialBudgetReservation{}, NewSafeAIError(AISafeErrorBudgetExhausted, nil)
	}
	gate.logicalTaskIDs[logicalTaskID] = struct{}{}
	gate.attemptCount++
	gate.reservedNanoRMB += estimatedCharge
	gate.reservations[associationID] = estimatedCharge
	gate.seenAssociationIDs[associationID] = struct{}{}
	return AITrialBudgetReservation{associationID: associationID}, nil
}

// Snapshot returns only safe aggregate accounting.
func (gate *AITrialBudgetGate) Snapshot() AITrialBudgetSnapshot {
	if gate == nil {
		return AITrialBudgetSnapshot{}
	}
	return AITrialBudgetSnapshot{
		AttemptCount:     gate.attemptCount,
		ChargedNanoRMB:   gate.chargedNanoRMB,
		LogicalTaskCount: len(gate.logicalTaskIDs),
		ReservedNanoRMB:  gate.reservedNanoRMB,
	}
}

func (profile AITrialPricingProfile) quote(inputTokens, outputTokens int) int64 {
	return int64(inputTokens)*profile.inputNanoRMBPerToken + int64(outputTokens)*profile.outputNanoRMBPerToken
}

func (gate *AITrialBudgetGate) quote(inputTokens, outputTokens int) int64 {
	return gate.profile.quote(inputTokens, outputTokens)
}

func (usage AIModelUsage) valid() bool {
	return usage.InputTokens >= 0 && usage.OutputTokens >= 0 && usage.TotalTokens >= 0
}

func validAITrialPricingProfile(profile AITrialPricingProfile) bool {
	return profile.verified && profile.usageSupported && profile.Version != "" &&
		(profile.Model == "qwen-plus" || profile.Model == "qwen-flash") &&
		profile.MaxInputTokens == 10000 && profile.MaxOutputTokens == 5000 &&
		profile.inputNanoRMBPerToken > 0 && profile.outputNanoRMBPerToken > 0
}
