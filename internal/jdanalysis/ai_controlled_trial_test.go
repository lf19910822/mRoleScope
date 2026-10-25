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
