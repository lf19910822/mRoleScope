package jdanalysis

import (
	"testing"
	"time"
)

func TestCandidateV04SeedContainsMandatoryExecutableScope(t *testing.T) {
	seed := CandidateV04Seed()
	if seed.VersionLabel != "technology-keyword-alias-map-v0.4" {
		t.Fatalf("v0.4 version label = %q", seed.VersionLabel)
	}
	if err := seed.Validate(); err != nil {
		t.Fatalf("validate v0.4 seed: %v", err)
	}

	wantedAliases := []string{
		"UE", "UE4", "UE5", "UE5.4", "Unity 3D", "Unity3D", "ReAct",
		"UObject", "World Partition", "WorldPartition", "StateTree", "MassEntity", "GameThread", "BuildGraph",
		"Wwise", "FMOD", "FMOD Studio", "CRIWARE", "Criware", "Maya",
		"DCGM", "Nsight Systems", "Nsight Compute", "NVSwitch", "CANN", "NUMA", "PCIe", "RoCE", "ROCE",
		"kubelet", "kube-scheduler", "controller-manager", "KubeFed", "controller-runtime", "client-go", "KEDA",
		"Elasticsearch", "Ceph", "OpenStack", "QEMU", "libvirt", "cloud-init",
	}
	aliases := make(map[string]DictionaryRule, len(seed.Rules))
	for _, rule := range seed.Rules {
		aliases[rule.Alias] = rule
	}
	for _, alias := range wantedAliases {
		rule, ok := aliases[alias]
		if !ok {
			t.Fatalf("v0.4 seed omitted mandatory alias %q", alias)
		}
		if rule.State != RuleCandidate {
			t.Fatalf("v0.4 alias %q state = %q, want candidate before fixed approval", alias, rule.State)
		}
	}
}

func TestCandidateV04SeedModelsUnrealIdentityExpansion(t *testing.T) {
	seed := CandidateV04Seed()
	var found bool
	for _, rule := range seed.Rules {
		if rule.Alias != "UE5" {
			continue
		}
		found = true
		if len(rule.Emissions) != 2 {
			t.Fatalf("UE5 emissions = %#v, want base and major-version identities", rule.Emissions)
		}
		if rule.Emissions[0].TechnologyKey != "unreal-engine" || rule.Emissions[1].TechnologyKey != "unreal-engine-5" {
			t.Fatalf("UE5 emissions = %#v", rule.Emissions)
		}
	}
	if !found {
		t.Fatal("v0.4 seed omitted UE5")
	}
}

func TestDictionarySeedRejectsInvalidEmissionReference(t *testing.T) {
	seed := DictionarySeed{
		VersionLabel:   "synthetic",
		SourceDocument: "synthetic.md",
		Rules: []DictionaryRule{{
			TechnologyKey: "source", CanonicalName: "Source", TechnologyKind: "synthetic",
			RuleKey: "source.01", Alias: "source", State: RuleCandidate,
			CaseMode: CaseSensitive, BoundaryMode: BoundaryExact, Summary: "synthetic",
			Emissions: []DictionaryEmission{{
				TechnologyKey: "missing", CanonicalName: "Missing", TechnologyKind: "synthetic",
				RelationType: "inherited",
			}},
		}},
	}
	if err := seed.Validate(); err == nil {
		t.Fatal("invalid emission reference was accepted")
	}
}

func TestDictionarySeedRejectsDuplicateExclusionPattern(t *testing.T) {
	seed := DictionarySeed{
		VersionLabel:   "synthetic",
		SourceDocument: "synthetic.md",
		Rules: []DictionaryRule{{
			TechnologyKey: "react", CanonicalName: "React", TechnologyKind: "synthetic",
			RuleKey: "react.01", Alias: "react", State: RuleCandidate,
			CaseMode: CaseSensitive, BoundaryMode: BoundaryASCIIIdentifier, Summary: "synthetic",
		}},
		ExclusionPatterns: []DictionaryExclusionPattern{
			{PatternKey: "react.to.01", SourceRuleKey: "react.01", Pattern: "react to", CaseMode: CaseSensitive, BoundaryMode: BoundaryExact, WindowCodePoints: 96, Provenance: "synthetic"},
			{PatternKey: "react.to.01", SourceRuleKey: "react.01", Pattern: "react to", CaseMode: CaseSensitive, BoundaryMode: BoundaryExact, WindowCodePoints: 96, Provenance: "synthetic"},
		},
	}
	if err := seed.Validate(); err == nil {
		t.Fatal("duplicate exclusion pattern was accepted")
	}
}

func TestCandidateV04MandatoryAliasesAreExecutableWhenApproved(t *testing.T) {
	seed := CandidateV04Seed()
	rulesByAlias := make(map[string]DictionaryRule, len(seed.Rules))
	for _, rule := range seed.Rules {
		rulesByAlias[rule.Alias] = rule
	}

	aliases := []string{
		"UObject", "World Partition", "WorldPartition", "StateTree", "MassEntity", "GameThread", "BuildGraph",
		"Wwise", "FMOD", "FMOD Studio", "CRIWARE", "Criware", "Maya",
		"DCGM", "Nsight Systems", "Nsight Compute", "NVSwitch", "CANN", "NUMA", "PCIe", "RoCE", "ROCE",
		"kubelet", "kube-scheduler", "controller-manager", "KubeFed", "controller-runtime", "client-go", "KEDA",
		"Elasticsearch", "Ceph", "OpenStack", "QEMU", "libvirt", "cloud-init",
	}
	rules := make([]DictionaryRule, 0, len(aliases))
	for _, alias := range aliases {
		rule, ok := rulesByAlias[alias]
		if !ok {
			t.Fatalf("v0.4 alias %q is not registered", alias)
		}
		rule.State = RuleEnabled
		rule.Approval = &Approval{
			Reviewer:  "reviewer",
			Decided:   time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC),
			Rationale: "synthetic approval",
		}
		rules = append(rules, rule)
	}

	matcher, err := NewMatcher(rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range aliases {
		matches, err := matcher.MatchField(SourceFieldDescription, alias)
		if err != nil {
			t.Fatalf("match %q: %v", alias, err)
		}
		if len(matches) == 0 {
			t.Fatalf("approved v0.4 alias %q produced no match", alias)
		}
	}
}

func TestV04DictionaryFingerprintIncludesPolicyRelations(t *testing.T) {
	seed := CandidateV04Seed()
	original, err := dictionarySeedSHA256(seed)
	if err != nil {
		t.Fatal(err)
	}

	exclusionChanged := CandidateV04Seed()
	exclusionChanged.ExclusionPatterns[0].Pattern = "react to input"
	exclusionFingerprint, err := dictionarySeedSHA256(exclusionChanged)
	if err != nil {
		t.Fatal(err)
	}
	if exclusionFingerprint == original {
		t.Fatal("changing an exclusion policy did not change the dictionary fingerprint")
	}

	emissionChanged := CandidateV04Seed()
	for index := range emissionChanged.Rules {
		if emissionChanged.Rules[index].RuleKey != "unreal-engine-5.01" {
			continue
		}
		emissionChanged.Rules[index].Emissions[1].RelationType = "inherited"
		break
	}
	emissionFingerprint, err := dictionarySeedSHA256(emissionChanged)
	if err != nil {
		t.Fatal(err)
	}
	if emissionFingerprint == original {
		t.Fatal("changing an emission relation did not change the dictionary fingerprint")
	}
}
