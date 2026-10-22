// Package jdanalysis contains the remote-only, deterministic JD technology
// analysis domain. It deliberately has no source-collection client and never
// writes body text to logs or local files.
package jdanalysis

import (
	"fmt"
	"strings"
	"time"
)

type RuleState string

const (
	RuleEnabled     RuleState = "enabled"
	RuleConditional RuleState = "conditional"
	RuleCandidate   RuleState = "candidate"
)

func (state RuleState) Valid() bool {
	return state == RuleEnabled || state == RuleConditional || state == RuleCandidate
}

type CaseMode string

const (
	CaseSensitive   CaseMode = "sensitive"
	CaseInsensitive CaseMode = "insensitive"
)

type BoundaryMode string

const (
	BoundaryExact           BoundaryMode = "exact"
	BoundaryASCIIIdentifier BoundaryMode = "ascii_identifier"
	BoundaryCustom          BoundaryMode = "custom"
)

// Approval is intentionally separate from the rule itself so an enabled rule
// always has a named, dated human decision that can be stored and audited.
type Approval struct {
	Reviewer  string
	Decided   time.Time
	Rationale string
}

type DictionaryRule struct {
	TechnologyKey   string
	CanonicalName   string
	TechnologyKind  string
	RuleKey         string
	Alias           string
	State           RuleState
	CaseMode        CaseMode
	BoundaryMode    BoundaryMode
	OverlapPriority int
	Summary         string
	Approval        *Approval
}

func (rule DictionaryRule) Validate() error {
	if rule.TechnologyKey == "" || rule.CanonicalName == "" || rule.TechnologyKind == "" || rule.RuleKey == "" || rule.Alias == "" || rule.Summary == "" {
		return fmt.Errorf("dictionary rule is missing required identity or matching fields")
	}
	if !rule.State.Valid() {
		return fmt.Errorf("dictionary rule has invalid state")
	}
	if rule.CaseMode != CaseSensitive && rule.CaseMode != CaseInsensitive {
		return fmt.Errorf("dictionary rule has invalid case mode")
	}
	if rule.BoundaryMode != BoundaryExact && rule.BoundaryMode != BoundaryASCIIIdentifier && rule.BoundaryMode != BoundaryCustom {
		return fmt.Errorf("dictionary rule has invalid boundary mode")
	}
	if rule.State == RuleEnabled {
		if rule.Approval == nil || strings.TrimSpace(rule.Approval.Reviewer) == "" || rule.Approval.Decided.IsZero() || strings.TrimSpace(rule.Approval.Rationale) == "" {
			return fmt.Errorf("enabled dictionary rule requires explicit approval")
		}
	}
	return nil
}

func (rule DictionaryRule) Executable() bool {
	return rule.State == RuleEnabled && rule.Validate() == nil
}

type CandidateTerm struct {
	Text                  string
	ProposedTechnologyKey string
	State                 RuleState
	Summary               string
}

type DictionarySeed struct {
	VersionLabel   string
	SourceDocument string
	Rules          []DictionaryRule
	Candidates     []CandidateTerm
}

func (seed DictionarySeed) Validate() error {
	if seed.VersionLabel == "" || seed.SourceDocument == "" {
		return fmt.Errorf("dictionary seed is missing version provenance")
	}
	ruleKeys := make(map[string]struct{}, len(seed.Rules))
	for _, rule := range seed.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
		if _, exists := ruleKeys[rule.RuleKey]; exists {
			return fmt.Errorf("dictionary seed repeats a rule key")
		}
		ruleKeys[rule.RuleKey] = struct{}{}
	}
	candidateTexts := make(map[string]struct{}, len(seed.Candidates))
	for _, candidate := range seed.Candidates {
		if candidate.Text == "" || candidate.Summary == "" || !candidate.State.Valid() || candidate.State == RuleEnabled {
			return fmt.Errorf("dictionary seed has an invalid candidate")
		}
		if _, exists := candidateTexts[candidate.Text]; exists {
			return fmt.Errorf("dictionary seed repeats a candidate term")
		}
		candidateTexts[candidate.Text] = struct{}{}
	}
	return nil
}

