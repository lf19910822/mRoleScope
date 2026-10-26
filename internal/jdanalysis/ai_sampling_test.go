package jdanalysis

import (
	"fmt"
	"testing"
)

func TestSelectAITrialSampleV2FreezesFiveNonTextBodyIDs(t *testing.T) {
	if aiTrialSampleSize != 5 {
		t.Fatalf("trial sample capacity = %d, want 5", aiTrialSampleSize)
	}
	candidates := make([]AIBodySamplingMetric, 0, 6)
	for index := 1; index <= 6; index++ {
		candidates = append(candidates, AIBodySamplingMetric{
			BodyContentVersionID: "body-" + twoDigit(index),
			BodyLengthCodePoints: 100 + index*75,
			CompleteFieldCount:   index % 4,
			VerifiedMentionCount: index % 7,
		})
	}

	first, err := SelectAITrialSampleV2(candidates)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SelectAITrialSampleV2(candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 5 || !equalStrings(first, second) {
		t.Fatalf("sample = %#v, want deterministic 5 IDs", first)
	}
	allCandidates := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		allCandidates[candidate.BodyContentVersionID] = struct{}{}
	}
	for _, bodyID := range first {
		if _, exists := allCandidates[bodyID]; !exists {
			t.Fatalf("sample included a body outside the frozen candidate set: %q", bodyID)
		}
	}

	if _, err := SelectAITrialSampleV2(candidates[:4]); err == nil {
		t.Fatal("a trial sample expanded a sub-five candidate set")
	}
}

func TestSelectAITrialSampleV1PreservesThirtyBodyHistoricalPolicy(t *testing.T) {
	candidates := make([]AIBodySamplingMetric, 0, 30)
	for index := 1; index <= 30; index++ {
		candidates = append(candidates, AIBodySamplingMetric{
			BodyContentVersionID: fmt.Sprintf("historical-body-%02d", index),
		})
	}
	sample, err := SelectAITrialSampleV1(candidates)
	if err != nil || len(sample) != aiLegacyTrialSampleSizeV1 {
		t.Fatalf("historical v1 sample = %#v, %v; want 30 frozen IDs", sample, err)
	}
}

func TestSelectAIOutputAuditDiagnosticSampleV1FreezesFiveNonTextBodyIDs(t *testing.T) {
	candidates := make([]AIBodySamplingMetric, 0, 6)
	for index := 1; index <= 6; index++ {
		candidates = append(candidates, AIBodySamplingMetric{
			BodyContentVersionID: fmt.Sprintf("body-%02d", index),
			BodyLengthCodePoints: 100 + index*75,
			CompleteFieldCount:   index % 4,
			VerifiedMentionCount: index % 7,
		})
	}

	first, err := SelectAIOutputAuditDiagnosticSampleV1(candidates)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SelectAIOutputAuditDiagnosticSampleV1(candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != aiOutputAuditDiagnosticSampleSize || !equalStrings(first, second) {
		t.Fatalf("diagnostic sample = %#v, want deterministic %d IDs", first, aiOutputAuditDiagnosticSampleSize)
	}
	if _, err := SelectAIOutputAuditDiagnosticSampleV1(candidates[:aiOutputAuditDiagnosticSampleSize-1]); err == nil {
		t.Fatal("a diagnostic sample expanded a sub-five candidate set")
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
