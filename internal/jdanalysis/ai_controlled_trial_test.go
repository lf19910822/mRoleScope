package jdanalysis

import (
	"strings"
	"testing"
)

func TestControlledAIInputContractBindsCVMOutputAuditPolicy(t *testing.T) {
	want := sha256Hex(strings.Join([]string{
		AIInputContractVersionV1,
		AISpanContractVersionV1,
		AIInputTokenEstimatorVersionV2,
		AIInputTokenEstimatorSHA256(),
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
	if validControlledAIOutputAuditInputContractSHA256(sha256Hex("uncontrolled-contract")) {
		t.Fatal("uncontrolled audit contract was accepted")
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