func (seed DictionarySeed) ExecutableRules() []DictionaryRule {
	rules := make([]DictionaryRule, 0)
	for _, rule := range seed.Rules {
		if rule.Executable() {
			rules = append(rules, rule)
		}
	}
	return rules
}

// CandidateV01Seed is a structured transcription of
// Docs/开发指导/JD建模/technical-keyword-alias-map-v0.1.md. It is deliberately
// not a production dictionary: all potential rules are candidate or
// conditional until a reviewer explicitly approves each deterministic rule.
func CandidateV01Seed() DictionarySeed {
	seed := DictionarySeed{
		VersionLabel:   "technology-keyword-alias-map-v0.1",
		SourceDocument: "Docs/开发指导/JD建模/technical-keyword-alias-map-v0.1.md",
	}
	addAliases := func(key, canonical, kind string, state RuleState, boundary BoundaryMode, summary string, aliases ...string) {
		for index, alias := range aliases {
			seed.Rules = append(seed.Rules, DictionaryRule{
				TechnologyKey: key, CanonicalName: canonical, TechnologyKind: kind,
				RuleKey: fmt.Sprintf("%s.%02d", key, index+1), Alias: alias,
				State: state, CaseMode: CaseSensitive, BoundaryMode: boundary,
				OverlapPriority: len([]rune(alias)), Summary: summary,
			})
		}
	}
	addExactCandidates := func(kind string, names ...string) {
		for _, name := range names {
			addAliases(stableTechnologyKey(name), name, kind, RuleCandidate, BoundaryASCIIIdentifier, "v0.1 exact-name candidate requires human approval", name)
		}
	}

	// A: explicit alias proposals. Context-dependent terms remain conditional;
	// the others are still candidates because the document is not an approval.
	addAliases("go", "Go", "programming-language", RuleConditional, BoundaryASCIIIdentifier, "requires programming-language context; exclude go-zero and go-micro", "Go", "Golang")
	addAliases("kubernetes", "Kubernetes", "container-orchestration", RuleCandidate, BoundaryASCIIIdentifier, "do not infer from Kubernetes components", "Kubernetes", "K8s", "K8S")
	addAliases("python", "Python", "programming-language", RuleCandidate, BoundaryASCIIIdentifier, "do not infer from PyTorch or PySpark", "Python", "python")
	addAliases("grpc", "gRPC", "rpc-framework", RuleCandidate, BoundaryASCIIIdentifier, "kept distinct from RPC", "gRPC", "grpc")
	addAliases("sglang", "SGLang", "model-serving", RuleCandidate, BoundaryASCIIIdentifier, "only registered spelling variants", "SGLang", "sglang", "SGlang")
	addAliases("vllm", "vLLM", "model-serving", RuleCandidate, BoundaryASCIIIdentifier, "do not infer related tools", "vLLM", "vllm")
	addAliases("tensorrt-llm", "TensorRT-LLM", "model-serving", RuleCandidate, BoundaryCustom, "kept distinct from TensorRT", "TensorRT-LLM", "TRT-LLM")
	addAliases("pytorch", "PyTorch", "machine-learning-framework", RuleCandidate, BoundaryASCIIIdentifier, "Pytouch is intentionally excluded", "PyTorch", "Pytorch")
	addAliases("tensorflow", "TensorFlow", "machine-learning-framework", RuleCandidate, BoundaryASCIIIdentifier, "TF remains a separate candidate", "TensorFlow", "Tensorflow")
	addAliases("unreal-engine", "Unreal Engine", "game-engine", RuleConditional, BoundaryASCIIIdentifier, "UE variants require their own review; do not collapse versions", "Unreal", "unreal")
	addAliases("unity", "Unity", "game-engine", RuleConditional, BoundaryASCIIIdentifier, "requires game-engine context", "Unity", "unity")
	addAliases("shell", "Shell", "scripting-environment", RuleConditional, BoundaryASCIIIdentifier, "does not imply Bash", "Shell", "SHELL")
	addAliases("aiops", "AIOps", "operations-concept", RuleCandidate, BoundaryASCIIIdentifier, "kept distinct from AI", "AIOps", "AIOPS")
	addAliases("aibrix", "AIBrix", "platform", RuleCandidate, BoundaryASCIIIdentifier, "does not imply internal dependencies", "aibrix", "AIBrix")
	addAliases("kubebuilder", "Kubebuilder", "developer-tool", RuleCandidate, BoundaryASCIIIdentifier, "kept distinct from controller-runtime", "kubebuilder", "Kubebuilder")
	addAliases("verl", "verl", "training-tool", RuleConditional, BoundaryASCIIIdentifier, "requires explicit post-training project context", "verl", "VeRL", "VERL")
	addAliases("etcd", "etcd", "distributed-datastore", RuleCandidate, BoundaryASCIIIdentifier, "independent technology", "etcd", "ETCD")
	addAliases("juicefs", "JuiceFS", "storage", RuleCandidate, BoundaryASCIIIdentifier, "independent technology", "JuiceFS", "Juicefs")
	addAliases("blendshape", "BlendShape", "graphics-technique", RuleCandidate, BoundaryASCIIIdentifier, "kept distinct from specific tools or models", "BlendShape", "Blendshape")

	// B: exact-name candidates, grouped exactly as the v0.1 mapping. A display
	// name is not considered an approved alias merely because it appears here.
	addExactCandidates("programming-language", "C++", "C#", "Java", "JavaScript", "TypeScript", "Rust", "Lua")
	addExactCandidates("frontend-framework", "Vue", "React")
	addExactCandidates("graphics-api-or-language", "Vulkan", "Metal", "DX11", "DX12", "HLSL", "Shader")
	addExactCandidates("database-or-storage", "MySQL", "PostgreSQL", "Redis", "MongoDB", "ClickHouse", "StarRocks", "TiDB", "TiKV", "Milvus", "Alluxio")
	addExactCandidates("data-processing-or-queue", "Kafka", "Flink", "Spark", "Hive", "Hadoop", "PySpark")
	addExactCandidates("cloud-native-component", "Docker", "containerd", "Helm", "Kustomize", "Volcano", "Kueue", "Koordinator", "YuniKorn", "Karmada", "Karpenter", "KServe", "Knative", "Calico", "Cilium")
	addExactCandidates("gateway-or-service-mesh", "Envoy", "Higress", "Kong", "APISIX", "Traefik", "Istio", "Linkerd", "Nginx", "OpenResty", "HAProxy")
	addExactCandidates("observability", "Prometheus", "Grafana", "OpenTelemetry", "Jaeger", "Loki", "Thanos", "VictoriaMetrics", "Zabbix", "Nightingale")
	addExactCandidates("model-or-training-tool", "TensorRT", "ONNX", "LMDeploy", "TGI", "DeepSpeed", "Megatron-LM", "TRL", "OpenRLHF", "LLaMA-Factory", "ComfyUI")
	addExactCandidates("evaluation-tool-or-benchmark", "EvalScope", "Inspect AI", "Langfuse", "Promptfoo", "OpenCompass", "lm-eval-harness", "SWE-bench", "Terminal-Bench", "GAIA")
	addExactCandidates("gpu-or-parallel-computing", "CUDA", "cuDNN", "NCCL", "ROCm", "NVLink", "RDMA", "InfiniBand", "RoCEv2")
	addExactCandidates("engineering-or-delivery-tool", "Git", "Perforce", "SVN", "Jenkins", "Teamcity", "GitLab CI", "Airflow", "ArgoCD", "Terraform", "Ansible")
	addExactCandidates("security-tool", "CodeQL", "Semgrep", "IDA", "x64dbg")
	addExactCandidates("ai-method-or-mechanism", "RAG", "Embedding", "SFT", "RLHF", "DPO", "PPO", "GRPO", "DAPO", "LoRA", "MoE", "PagedAttention", "LLM-as-Judge")
	addExactCandidates("protocol-or-interface-specification", "HTTP", "HTTPS", "TCP", "UDP", "WebSocket", "Protobuf", "OpenAPI", "OAuth2", "OIDC")

	// C: terms that need context, version handling, or a future canonical
	// identity. Candidate terms intentionally cannot produce a mention.
	seed.Candidates = []CandidateTerm{
		{Text: "UE", ProposedTechnologyKey: "unreal-engine", State: RuleConditional, Summary: "short game-engine abbreviation requires context"},
		{Text: "UE4", State: RuleCandidate, Summary: "preserve engine version separately"},
		{Text: "UE5", State: RuleCandidate, Summary: "preserve engine version separately"},
		{Text: "UE5.4", State: RuleCandidate, Summary: "preserve engine version separately"},
		{Text: "C++17", ProposedTechnologyKey: "cpp", State: RuleConditional, Summary: "language and standard version need separate modeling"},
		{Text: "JS", ProposedTechnologyKey: "javascript", State: RuleConditional, Summary: "requires language context"},
		{Text: "TF", State: RuleCandidate, Summary: "ambiguous abbreviation"},
		{Text: "HF", State: RuleCandidate, Summary: "ambiguous ecosystem abbreviation"},
		{Text: "Transformers", State: RuleConditional, Summary: "library and architecture are ambiguous"},
		{Text: "transformers", State: RuleConditional, Summary: "library and architecture are ambiguous"},
		{Text: "Triton", State: RuleCandidate, Summary: "server and programming language are distinct"},
		{Text: "Argo", State: RuleCandidate, Summary: "does not imply ArgoCD or Argo Workflows"},
		{Text: "Megatron", State: RuleCandidate, Summary: "does not imply Megatron-LM"},
		{Text: "Inspect", State: RuleConditional, Summary: "ordinary word and evaluation tool are ambiguous"},
		{Text: "KVCache", State: RuleCandidate, Summary: "requires distinct cache-concept review"},
		{Text: "kvcache", State: RuleCandidate, Summary: "requires distinct cache-concept review"},
		{Text: "KV Cache", State: RuleCandidate, Summary: "requires distinct cache-concept review"},
		{Text: "RoCE", State: RuleCandidate, Summary: "must not imply RoCEv2"},
		{Text: "ROCE", State: RuleCandidate, Summary: "must not imply RoCEv2"},
		{Text: "ReAct", State: RuleCandidate, Summary: "kept distinct from React"},
		{Text: "MCP", State: RuleCandidate, Summary: "ambiguous technology or product terminology"},
		{Text: "Skills", State: RuleCandidate, Summary: "ambiguous general term"},
		{Text: "Harness", State: RuleCandidate, Summary: "ambiguous general term"},
		{Text: "Pytouch", State: RuleCandidate, Summary: "suspected typo; no automatic correction"},
		{Text: "SIGGRAPGH", State: RuleCandidate, Summary: "suspected typo; no automatic correction"},
		{Text: "Cublas", State: RuleCandidate, Summary: "requires canonical-name review"},
		{Text: "Cutlass", State: RuleCandidate, Summary: "requires canonical-name review"},
	}
	return seed
}

func stableTechnologyKey(name string) string {
	special := map[string]string{
		"C++": "cpp", "C#": "csharp", "gRPC": "grpc", "vLLM": "vllm",
		"SGLang": "sglang", "AIOps": "aiops", "AIBrix": "aibrix",
	}
	if key, ok := special[name]; ok {
		return key
	}
	var builder strings.Builder
	for _, runeValue := range strings.ToLower(name) {
		if (runeValue >= 'a' && runeValue <= 'z') || (runeValue >= '0' && runeValue <= '9') {
			builder.WriteRune(runeValue)
			continue
		}
		if builder.Len() > 0 && !strings.HasSuffix(builder.String(), "-") {
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}
