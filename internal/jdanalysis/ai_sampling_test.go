package jdanalysis

import "testing"

func TestSelectAITrialSampleV1FreezesThirtyNonTextBodyIDs(t *testing.T) {
	candidates := make([]AIBodySamplingMetric, 0, 35)
	for index := 1; index <= 35; index++ {
		candidates = append(candidates, AIBodySamplingMetric{
			BodyContentVersionID: "body-" + twoDigit(index),
			BodyLengthCodePoints: 100 + index*75,
			CompleteFieldCount:   index % 4,
			VerifiedMentionCount: index % 7,
		})
	}

	first, err := SelectAITrialSampleV1(candidates)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SelectAITrialSampleV1(candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 30 || !equalStrings(first, second) {
		t.Fatalf("sample = %#v, want deterministic 30 IDs", first)
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

	if _, err := SelectAITrialSampleV1(candidates[:29]); err == nil {
		t.Fatal("a trial sample expanded a sub-30 candidate set")
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
