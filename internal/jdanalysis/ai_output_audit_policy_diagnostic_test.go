package jdanalysis

import (
	"errors"
	"strings"
	"testing"
)

func TestAIOutputAuditPolicyDiagnosticClassifiesPersistenceStageWithoutCause(t *testing.T) {
	internalCause := errors.New("untrusted model response must never reach diagnostic output")
	err := newAIPersistenceStageError(aiPersistenceStageCandidateMention, NewSafeAIError(AISafeErrorPolicyRejected, internalCause))

	if got := classifyAIOutputAuditPolicyDiagnostic(err); got != AIOutputAuditPolicyDiagnosticCandidateMention {
		t.Fatalf("policy diagnostic category = %q, want %q", got, AIOutputAuditPolicyDiagnosticCandidateMention)
	}
	if got := safeAIErrorCode(err); got != AISafeErrorPolicyRejected {
		t.Fatalf("safe failure code = %q, want %q", got, AISafeErrorPolicyRejected)
	}
	if strings.Contains(err.Error(), "untrusted model response") {
		t.Fatalf("staged persistence error leaked its discarded cause: %q", err)
	}
}

func TestAIOutputAuditPolicyDiagnosticUsesFixedFallback(t *testing.T) {
	if got := classifyAIOutputAuditPolicyDiagnostic(errors.New("untrusted provider detail")); got != AIOutputAuditPolicyDiagnosticUnknown {
		t.Fatalf("unknown policy diagnostic category = %q, want %q", got, AIOutputAuditPolicyDiagnosticUnknown)
	}
}
