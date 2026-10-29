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

func TestParseJobFamilyTitleClassifiesReviewedDictionaryV2AuditTitles(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		status  JobFamilyTitleStatus
		primary JobFamilyKey
	}{
		{name: "AI storage", title: "AI存储资深研发工程师（大模型）", status: JobFamilyClassified, primary: JobFamilyPlatformInfra},
		{name: "agent development", title: "Agent开发工程师（数据方向）", status: JobFamilyClassified, primary: JobFamilyBackend},
		{name: "inference optimization", title: "Infra 推理优化工程师（3D生成）", status: JobFamilyClassified, primary: JobFamilyPlatformInfra},
		{name: "cloud native", title: "K8S云原生开发", status: JobFamilyClassified, primary: JobFamilyPlatformInfra},
		{name: "posttrain research", title: "LLM Posttrain研究员", status: JobFamilyClassified, primary: JobFamilyAlgorithmResearch},
		{name: "LLM data processing", title: "LLM数据处理", status: JobFamilyClassified, primary: JobFamilyDataEngineering},
		{name: "MaaS platform", title: "MaaS平台开发工程师", status: JobFamilyClassified, primary: JobFamilyPlatformInfra},
		{name: "cloud database", title: "云数据库开发工程师", status: JobFamilyClassified, primary: JobFamilyBackend},
		{name: "cloud gaming is domain only", title: "云游戏技术专家", status: JobFamilyUnclassified},
		{name: "model inference reliability", title: "大模型推理服务可靠性工程师", status: JobFamilyClassified, primary: JobFamilyPlatformInfra},
		{name: "model routing and evaluation", title: "大模型路由与评测工程师", status: JobFamilyClassified, primary: JobFamilyPlatformInfra},
		{name: "game AI research", title: "游戏AI研发工程师（AIGC方向）-绝区零", status: JobFamilyClassified, primary: JobFamilyPlatformInfra},
		{name: "network engineer", title: "网络工程师", status: JobFamilyClassified, primary: JobFamilyOperationsSRE},
		{name: "agent research and development", title: "资深Agent研发工程师（游戏发行）", status: JobFamilyClassified, primary: JobFamilyBackend},
		{name: "search and recommendation", title: "资深搜推AI工程师", status: JobFamilyClassified, primary: JobFamilyAlgorithmResearch},
		{name: "senior game AI research", title: "资深游戏AI研发工程师", status: JobFamilyClassified, primary: JobFamilyPlatformInfra},
		{name: "high-performance network engineer", title: "高性能网络工程师", status: JobFamilyClassified, primary: JobFamilyOperationsSRE},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ParseJobFamilyTitle(test.title)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != test.status || result.PrimaryFamilyKey != test.primary {
				t.Fatalf("ParseJobFamilyTitle(%q) = %#v, want status %q and primary %q", test.title, result, test.status, test.primary)
			}
		})
	}
}

func TestParseJobFamilyTitleAppliesReviewedDictionaryV2CrossFamilyRules(t *testing.T) {
	agentDevelopment, err := ParseJobFamilyTitle("Agent开发工程师")
	if err != nil {
		t.Fatal(err)
	}
	if agentDevelopment.FamilyMatchCounts[JobFamilyBackend] != 1 || agentDevelopment.FamilyMatchCounts[JobFamilyPlatformInfra] != 1 || agentDevelopment.PrimaryFamilyKey != JobFamilyBackend {
		t.Fatalf("Agent development = %#v, want backend:1 platform_infra:1 with backend primary", agentDevelopment)
	}

	cloudNative, err := ParseJobFamilyTitle("K8S云原生开发")
	if err != nil {
		t.Fatal(err)
	}
	if cloudNative.FamilyMatchCounts[JobFamilyOperationsSRE] != 1 || cloudNative.FamilyMatchCounts[JobFamilyPlatformInfra] != 1 || cloudNative.PrimaryFamilyKey != JobFamilyPlatformInfra {
		t.Fatalf("cloud native = %#v, want operations_sre:1 platform_infra:1 with platform_infra primary", cloudNative)
	}

	aiops, err := ParseJobFamilyTitle("AIOps工程师")
	if err != nil {
		t.Fatal(err)
	}
	if aiops.FamilyMatchCounts[JobFamilyOperationsSRE] != 1 || aiops.FamilyMatchCounts[JobFamilyPlatformInfra] != 0 || aiops.PrimaryFamilyKey != JobFamilyOperationsSRE {
		t.Fatalf("AIOps = %#v, want only bounded operations_sre match", aiops)
	}
}
