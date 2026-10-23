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

func TestCandidateV02SeedPreservesV01AndApprovesReviewedKubernetesAliases(t *testing.T) {
	v01 := CandidateV01Seed()
	v02 := CandidateV02Seed()
	if v01.VersionLabel != "technology-keyword-alias-map-v0.1" {
		t.Fatalf("v0.1 label changed: %q", v01.VersionLabel)
	}
	if v02.VersionLabel != "technology-keyword-alias-map-v0.2" {
		t.Fatalf("v0.2 label = %q", v02.VersionLabel)
	}
	if err := v02.Validate(); err != nil {
		t.Fatalf("v0.2 seed is invalid: %v", err)
	}

	aliases := make(map[string]DictionaryRule)
	for _, rule := range v02.Rules {
		if rule.TechnologyKey == "kubernetes" {
			aliases[rule.Alias] = rule
		}
	}
	for _, alias := range []string{"Kubernetes", "K8s", "K8S"} {
		rule, exists := aliases[alias]
		if !exists {
			t.Fatalf("v0.2 does not register Kubernetes alias %q", alias)
		}
		if rule.State != RuleCandidate || rule.CaseMode != CaseSensitive || rule.BoundaryMode != BoundaryASCIIIdentifier {
			t.Fatalf("Kubernetes alias %q has unreviewed matching policy: %#v", alias, rule)
		}
	}

	approved := make(map[string]struct{})
	for _, ruleKey := range V02ApprovedRuleKeys() {
		approved[ruleKey] = struct{}{}
	}
	for _, ruleKey := range []string{"kubernetes.01", "kubernetes.02", "kubernetes.03"} {
		if _, exists := approved[ruleKey]; !exists {
			t.Fatalf("reviewed Kubernetes rule %q is not approval-eligible", ruleKey)
		}
	}
}

func TestCandidateV02SeedRegistersPytouchOnlyAsReviewedExactAlias(t *testing.T) {
	seed := CandidateV02Seed()
	var pytouch *DictionaryRule
	for index := range seed.Rules {
		rule := &seed.Rules[index]
		if rule.Alias == "Pytouch" {
			pytouch = rule
		}
	}
	if pytouch == nil {
		t.Fatal("v0.2 does not register the reviewed Pytouch alias")
	}
	if pytouch.TechnologyKey != "pytorch" || pytouch.CanonicalName != "PyTorch" ||
		pytouch.RuleKey != "pytorch.03" || pytouch.State != RuleCandidate ||
		pytouch.CaseMode != CaseSensitive || pytouch.BoundaryMode != BoundaryASCIIIdentifier {
		t.Fatalf("Pytouch must be a reviewed exact PyTorch alias, got %#v", pytouch)
	}
	for _, candidate := range seed.Candidates {
		if candidate.Text == "Pytouch" {
			t.Fatal("v0.2 retains Pytouch as an unreviewed typo candidate")
		}
	}
	approved := make(map[string]struct{})
	for _, ruleKey := range V02ApprovedRuleKeys() {
		approved[ruleKey] = struct{}{}
	}
	if _, exists := approved["pytorch.03"]; !exists {
		t.Fatal("reviewed Pytouch rule is not approval-eligible")
	}
}

