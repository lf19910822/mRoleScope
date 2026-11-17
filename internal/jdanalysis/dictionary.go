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

const (
	MatcherPolicyV01 = "technology-mentions-v1"
	MatcherPolicyV04 = "technology-mentions-v4"
)

// Approval is intentionally separate from the rule itself so an enabled rule
// always has a named, dated human decision that can be stored and audited.
type Approval struct {
	Reviewer  string
	Decided   time.Time
	Rationale string
}

type DictionaryEmission struct {
	TechnologyKey  string `json:"technology_key"`
	CanonicalName  string `json:"canonical_name"`
	TechnologyKind string `json:"technology_kind"`
	RelationType   string `json:"relation_type"`
}

func (emission DictionaryEmission) Validate() error {
	if emission.TechnologyKey == "" || emission.CanonicalName == "" || emission.TechnologyKind == "" {
		return fmt.Errorf("dictionary emission is missing identity fields")
	}
	if emission.RelationType != "self" && emission.RelationType != "inherited" {
		return fmt.Errorf("dictionary emission has invalid relation type")
	}
	return nil
}

type DictionaryExclusionPattern struct {
	PatternKey       string
	SourceRuleKey    string
	Pattern          string
	State            RuleState
	CaseMode         CaseMode
	BoundaryMode     BoundaryMode
	WindowCodePoints int
	Provenance       string
	Approval         *Approval
}

func (pattern DictionaryExclusionPattern) Validate() error {
	if pattern.PatternKey == "" || pattern.SourceRuleKey == "" || pattern.Pattern == "" || pattern.Provenance == "" {
		return fmt.Errorf("dictionary exclusion pattern is missing required fields")
	}
	if !pattern.State.Valid() {
		return fmt.Errorf("dictionary exclusion pattern has invalid state")
	}
	if pattern.CaseMode != CaseSensitive && pattern.CaseMode != CaseInsensitive {
		return fmt.Errorf("dictionary exclusion pattern has invalid case mode")
	}
	if pattern.BoundaryMode != BoundaryExact && pattern.BoundaryMode != BoundaryASCIIIdentifier && pattern.BoundaryMode != BoundaryCustom {
		return fmt.Errorf("dictionary exclusion pattern has invalid boundary mode")
	}
	if pattern.WindowCodePoints < 1 || pattern.WindowCodePoints > 96 {
		return fmt.Errorf("dictionary exclusion pattern window is outside the allowed range")
	}
	if pattern.State == RuleEnabled {
		if pattern.Approval == nil || strings.TrimSpace(pattern.Approval.Reviewer) == "" || pattern.Approval.Decided.IsZero() || strings.TrimSpace(pattern.Approval.Rationale) == "" {
			return fmt.Errorf("enabled dictionary exclusion pattern requires explicit approval")
		}
	}
	return nil
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
	Emissions       []DictionaryEmission
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
	for _, emission := range rule.Emissions {
		if err := emission.Validate(); err != nil {
			return fmt.Errorf("rule %s: %w", rule.RuleKey, err)
		}
	}
	return nil
}

func (rule DictionaryRule) Executable() bool {
	return rule.State == RuleEnabled && rule.Validate() == nil
}

func (rule DictionaryRule) EffectiveEmissions() []DictionaryEmission {
	if len(rule.Emissions) != 0 {
		return append([]DictionaryEmission(nil), rule.Emissions...)
	}
	return []DictionaryEmission{{
		TechnologyKey: rule.TechnologyKey, CanonicalName: rule.CanonicalName,
		TechnologyKind: rule.TechnologyKind, RelationType: "self",
	}}
}

type CandidateTerm struct {
	Text                  string
	ProposedTechnologyKey string
	State                 RuleState
	Summary               string
}

type DictionarySeed struct {
	VersionLabel         string
	SourceDocument       string
	MatcherPolicyVersion string
	Rules                []DictionaryRule
	Candidates           []CandidateTerm
	ExclusionPatterns    []DictionaryExclusionPattern
}

