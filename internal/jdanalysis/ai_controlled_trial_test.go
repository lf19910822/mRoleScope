package jdanalysis

import (
	"strings"
	"testing"
)

func TestControlledAIInputContractBindsCVMOutputAuditPolicy(t *testing.T) {
	want := sha256Hex(strings.Join([]string{
		AIInputContractVersionV1,
		AISpanContractVersionV1,
		AIInputTokenEstimatorVersionV3,
		AIInputTokenEstimatorSHA256(),
		"formal_trial_sampling_version=" + aiTrialSamplingVersionV2,
		"formal_trial_sample_size=5",
		"formal_trial_max_logical_tasks=10",
		"formal_trial_max_attempts=30",
		AIOutputAuditPolicyVersionV1,
		"retention=168h",
		"capacity_bytes=67108864",
		"response_reservation_bytes=1052672",
	}, "|"))
	if got := controlledAIInputContractSHA256(); got != want {
		t.Fatalf("controlled input-contract digest = %q, want audit-bound digest %q", got, want)
	}
}

func TestControlledAIOutputAuditAcceptsOnlyFrozenInputContracts(t *testing.T) {
	if !validControlledAIOutputAuditInputContractSHA256(controlledAIInputContractSHA256()) {
		t.Fatal("baseline audited trial contract was rejected")
	}
	if !validControlledAIOutputAuditInputContractSHA256(controlledAIOutputAuditDiagnosticInputContractSHA256()) {
		t.Fatal("five-body diagnostic contract was rejected")
	}
	if !validControlledAIOutputAuditInputContractSHA256(legacyThirtyBodyControlledAIInputContractSHA256()) {
		t.Fatal("historical 30-body contract was rejected")
	}
	if !validControlledAIOutputAuditInputContractSHA256(legacyThirtyBodyControlledAIOutputAuditDiagnosticInputContractSHA256()) {
		t.Fatal("historical diagnostic contract was rejected")
	}
	if validControlledAIOutputAuditInputContractSHA256(sha256Hex("uncontrolled-contract")) {
		t.Fatal("uncontrolled audit contract was accepted")
	}
}

func TestControlledAITrialConfigurationDigestChangesFromThirtyBodyTrialPolicy(t *testing.T) {
	profile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	current := controlledAITrialConfigurationInput("technology-configuration-id", profile, QwenPlusTrialPricingProfile())
	historical := current
	historical.InputContractSHA256 = legacyThirtyBodyControlledAIInputContractSHA256()
	historical.ConfigurationSHA256 = candidateAIConfigurationSHA256(historical)
	if current.ConfigurationSHA256 == historical.ConfigurationSHA256 {
		t.Fatal("five-body controlled configuration reused the historical 30-body identity")
	}
}

func TestControlledAITrialConfigurationInputUsesV3FrozenContract(t *testing.T) {
	profile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	input := controlledAITrialConfigurationInput("technology-configuration-id", profile, QwenPlusTrialPricingProfile())
	if input.PromptSHA256 != AISystemPromptV3SHA256() || input.ResponseSchemaVersion != AIResponseSchemaVersionV3 || input.ResponseSchemaSHA256 != AIResponseSchemaV3SHA256() || input.ValidatorVersion != AIValidatorVersionV3 {
		t.Fatalf("controlled trial input = %#v, want the frozen v3 contract", input)
	}
	if input.RequestProfileSHA256 != profile.SHA256() || input.PricingProfileSHA256 != QwenPlusTrialPricingProfile().SHA256() {
		t.Fatal("controlled trial input did not bind v3 request/pricing profile digests")
	}
}

func TestControlledAITrialConfigurationDigestChangesFromHistoricalOutputBudget(t *testing.T) {
	profile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	current := controlledAITrialConfigurationInput("technology-configuration-id", profile, QwenPlusTrialPricingProfile())

	historicalProfile := profile
	historicalProfile.maxOutputTokens = 2500
	historicalPricing := QwenPlusTrialPricingProfile()
	historicalPricing.MaxOutputTokens = 2500
	historicalPricing.Version = "bailian-beijing-short-context-2026-10-23-v1"
	historical := CandidateAIConfigurationInput{
		TechnologyConfigurationID: current.TechnologyConfigurationID,
		Provider:                  current.Provider,
		Model:                     current.Model,
		RequestProfileSHA256:      historicalProfile.SHA256(),
		PromptSHA256:              AISystemPromptV1SHA256(),
		ResponseSchemaVersion:     AIResponseSchemaVersionV1,
		ResponseSchemaSHA256:      AIResponseSchemaV1SHA256(),
		InputContractVersion:      current.InputContractVersion,
		InputContractSHA256:       current.InputContractSHA256,
		ValidatorVersion:          AIValidatorVersionV1,
		ValidatorSHA256:           sha256Hex(AIValidatorVersionV1),
		PricingProfileVersion:     historicalPricing.Version,
		PricingProfileSHA256:      historicalPricing.SHA256(),
		CreatedBy:                 current.CreatedBy,
	}
	historical.ConfigurationSHA256 = candidateAIConfigurationSHA256(historical)
	if current.ConfigurationSHA256 == historical.ConfigurationSHA256 {
		t.Fatal("v3 output budget and contract reused the historical configuration digest")
	}
}

func TestControlledAIOutputAuditDiagnosticContractBindsPersistableExecution(t *testing.T) {
	want := sha256Hex(strings.Join([]string{
		controlledAIInputContractSHA256(),
		"diagnostic_execution=plus_only",
		"diagnostic_sample_size=5",
		"diagnostic_output_audit_execution=ai-output-audit-root-mount-v1",
	}, "|"))
	if got := controlledAIOutputAuditDiagnosticInputContractSHA256(); got != want {
		t.Fatalf("output-audit diagnostic contract = %q, want persistable-execution contract %q", got, want)
	}
}

func TestControlledAIConfigurationInputsFreezeActiveV2PromptIdentity(t *testing.T) {
	plusProfile, err := QwenPlusTrialProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	flashProfile, err := QwenFlashComparisonProfileForBeijingWorkspace("llm-mrolescope")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []CandidateAIConfigurationInput{
		controlledAITrialConfigurationInput("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", plusProfile, QwenPlusTrialPricingProfile()),
		controlledAIOutputAuditDiagnosticConfigurationInput("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", plusProfile, QwenPlusTrialPricingProfile()),
		controlledAITrialConfigurationInput("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", flashProfile, QwenFlashComparisonPricingProfile()),
	} {
		if input.PromptSHA256 != AISystemPromptV2SHA256() {
			t.Fatalf("controlled configuration prompt SHA-256 = %q, want active V2 %q", input.PromptSHA256, AISystemPromptV2SHA256())
		}
		if input.PromptSHA256 == AISystemPromptV1SHA256() {
			t.Fatal("controlled configuration reused the historic V1 prompt identity")
		}
	}
}
