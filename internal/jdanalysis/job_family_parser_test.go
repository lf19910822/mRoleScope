package jdanalysis

import (
	"encoding/json"
	"testing"
)

func TestParseJobFamilyTitleRetainsCrossFamilyMatchesAndChoosesLaterTie(t *testing.T) {
	result, err := ParseJobFamilyTitle("服务端运维")
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != JobFamilyClassified {
		t.Fatalf("status = %q, want %q", result.Status, JobFamilyClassified)
	}
	if result.PrimaryFamilyKey != JobFamilyOperationsSRE {
		t.Fatalf("primary family = %q, want %q", result.PrimaryFamilyKey, JobFamilyOperationsSRE)
	}
	if result.FamilyMatchCounts[JobFamilyBackend] != 1 {
		t.Fatalf("backend count = %d, want 1", result.FamilyMatchCounts[JobFamilyBackend])
	}
	if result.FamilyMatchCounts[JobFamilyOperationsSRE] != 1 {
		t.Fatalf("operations_sre count = %d, want 1", result.FamilyMatchCounts[JobFamilyOperationsSRE])
	}
}

func TestParseJobFamilyTitleClassifiesReviewedFamilies(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		primary JobFamilyKey
	}{
		{name: "algorithm research", title: "模型研究工程师", primary: JobFamilyAlgorithmResearch},
		{name: "backend", title: "后端开发工程师", primary: JobFamilyBackend},
		{name: "game client", title: "游戏客户端开发", primary: JobFamilyClient},
		{name: "data engineering", title: "ETL工程师", primary: JobFamilyDataEngineering},
		{name: "engineering management", title: "外包管理", primary: JobFamilyEngineeringManagement},
		{name: "engine systems", title: "游戏引擎开发", primary: JobFamilyEngineSystems},
		{name: "frontend", title: "前端开发", primary: JobFamilyFrontend},
		{name: "hardware embedded", title: "嵌入式软件工程师", primary: JobFamilyHardwareEmbedded},
		{name: "operations sre", title: "sre工程师", primary: JobFamilyOperationsSRE},
		{name: "platform infrastructure", title: "训练平台研发", primary: JobFamilyPlatformInfra},
		{name: "security", title: "漏洞挖掘工程师", primary: JobFamilySecurity},
		{name: "testing quality", title: "自动化测试工程师", primary: JobFamilyTestingQuality},
		{name: "toolchain", title: "客户端工具开发", primary: JobFamilyToolchain},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ParseJobFamilyTitle(test.title)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != JobFamilyClassified || result.PrimaryFamilyKey != test.primary {
				t.Fatalf("ParseJobFamilyTitle(%q) = %#v, want classified primary %q", test.title, result, test.primary)
			}
		})
	}
}

func TestParseJobFamilyTitleMasksReviewedProjectsAndKeepsOtherFamilyEvidence(t *testing.T) {
	result, err := ParseJobFamilyTitle("全栈开发-AI引擎")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != JobFamilyClassified || result.PrimaryFamilyKey != JobFamilyBackend || result.FamilyMatchCounts[JobFamilyBackend] != 1 {
		t.Fatalf("masked project title = %#v, want one backend match", result)
	}
	if result.FamilyMatchCounts[JobFamilyEngineSystems] != 0 {
		t.Fatalf("masked project title engine count = %d, want 0", result.FamilyMatchCounts[JobFamilyEngineSystems])
	}
}

func TestParseJobFamilyTitleKeepsCrossFamilyOverlapAndBoundsASCIIAliases(t *testing.T) {
	overlapped, err := ParseJobFamilyTitle("客户端安全开发")
	if err != nil {
		t.Fatal(err)
	}
	if overlapped.FamilyMatchCounts[JobFamilyClient] != 1 || overlapped.FamilyMatchCounts[JobFamilySecurity] != 1 || overlapped.PrimaryFamilyKey != JobFamilySecurity {
		t.Fatalf("cross-family overlap = %#v, want client:1 security:1 primary security", overlapped)
	}

	notSRE, err := ParseJobFamilyTitle("SRETools工程师")
	if err != nil {
		t.Fatal(err)
	}
	if notSRE.Status != JobFamilyUnclassified {
		t.Fatalf("identifier substring status = %q, want %q", notSRE.Status, JobFamilyUnclassified)
	}
}

func TestJobFamilyDictionaryRejectsCaseInsensitiveCrossFamilyAliasOwnership(t *testing.T) {
	asset := jobFamilyDictionaryAsset{}
	if err := json.Unmarshal(deterministicJobFamilyDictionaryAsset, &asset); err != nil {
		t.Fatal(err)
	}
	for index := range asset.Families {
		if asset.Families[index].FamilyKey == JobFamilyBackend {
			asset.Families[index].Aliases = append(asset.Families[index].Aliases, jobFamilyAlias{
				Value:                   "sre",
				ASCIICaseInsensitive:    true,
				ASCIIIdentifierBoundary: true,
			})
			break
		}
	}
	if err := validateJobFamilyDictionary(asset); err == nil {
		t.Fatal("case-insensitive cross-family alias ownership was accepted")
	}
}

func TestParseJobFamilyTitleDoesNotExpandUnlistedTermsOrOvermaskGenericRoleWords(t *testing.T) {
	unlisted, err := ParseJobFamilyTitle("接口开发工程师")
	if err != nil {
		t.Fatal(err)
	}
	if unlisted.Status != JobFamilyUnclassified || unlisted.PrimaryFamilyKey != "" {
		t.Fatalf("unlisted title = %#v, want completed unclassified result", unlisted)
	}

	engine, err := ParseJobFamilyTitle("游戏引擎开发")
	if err != nil {
		t.Fatal(err)
	}
	if engine.Status != JobFamilyClassified || engine.FamilyMatchCounts[JobFamilyEngineSystems] != 1 {
		t.Fatalf("generic engine role = %#v, want one unmasked engine-systems match", engine)
	}
}
