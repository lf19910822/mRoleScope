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

func TestV04MatcherSeparatesReactAndReAct(t *testing.T) {
	matcher := mustMatcher(t,
		approvedRule("react", "React", "React", CaseSensitive, BoundaryASCIIIdentifier),
		approvedRule("react", "React", "react", CaseSensitive, BoundaryASCIIIdentifier),
		approvedRule("react-agent", "ReAct", "ReAct", CaseSensitive, BoundaryASCIIIdentifier),
	)
	matches, err := matcher.MatchField(SourceFieldJobRequire, "React react ReAct REACT")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 {
		t.Fatalf("React/ReAct matches = %#v", matches)
	}
	if matches[0].TechnologyKey != "react" || matches[1].TechnologyKey != "react" || matches[2].TechnologyKey != "react-agent" {
		t.Fatalf("React/ReAct identities = %#v", matches)
	}
}

func TestV04MatcherExpandsUnrealVersionWithoutDroppingBaseIdentity(t *testing.T) {
	base := approvedRule("unreal-engine", "Unreal Engine", "Unreal", CaseSensitive, BoundaryASCIIIdentifier)
	version := approvedRule("unreal-engine-5", "Unreal Engine 5", "UE5", CaseSensitive, BoundaryASCIIIdentifier)
	version.Emissions = []DictionaryEmission{
		{TechnologyKey: "unreal-engine", CanonicalName: "Unreal Engine", TechnologyKind: "game-engine", RelationType: "inherited"},
		{TechnologyKey: "unreal-engine-5", CanonicalName: "Unreal Engine 5", TechnologyKind: "game-engine", RelationType: "self"},
	}
	matcher := mustMatcher(t, base, version)
	matches, err := matcher.MatchField(SourceFieldDescription, "熟悉 UE5")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("UE5 expansion = %#v", matches)
	}
	if matches[0].TechnologyKey != "unreal-engine" || matches[1].TechnologyKey != "unreal-engine-5" {
		t.Fatalf("UE5 identities = %#v", matches)
	}
	if matches[0].ByteStart != matches[1].ByteStart || matches[0].ByteEnd != matches[1].ByteEnd {
		t.Fatalf("UE5 emissions do not share source span: %#v", matches)
	}
}

func TestV04MatcherAppliesOnlyExplicitLocalExclusions(t *testing.T) {
	rule := approvedRule("react", "React", "react", CaseSensitive, BoundaryASCIIIdentifier)
	matcher, err := NewMatcherWithExclusions([]DictionaryRule{rule}, []DictionaryExclusionPattern{{
		PatternKey: "react.to.01", SourceRuleKey: rule.RuleKey, Pattern: "react to",
		CaseMode: CaseSensitive, BoundaryMode: BoundaryExact, WindowCodePoints: 96, Provenance: "synthetic",
		State: RuleEnabled, Approval: &Approval{Reviewer: "reviewer", Decided: rule.Approval.Decided, Rationale: "synthetic"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	matches, err := matcher.MatchField(SourceFieldDescription, "熟悉 react。系统会 react to user input。熟悉 react")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("excluded react matches = %#v", matches)
	}
}

func TestV04MatcherSuppressesUEOnlyWhenUserExperienceIsInTheLocalWindow(t *testing.T) {
	rule := approvedRule("unreal-engine", "Unreal Engine", "UE", CaseSensitive, BoundaryASCIIIdentifier)
	matcher, err := NewMatcherWithExclusions([]DictionaryRule{rule}, []DictionaryExclusionPattern{{
		PatternKey: "unreal.user-experience.01", SourceRuleKey: rule.RuleKey, Pattern: "user experience",
		CaseMode: CaseInsensitive, BoundaryMode: BoundaryExact, WindowCodePoints: 96, Provenance: "synthetic",
		State: RuleEnabled, Approval: &Approval{Reviewer: "reviewer", Decided: rule.Approval.Decided, Rationale: "synthetic"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	matches, err := matcher.MatchField(SourceFieldDescription, "熟悉 UE，负责 user experience 设计，熟悉 UE")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("UE ordinary-language exclusion = %#v, want no matches", matches)
	}

	matches, err = matcher.MatchField(SourceFieldDescription, "熟悉 UE。负责客户端开发。")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].TechnologyKey != "unreal-engine" {
		t.Fatalf("standalone UE match = %#v", matches)
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