func (seed DictionarySeed) Validate() error {
	if seed.VersionLabel == "" || seed.SourceDocument == "" {
		return fmt.Errorf("dictionary seed is missing version provenance")
	}
	ruleKeys := make(map[string]struct{}, len(seed.Rules))
	technologyKeys := make(map[string]struct{}, len(seed.Rules))
	for _, rule := range seed.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
		if _, exists := ruleKeys[rule.RuleKey]; exists {
			return fmt.Errorf("dictionary seed repeats a rule key")
		}
		ruleKeys[rule.RuleKey] = struct{}{}
		technologyKeys[rule.TechnologyKey] = struct{}{}
	}
	for _, rule := range seed.Rules {
		for _, emission := range rule.Emissions {
			if _, exists := technologyKeys[emission.TechnologyKey]; !exists {
				return fmt.Errorf("dictionary rule %s emits an unknown technology identity", rule.RuleKey)
			}
		}
	}
	patternKeys := make(map[string]struct{}, len(seed.ExclusionPatterns))
	for _, pattern := range seed.ExclusionPatterns {
		if err := pattern.Validate(); err != nil {
			return err
		}
		if _, exists := ruleKeys[pattern.SourceRuleKey]; !exists {
			return fmt.Errorf("dictionary exclusion pattern references an unknown source rule")
		}
		if _, exists := patternKeys[pattern.PatternKey]; exists {
			return fmt.Errorf("dictionary seed repeats an exclusion pattern key")
		}
		patternKeys[pattern.PatternKey] = struct{}{}
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

func (seed DictionarySeed) ExecutableExclusionPatterns() []DictionaryExclusionPattern {
	patterns := make([]DictionaryExclusionPattern, 0)
	for _, pattern := range seed.ExclusionPatterns {
		if pattern.State == RuleEnabled && pattern.Validate() == nil {
			patterns = append(patterns, pattern)
		}
	}
	return patterns
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

// CandidateV02Seed starts a new immutable dictionary version from the v0.1
// candidate policy. Review-driven changes are added in later, separately
// tested slices so the deployed v0.1 seed remains byte-for-byte unchanged.
func CandidateV02Seed() DictionarySeed {
	seed := CandidateV01Seed()
	seed.VersionLabel = "technology-keyword-alias-map-v0.2"
	seed.SourceDocument = "Docs/开发指导/JD建模/technical-keyword-alias-map-v0.2.md"
	for index := range seed.Rules {
		if seed.Rules[index].Summary == "v0.1 exact-name candidate requires human approval" {
			seed.Rules[index].CaseMode = CaseInsensitive
			seed.Rules[index].Summary = "reviewed v0.2 exact name; ASCII case-insensitive with explicit identifier boundary"
		}
	}
	seed.Rules = append(seed.Rules, DictionaryRule{
		TechnologyKey: "pytorch", CanonicalName: "PyTorch", TechnologyKind: "machine-learning-framework",
		RuleKey: "pytorch.03", Alias: "Pytouch", State: RuleCandidate,
		CaseMode: CaseSensitive, BoundaryMode: BoundaryASCIIIdentifier,
		OverlapPriority: len([]rune("Pytouch")),
		Summary:         "reviewed explicit alias; does not enable fuzzy spelling correction",
	})
	candidates := make([]CandidateTerm, 0, len(seed.Candidates)-1)
	for _, candidate := range seed.Candidates {
		if candidate.Text != "Pytouch" {
			candidates = append(candidates, candidate)
		}
	}
	seed.Candidates = candidates
	return seed
}

// CandidateV03Seed retains the completed v0.2 policy as immutable history and
// adds the separately reviewed Go programming-language aliases.  A new seed
// identity forces full historical reanalysis rather than rewriting v0.2.
func CandidateV03Seed() DictionarySeed {
	seed := CandidateV02Seed()
	seed.VersionLabel = "technology-keyword-alias-map-v0.3"
	seed.SourceDocument = "Docs/开发指导/JD建模/technical-keyword-alias-map-v0.3.md"
	for index := range seed.Rules {
		rule := &seed.Rules[index]
		if rule.RuleKey != "go.01" && rule.RuleKey != "go.02" {
			continue
		}
		rule.State = RuleCandidate
		rule.CaseMode = CaseInsensitive
		rule.BoundaryMode = BoundaryASCIIIdentifier
		rule.Summary = "reviewed Go programming-language alias; ASCII case-insensitive with explicit identifier boundary"
	}
	return seed
}

// CandidateV04Seed is the immutable successor policy for the reviewed v0.3
// dictionary. It keeps the v0.3 rules as historical input, then adds only the
// explicitly reviewed aliases, identity emissions, and observed exclusions.
func CandidateV04Seed() DictionarySeed {
	seed := CandidateV03Seed()
	seed.VersionLabel = "technology-keyword-alias-map-v0.4"
	seed.SourceDocument = "Docs/开发指导/JD建模/technical-keyword-alias-map-v0.4.md"
	seed.MatcherPolicyVersion = MatcherPolicyV04

	for index := range seed.Rules {
		rule := &seed.Rules[index]
		switch rule.RuleKey {
		case "unreal-engine.01", "unreal-engine.02", "unity.01", "unity.02":
			rule.State = RuleCandidate
			rule.CaseMode = CaseSensitive
			rule.Summary = "reviewed v0.4 direct technology alias"
		case "react.01":
			rule.State = RuleCandidate
			rule.CaseMode = CaseSensitive
			rule.Summary = "reviewed exact-case frontend React alias"
		}
	}

	appendRule := func(key, canonical, kind, ruleKey, alias string, boundary BoundaryMode, emissions ...DictionaryEmission) {
		seed.Rules = append(seed.Rules, DictionaryRule{
			TechnologyKey: key, CanonicalName: canonical, TechnologyKind: kind,
			RuleKey: ruleKey, Alias: alias, State: RuleCandidate,
			CaseMode: CaseSensitive, BoundaryMode: boundary,
			OverlapPriority: len([]rune(alias)), Summary: "reviewed v0.4 direct technology alias",
			Emissions: emissions,
		})
	}
	self := func(key, canonical, kind string) DictionaryEmission {
		return DictionaryEmission{TechnologyKey: key, CanonicalName: canonical, TechnologyKind: kind, RelationType: "self"}
	}
	inherited := func(key, canonical, kind string) DictionaryEmission {
		return DictionaryEmission{TechnologyKey: key, CanonicalName: canonical, TechnologyKind: kind, RelationType: "inherited"}
	}

	appendRule("unreal-engine", "Unreal Engine", "game-engine", "unreal-engine.03", "Unreal Engine", BoundaryASCIIIdentifier)
	appendRule("unreal-engine", "Unreal Engine", "game-engine", "unreal-engine.04", "UE", BoundaryASCIIIdentifier)
	appendRule("unreal-engine-4", "Unreal Engine 4", "game-engine", "unreal-engine-4.01", "UE4", BoundaryASCIIIdentifier,
		inherited("unreal-engine", "Unreal Engine", "game-engine"), self("unreal-engine-4", "Unreal Engine 4", "game-engine"))
	appendRule("unreal-engine-5", "Unreal Engine 5", "game-engine", "unreal-engine-5.01", "UE5", BoundaryASCIIIdentifier,
		inherited("unreal-engine", "Unreal Engine", "game-engine"), self("unreal-engine-5", "Unreal Engine 5", "game-engine"))
	appendRule("unreal-engine-5", "Unreal Engine 5", "game-engine", "unreal-engine-5.02", "UE5.4", BoundaryASCIIIdentifier,
		inherited("unreal-engine", "Unreal Engine", "game-engine"), self("unreal-engine-5", "Unreal Engine 5", "game-engine"))
	appendRule("unity", "Unity", "game-engine", "unity.03", "Unity 3D", BoundaryASCIIIdentifier)
	appendRule("unity", "Unity", "game-engine", "unity.04", "Unity3D", BoundaryASCIIIdentifier)
	appendRule("react", "React", "frontend-framework", "react.02", "react", BoundaryASCIIIdentifier)
	appendRule("react-agent", "ReAct", "agent-paradigm", "react-agent.01", "ReAct", BoundaryASCIIIdentifier)

	appendRule("uobject", "UObject", "game-engine-subsystem", "uobject.01", "UObject", BoundaryASCIIIdentifier)
	appendRule("world-partition", "World Partition", "game-engine-subsystem", "world-partition.01", "World Partition", BoundaryASCIIIdentifier)
	appendRule("world-partition", "World Partition", "game-engine-subsystem", "world-partition.02", "WorldPartition", BoundaryASCIIIdentifier)
	appendRule("statetree", "StateTree", "game-engine-subsystem", "statetree.01", "StateTree", BoundaryASCIIIdentifier)
	appendRule("massentity", "MassEntity", "game-engine-subsystem", "massentity.01", "MassEntity", BoundaryASCIIIdentifier)
	appendRule("gamethread", "GameThread", "game-engine-subsystem", "gamethread.01", "GameThread", BoundaryASCIIIdentifier)
	appendRule("buildgraph", "BuildGraph", "game-engine-subsystem", "buildgraph.01", "BuildGraph", BoundaryASCIIIdentifier)

	appendRule("wwise", "Wwise", "game-audio-tool", "wwise.01", "Wwise", BoundaryASCIIIdentifier)
	appendRule("fmod", "FMOD", "game-audio-tool", "fmod.01", "FMOD", BoundaryASCIIIdentifier)
	appendRule("fmod", "FMOD", "game-audio-tool", "fmod.02", "FMOD Studio", BoundaryASCIIIdentifier)
	appendRule("criware", "CRIWARE", "game-audio-tool", "criware.01", "CRIWARE", BoundaryASCIIIdentifier)
	appendRule("criware", "CRIWARE", "game-audio-tool", "criware.02", "Criware", BoundaryASCIIIdentifier)
	appendRule("maya", "Maya", "3d-art-tool", "maya.01", "Maya", BoundaryASCIIIdentifier)

	appendRule("dcgm", "DCGM", "gpu-or-parallel-computing", "dcgm.01", "DCGM", BoundaryASCIIIdentifier)
	appendRule("nsight-systems", "Nsight Systems", "gpu-or-parallel-computing", "nsight-systems.01", "Nsight Systems", BoundaryASCIIIdentifier)
	appendRule("nsight-compute", "Nsight Compute", "gpu-or-parallel-computing", "nsight-compute.01", "Nsight Compute", BoundaryASCIIIdentifier)
	appendRule("nvswitch", "NVSwitch", "gpu-or-parallel-computing", "nvswitch.01", "NVSwitch", BoundaryASCIIIdentifier)
	appendRule("cann", "CANN", "gpu-or-parallel-computing", "cann.01", "CANN", BoundaryASCIIIdentifier)
	appendRule("numa", "NUMA", "gpu-or-parallel-computing", "numa.01", "NUMA", BoundaryASCIIIdentifier)
	appendRule("pcie", "PCIe", "gpu-or-parallel-computing", "pcie.01", "PCIe", BoundaryASCIIIdentifier)
	appendRule("roce", "RoCE", "gpu-or-parallel-computing", "roce.01", "RoCE", BoundaryASCIIIdentifier)
	appendRule("roce", "RoCE", "gpu-or-parallel-computing", "roce.02", "ROCE", BoundaryASCIIIdentifier)

	appendRule("kubelet", "kubelet", "cloud-native-component", "kubelet.01", "kubelet", BoundaryASCIIIdentifier)
	appendRule("kube-scheduler", "kube-scheduler", "cloud-native-component", "kube-scheduler.01", "kube-scheduler", BoundaryASCIIIdentifier)
	appendRule("controller-manager", "controller-manager", "cloud-native-component", "controller-manager.01", "controller-manager", BoundaryASCIIIdentifier)
	appendRule("kubefed", "KubeFed", "cloud-native-component", "kubefed.01", "KubeFed", BoundaryASCIIIdentifier)
	appendRule("controller-runtime", "controller-runtime", "cloud-native-component", "controller-runtime.01", "controller-runtime", BoundaryASCIIIdentifier)
	appendRule("client-go", "client-go", "cloud-native-component", "client-go.01", "client-go", BoundaryASCIIIdentifier)
	appendRule("keda", "KEDA", "cloud-native-component", "keda.01", "KEDA", BoundaryASCIIIdentifier)

	appendRule("elasticsearch", "Elasticsearch", "infrastructure", "elasticsearch.01", "Elasticsearch", BoundaryASCIIIdentifier)
	appendRule("ceph", "Ceph", "infrastructure", "ceph.01", "Ceph", BoundaryASCIIIdentifier)
	appendRule("openstack", "OpenStack", "infrastructure", "openstack.01", "OpenStack", BoundaryASCIIIdentifier)
	appendRule("qemu", "QEMU", "infrastructure", "qemu.01", "QEMU", BoundaryASCIIIdentifier)
	appendRule("libvirt", "libvirt", "infrastructure", "libvirt.01", "libvirt", BoundaryASCIIIdentifier)
	appendRule("cloud-init", "cloud-init", "infrastructure", "cloud-init.01", "cloud-init", BoundaryASCIIIdentifier)

	seed.ExclusionPatterns = []DictionaryExclusionPattern{
		{PatternKey: "react.to.01", SourceRuleKey: "react.02", Pattern: "react to", State: RuleCandidate, CaseMode: CaseSensitive, BoundaryMode: BoundaryExact, WindowCodePoints: 96, Provenance: "Docs/开发指导/JD建模/技术关键词候选审计-2026-11-13.md"},
		{PatternKey: "unreal.user-experience.01", SourceRuleKey: "unreal-engine.04", Pattern: "user experience", State: RuleCandidate, CaseMode: CaseInsensitive, BoundaryMode: BoundaryExact, WindowCodePoints: 96, Provenance: "Docs/开发指导/JD建模/技术关键词候选审计-2026-11-13.md"},
		{PatternKey: "unreal.用户体验.01", SourceRuleKey: "unreal-engine.04", Pattern: "用户体验", State: RuleCandidate, CaseMode: CaseSensitive, BoundaryMode: BoundaryExact, WindowCodePoints: 96, Provenance: "Docs/开发指导/JD建模/技术关键词候选审计-2026-11-13.md"},
	}

	filteredCandidates := make([]CandidateTerm, 0, len(seed.Candidates))
	for _, candidate := range seed.Candidates {
		switch candidate.Text {
		case "UE", "UE4", "UE5", "UE5.4", "ReAct":
			continue
		}
		filteredCandidates = append(filteredCandidates, candidate)
	}
	seed.Candidates = filteredCandidates
	return seed
}

// V02ApprovedRuleKeys is the fixed policy allowlist derived from the local
// human review. It is deliberately separate from candidate import: every key
// still requires the durable, authenticated approval operation before use.
func V02ApprovedRuleKeys() []string {
	approved := []string{
		"kubernetes.01",
		"kubernetes.02",
		"kubernetes.03",
		"python.01",
		"python.02",
		"grpc.01",
		"grpc.02",
		"sglang.01",
		"sglang.02",
		"sglang.03",
		"vllm.01",
		"vllm.02",
		"tensorrt-llm.01",
		"tensorrt-llm.02",
		"pytorch.01",
		"pytorch.02",
		"pytorch.03",
		"tensorflow.01",
		"tensorflow.02",
		"aiops.01",
		"aiops.02",
		"aibrix.01",
		"aibrix.02",
		"kubebuilder.01",
		"kubebuilder.02",
		"etcd.01",
		"etcd.02",
		"juicefs.01",
		"juicefs.02",
		"blendshape.01",
		"blendshape.02",
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
		approved = append(approved, stableTechnologyKey(name)+".01")
	}
	return approved
}

// V02RuleApprovalEligible reports whether the fixed v0.2 review policy
// permits a candidate rule to reach the durable approval operation.
func V02RuleApprovalEligible(ruleKey string) bool {
	for _, approvedRuleKey := range V02ApprovedRuleKeys() {
		if ruleKey == approvedRuleKey {
			return true
		}
	}
	return false
}

// V03ApprovedRuleKeys preserves the completed v0.2 allowlist and adds only
// the separately reviewed Go aliases.  The list is intentionally closed so a
// caller cannot promote an imported conditional/candidate rule by key.
func V03ApprovedRuleKeys() []string {
	approved := append([]string(nil), V02ApprovedRuleKeys()...)
	return append(approved, "go.01", "go.02")
}

// V03RuleApprovalEligible reports whether the fixed v0.3 review permits one
// candidate rule to reach the durable approval operation.
func V03RuleApprovalEligible(ruleKey string) bool {
	for _, approvedRuleKey := range V03ApprovedRuleKeys() {
		if ruleKey == approvedRuleKey {
			return true
		}
	}
	return false
}

func V04ApprovedRuleKeys() []string {
	approved := append([]string(nil), V03ApprovedRuleKeys()...)
	approved = append(approved,
		"unreal-engine.01", "unreal-engine.02", "unreal-engine.03", "unreal-engine.04",
		"unreal-engine-4.01", "unreal-engine-5.01", "unreal-engine-5.02",
		"unity.01", "unity.02", "unity.03", "unity.04", "react.01", "react.02", "react-agent.01",
		"uobject.01", "world-partition.01", "world-partition.02", "statetree.01", "massentity.01", "gamethread.01", "buildgraph.01",
		"wwise.01", "fmod.01", "fmod.02", "criware.01", "criware.02", "maya.01",
		"dcgm.01", "nsight-systems.01", "nsight-compute.01", "nvswitch.01", "cann.01", "numa.01", "pcie.01", "roce.01", "roce.02",
		"kubelet.01", "kube-scheduler.01", "controller-manager.01", "kubefed.01", "controller-runtime.01", "client-go.01", "keda.01",
		"elasticsearch.01", "ceph.01", "openstack.01", "qemu.01", "libvirt.01", "cloud-init.01",
	)
	return approved
}

func V04RuleApprovalEligible(ruleKey string) bool {
	for _, approvedRuleKey := range V04ApprovedRuleKeys() {
		if ruleKey == approvedRuleKey {
			return true
		}
	}
	return false
}

func V04ApprovedExclusionPatternKeys() []string {
	return []string{"react.to.01", "unreal.user-experience.01", "unreal.用户体验.01"}
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
