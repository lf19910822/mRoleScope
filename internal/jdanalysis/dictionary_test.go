package jdanalysis

import (
	"testing"
	"time"
)

func TestCandidateV01SeedIsStructuredAndNonExecutable(t *testing.T) {
	seed := CandidateV01Seed()
	if err := seed.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(seed.Rules) < 100 || len(seed.Candidates) < 20 {
		t.Fatalf("v0.1 seed is unexpectedly incomplete: rules=%d candidates=%d", len(seed.Rules), len(seed.Candidates))
	}
	if executable := seed.ExecutableRules(); len(executable) != 0 {
		t.Fatalf("unapproved v0.1 seed produced executable rules: %d", len(executable))
	}

	seenKeys := map[string]struct{}{}
	for _, rule := range seed.Rules {
		if _, duplicate := seenKeys[rule.RuleKey]; duplicate {
			t.Fatalf("duplicate rule key %q", rule.RuleKey)
		}
		seenKeys[rule.RuleKey] = struct{}{}
	}
}

func TestEnabledRuleRequiresExplicitHumanApproval(t *testing.T) {
	rule := DictionaryRule{
		TechnologyKey: "go", CanonicalName: "Go", TechnologyKind: "programming-language",
		RuleKey: "go.01", Alias: "Go", State: RuleEnabled,
		CaseMode: CaseSensitive, BoundaryMode: BoundaryASCIIIdentifier, Summary: "synthetic test rule",
	}
	if err := rule.Validate(); err == nil || rule.Executable() {
		t.Fatal("enabled rule without an approval was accepted")
	}
	rule.Approval = &Approval{Reviewer: "reviewer", Decided: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC), Rationale: "synthetic approval"}
	if err := rule.Validate(); err != nil || !rule.Executable() {
		t.Fatalf("approved enabled rule was not executable: %v", err)
	}
}

func TestV01KeepsConfusableTechnologiesSeparate(t *testing.T) {
	seed := CandidateV01Seed()
	keys := make(map[string]struct{})
	for _, rule := range seed.Rules {
		keys[rule.TechnologyKey] = struct{}{}
	}
	for _, key := range []string{"java", "javascript", "typescript", "cpp", "csharp", "react", "tensorrt", "tensorrt-llm"} {
		if _, exists := keys[key]; !exists {
			t.Fatalf("expected distinct v0.1 technology key %q", key)
		}
	}
}
