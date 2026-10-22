package jdanalysis

import (
	"testing"
	"time"
)

func TestMatcherUsesOnlyApprovedRulesAndApprovedBodyFields(t *testing.T) {
	candidateMatcher, err := NewMatcher(CandidateV01Seed().Rules)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := candidateMatcher.MatchField(SourceFieldDescription, "Go Docker")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("candidate rules emitted mentions: %#v", matches)
	}

	matcher := mustMatcher(t, approvedRule("docker", "Docker", "Docker", CaseInsensitive, BoundaryASCIIIdentifier))
	matches, err = matcher.MatchField(SourceFieldJobRequire, "docker and DOCKER")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].TechnologyKey != "docker" || matches[1].TechnologyKey != "docker" {
		t.Fatalf("case-specific rule results = %#v", matches)
	}
	if _, err := matcher.MatchField("title", "Docker"); err == nil {
		t.Fatal("title was accepted as a body analysis field")
	}
}

func TestMatcherHonorsBoundariesAndDoesNotInferEcosystemSkills(t *testing.T) {
	matcher := mustMatcher(t,
		approvedRule("go", "Go", "Go", CaseSensitive, BoundaryASCIIIdentifier),
		approvedRule("java", "Java", "Java", CaseSensitive, BoundaryASCIIIdentifier),
		approvedRule("python", "Python", "Python", CaseSensitive, BoundaryASCIIIdentifier),
		approvedRule("kubernetes", "Kubernetes", "Kubernetes", CaseSensitive, BoundaryASCIIIdentifier),
	)
	matches, err := matcher.MatchField(SourceFieldDescription, "Go and Go-zero; JavaScript and Java; PyTorch; Kubelet; Kubernetes; AI")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 || matches[0].TechnologyKey != "go" || matches[1].TechnologyKey != "java" || matches[2].TechnologyKey != "kubernetes" {
		t.Fatalf("boundary and inference result = %#v", matches)
	}
}

func TestMatcherPrefersLongestRegisteredOverlapAndKeepsAliases(t *testing.T) {
	matcher := mustMatcher(t,
		approvedRule("tensorrt", "TensorRT", "TensorRT", CaseSensitive, BoundaryExact),
		approvedRule("tensorrt-llm", "TensorRT-LLM", "TensorRT-LLM", CaseSensitive, BoundaryCustom),
		approvedRule("go", "Go", "Golang", CaseSensitive, BoundaryASCIIIdentifier),
	)
	matches, err := matcher.MatchField(SourceFieldAddition, "TensorRT-LLM with Golang")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches = %#v", matches)
	}
	if matches[0].TechnologyKey != "tensorrt-llm" || matches[0].MatchedAlias != "TensorRT-LLM" {
		t.Fatalf("longest overlap was not retained: %#v", matches[0])
	}
	if matches[1].TechnologyKey != "go" || matches[1].MatchedAlias != "Golang" {
		t.Fatalf("alias was not retained: %#v", matches[1])
	}
}

func TestMatcherKeepsEachFieldIndependent(t *testing.T) {
	matcher := mustMatcher(t, approvedRule("go", "Go", "Go", CaseSensitive, BoundaryASCIIIdentifier))
	matches, err := matcher.MatchBodyFields(map[string]string{
		SourceFieldDescription: "中文Go",
		SourceFieldJobRequire:  "Go",
		SourceFieldAddition:    "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || matches[0].SourceField != SourceFieldDescription || matches[1].SourceField != SourceFieldJobRequire {
		t.Fatalf("field-independent matches = %#v", matches)
	}
}

func mustMatcher(t *testing.T, rules ...DictionaryRule) Matcher {
	t.Helper()
	matcher, err := NewMatcher(rules)
	if err != nil {
		t.Fatal(err)
	}
	return matcher
}

func approvedRule(key, canonical, alias string, caseMode CaseMode, boundary BoundaryMode) DictionaryRule {
	return DictionaryRule{
		TechnologyKey: key, CanonicalName: canonical, TechnologyKind: "synthetic",
		RuleKey: key + ".01", Alias: alias, State: RuleEnabled,
		CaseMode: caseMode, BoundaryMode: boundary, OverlapPriority: len(alias), Summary: "synthetic test rule",
		Approval: &Approval{Reviewer: "reviewer", Decided: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC), Rationale: "synthetic approval"},
	}
}