func TestCandidateV02ApprovalEligibilityMatchesCompletedReview(t *testing.T) {
	seed := CandidateV02Seed()
	rulesByKey := make(map[string]DictionaryRule, len(seed.Rules))
	for _, rule := range seed.Rules {
		rulesByKey[rule.RuleKey] = rule
	}
	approved := make(map[string]struct{})
	for _, ruleKey := range V02ApprovedRuleKeys() {
		approved[ruleKey] = struct{}{}
	}

	for technologyKey, aliases := range map[string][]string{
		"python":       {"Python", "python"},
		"grpc":         {"gRPC", "grpc"},
		"sglang":       {"SGLang", "sglang", "SGlang"},
		"vllm":         {"vLLM", "vllm"},
		"tensorrt-llm": {"TensorRT-LLM", "TRT-LLM"},
		"pytorch":      {"PyTorch", "Pytorch", "Pytouch"},
		"tensorflow":   {"TensorFlow", "Tensorflow"},
		"aiops":        {"AIOps", "AIOPS"},
		"aibrix":       {"aibrix", "AIBrix"},
		"kubebuilder":  {"kubebuilder", "Kubebuilder"},
		"etcd":         {"etcd", "ETCD"},
		"juicefs":      {"JuiceFS", "Juicefs"},
		"blendshape":   {"BlendShape", "Blendshape"},
	} {
		for _, alias := range aliases {
			found := false
			for ruleKey, rule := range rulesByKey {
				if rule.TechnologyKey == technologyKey && rule.Alias == alias {
					if _, eligible := approved[ruleKey]; !eligible {
						t.Fatalf("reviewed alias %q (%s) is not approval-eligible", alias, ruleKey)
					}
					found = true
				}
			}
			if !found {
				t.Fatalf("v0.2 does not explicitly register reviewed alias %q for %s", alias, technologyKey)
			}
		}
	}

	for _, name := range []string{
		"C++", "C#", "Java", "JavaScript", "TypeScript", "Rust", "Lua",
		"Vue", "React", "Vulkan", "Metal", "DX11", "DX12", "HLSL", "Shader",
		"MySQL", "PostgreSQL", "Redis", "MongoDB", "ClickHouse", "StarRocks", "TiDB", "TiKV", "Milvus", "Alluxio",
		"Kafka", "Flink", "Spark", "Hive", "Hadoop", "PySpark",
		"Docker", "containerd", "Helm", "Kustomize", "Volcano", "Kueue", "Koordinator", "YuniKorn", "Karmada", "Karpenter", "KServe", "Knative", "Calico", "Cilium",
		"Envoy", "Higress", "Kong", "APISIX", "Traefik", "Istio", "Linkerd", "Nginx", "OpenResty", "HAProxy",
		"Prometheus", "Grafana", "OpenTelemetry", "Jaeger", "Loki", "Thanos", "VictoriaMetrics", "Zabbix", "Nightingale",
		"TensorRT", "ONNX", "LMDeploy", "TGI", "DeepSpeed", "Megatron-LM", "TRL", "OpenRLHF", "LLaMA-Factory", "ComfyUI",
		"EvalScope", "Inspect AI", "Langfuse", "Promptfoo", "OpenCompass", "lm-eval-harness", "SWE-bench", "Terminal-Bench", "GAIA",
		"CUDA", "cuDNN", "NCCL", "ROCm", "NVLink", "RDMA", "InfiniBand", "RoCEv2",
		"Git", "Perforce", "SVN", "Jenkins", "Teamcity", "GitLab CI", "Airflow", "ArgoCD", "Terraform", "Ansible",
		"CodeQL", "Semgrep", "IDA", "x64dbg",
		"RAG", "Embedding", "SFT", "RLHF", "DPO", "PPO", "GRPO", "DAPO", "LoRA", "MoE", "PagedAttention", "LLM-as-Judge",
		"HTTP", "HTTPS", "TCP", "UDP", "WebSocket", "Protobuf", "OpenAPI", "OAuth2", "OIDC",
	} {
		ruleKey := stableTechnologyKey(name) + ".01"
		if _, exists := approved[ruleKey]; !exists {
			t.Fatalf("reviewed B-table rule %q is not approval-eligible", ruleKey)
		}
	}

	for _, ruleKey := range []string{"go.01", "unreal-engine.01", "unity.01", "shell.01", "verl.01"} {
		if _, eligible := approved[ruleKey]; eligible {
			t.Fatalf("context-dependent rule %q became approval-eligible", ruleKey)
		}
	}
}

func TestV02RuleApprovalEligibilityRejectsAnythingOutsideReview(t *testing.T) {
	for _, ruleKey := range []string{"kubernetes.01", "kubernetes.03", "pytorch.03", "docker.01"} {
		if !V02RuleApprovalEligible(ruleKey) {
			t.Fatalf("reviewed rule %q is not approval-eligible", ruleKey)
		}
	}
	for _, ruleKey := range []string{"go.01", "unreal-engine.01", "ue5.01", "not-a-rule"} {
		if V02RuleApprovalEligible(ruleKey) {
			t.Fatalf("unreviewed rule %q is approval-eligible", ruleKey)
		}
	}
}

func TestCandidateV02SeedUsesCaseInsensitiveExactMatchingOnlyForBTableNames(t *testing.T) {
	seed := CandidateV02Seed()
	var docker, pytorch DictionaryRule
	for _, rule := range seed.Rules {
		switch rule.RuleKey {
		case "docker.01":
			docker = rule
		case "pytorch.01":
			pytorch = rule
		}
	}
	if docker.CaseMode != CaseInsensitive || docker.BoundaryMode != BoundaryASCIIIdentifier {
		t.Fatalf("reviewed B-table Docker rule must be case-insensitive with an exact identifier boundary: %#v", docker)
	}
	if pytorch.CaseMode != CaseSensitive {
		t.Fatalf("A-table PyTorch aliases must remain the explicitly reviewed spellings: %#v", pytorch)
	}
	docker.State = RuleEnabled
	docker.Approval = &Approval{Reviewer: "reviewer", Decided: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), Rationale: "synthetic approval"}
	matcher := mustMatcher(t, docker)
	matches, err := matcher.MatchField(SourceFieldDescription, "Docker docker DOCKER Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 {
		t.Fatalf("case-insensitive exact Docker matches = %#v", matches)
	}
}
