# Spec Delta

## Purpose

为 JD 正文提供不可变、可复演、可审计的技术身份归一化能力，既保留明确技术词面的召回能力，也能表达版本身份、大小写冲突和经过证据支持的普通语言排除规则。

## ADDED Requirements

### Requirement: Dictionary versions are immutable and independently identifiable
每个对外可用的技术词典 SHALL 以独立版本身份发布，并保存来源文档、规则内容指纹、审核状态和发布时间。已发布或已退役版本的规则、身份、排除模式和展开关系 MUST NOT 被原地修改。

#### Scenario: New dictionary version preserves prior facts
- **WHEN** v0.4 replaces v0.3 as the active dictionary
- **THEN** v0.3 的规则、命中结果、证据和发布记录仍可被审计读取，且 v0.3 的内容不被改写成 v0.4

#### Scenario: Published dictionary rejects rule mutation
- **WHEN** an operator attempts to alter an alias, case policy, exclusion pattern, or identity relation in a published version
- **THEN** the operation is rejected and the published version remains unchanged

### Requirement: Explicit technical terms default to direct matching
已登记且通过审核的技术词面 SHALL 在批准的 JD 正文字段中按明确大小写、边界和别名规则直接匹配；缺少其他技术词、游戏词或前端词不得单独成为不命中的理由。系统 MUST NOT 通过模糊相似、前缀、子串或未登记缩写补充命中。

#### Scenario: Standalone Unity mention is retained
- **WHEN** a JD body contains `熟悉 Unity` and no nearby technology term
- **THEN** the result contains the `Unity` identity with evidence pointing to the exact `Unity` span

#### Scenario: Standalone React mention is retained
- **WHEN** a JD body contains `熟悉 react` and no nearby frontend term
- **THEN** the result contains the `React` identity with evidence pointing to the exact lowercase span

#### Scenario: Unregistered shorthand is not inferred
- **WHEN** a JD body contains a shorthand or related product name that is not registered in the active dictionary
- **THEN** the result does not infer a technology identity from that text

### Requirement: Unreal versions emit base and version identities
命中 `UE4`、`UE5` 或 `UE5.4` 时，系统 SHALL 同时产生基础 `Unreal Engine` 身份和对应的 `Unreal Engine 4` 或 `Unreal Engine 5` 身份。`UE5.4` SHALL 归入 `Unreal Engine 5`，不得创建独立的 `Unreal Engine 5.4` 展示身份。

#### Scenario: UE5 emits two identities
- **WHEN** a JD body contains `UE5`
- **THEN** the result contains `Unreal Engine` and `Unreal Engine 5`, and both identities retain traceable evidence for the `UE5` span

#### Scenario: UE5.4 keeps the major version boundary
- **WHEN** a JD body contains `UE5.4`
- **THEN** the result contains `Unreal Engine` and `Unreal Engine 5`, preserves `UE5.4` as the evidence alias, and does not expose `Unreal Engine 5.4` as a separate identity

#### Scenario: UE4 does not become UE5
- **WHEN** a JD body contains `UE4`
- **THEN** the result contains `Unreal Engine` and `Unreal Engine 4`, and does not contain `Unreal Engine 5`

### Requirement: Unity aliases share one identity
`Unity`、`unity`、`Unity 3D` 和 `Unity3D` SHALL map to one canonical `Unity` identity. The original matched spelling and coordinate MUST remain available as evidence, but aliases MUST NOT become separate filter options or trend identities.

#### Scenario: Unity 3D maps to Unity
- **WHEN** a JD body contains `Unity 3D` or `Unity3D`
- **THEN** the result contains one `Unity` identity with evidence for the complete registered alias

#### Scenario: Unity aliases are not split in serving
- **WHEN** a job contains both `Unity` and `Unity3D`
- **THEN** the job has one `Unity` technology identity with multiple evidence locations, not two technology identities

### Requirement: React and ReAct remain separate identities
`React` 和 `react` SHALL map to the frontend `React` identity, while exact-case `ReAct` SHALL map to an independent agent-paradigm identity. Matching MUST NOT apply a global case-insensitive comparison that turns `ReAct` into `React`.

#### Scenario: React case variants map to frontend React
- **WHEN** a JD body contains `React` or `react`
- **THEN** the result contains the frontend `React` identity

#### Scenario: ReAct maps to the agent identity
- **WHEN** a JD body contains exact-case `ReAct`
- **THEN** the result contains the `ReAct` agent-paradigm identity and does not contain the frontend `React` identity solely because of that span

#### Scenario: Other case variants are not silently merged
- **WHEN** a JD body contains an unregistered case variant such as `REACT`
- **THEN** the result does not silently choose either `React` or `ReAct` unless that spelling has an explicit approved rule

### Requirement: v0.4 includes the Unreal subsystem identities
v0.4 SHALL include separate formal identities for `UObject`, `World Partition`, `WorldPartition`, `StateTree`, `MassEntity`, `GameThread`, and `BuildGraph`. These identities MUST NOT be inferred as aliases of `Unreal Engine` or of one another.

#### Scenario: Unreal subsystem aliases are executable
- **WHEN** the v0.4 dictionary is loaded for analysis
- **THEN** each listed Unreal subsystem has an approved executable rule for its registered exact spelling, and a body containing that spelling produces the corresponding independent identity

#### Scenario: Unreal subsystem mentions remain independent
- **WHEN** a JD body contains `UObject`, `StateTree`, and `BuildGraph`
- **THEN** the result contains those three identities separately and does not replace them with only `Unreal Engine`

### Requirement: v0.4 includes the game tool identities
v0.4 SHALL include formal identities for `Wwise`, `FMOD`, `FMOD Studio`, `CRIWARE`, `Criware`, and `Maya`, with `FMOD`/`FMOD Studio` and `CRIWARE`/`Criware` represented as reviewed aliases rather than unrelated technologies.

#### Scenario: Game tool aliases are executable
- **WHEN** the v0.4 dictionary is loaded for analysis
- **THEN** the listed game tool names and aliases are registered in the executable v0.4 rule set and are not left as candidate-only records

#### Scenario: FMOD aliases share one identity
- **WHEN** a JD body contains both `FMOD` and `FMOD Studio`
- **THEN** the result contains one canonical FMOD identity with evidence for each matched alias and does not create a second technology for the longer spelling

### Requirement: v0.4 includes GPU and performance identities
v0.4 SHALL include formal identities for `DCGM`, `Nsight Systems`, `Nsight Compute`, `NVSwitch`, `CANN`, `NUMA`, `PCIe`, and `RoCE`, with `ROCE` as an explicitly reviewed alias of `RoCE`. These identities MUST remain distinct from `GPU`, `NVIDIA`, `CUDA`, `NVLink`, and `RoCEv2`.

#### Scenario: GPU and performance names are executable
- **WHEN** the v0.4 dictionary is loaded for analysis
- **THEN** every listed GPU/performance name has an approved executable rule, while the bare unregistered `Nsight` spelling is not silently expanded to either complete product name

#### Scenario: Related GPU identities are not inferred
- **WHEN** a JD body contains `DCGM`, `NVSwitch`, `CANN`, `PCIe`, and `RoCE`
- **THEN** the result contains those exact independent identities and does not add `NVIDIA`, `GPU`, `CUDA`, `NVLink`, or `RoCEv2` without separate evidence

### Requirement: v0.4 includes Kubernetes component identities
v0.4 SHALL include separate formal identities for `kubelet`, `kube-scheduler`, `controller-manager`, `KubeFed`, `controller-runtime`, `client-go`, and `KEDA`. These identities MUST NOT be collapsed into `Kubernetes`, `Kubebuilder`, or `Go`.

#### Scenario: Kubernetes components are independently filterable
- **WHEN** a JD body contains `kubelet`, `controller-runtime`, and `client-go`
- **THEN** the result contains three corresponding component identities, each with its own evidence, and does not replace them with only Kubernetes or Go

#### Scenario: KEDA is not inferred from Kubernetes
- **WHEN** a JD body contains only `Kubernetes`
- **THEN** the result does not contain `KEDA`, kubelet, or another component identity without its own registered evidence

### Requirement: v0.4 includes infrastructure identities
v0.4 SHALL include separate formal identities for `Elasticsearch`, `Ceph`, `OpenStack`, `QEMU`, `libvirt`, and `cloud-init`. These identities MUST NOT infer `Elastic`, `ELK`, `Kibana`, `KVM`, Kubernetes, or another related platform without independently registered evidence.

#### Scenario: Infrastructure names are executable
- **WHEN** the v0.4 dictionary is loaded for analysis
- **THEN** every listed infrastructure name has an approved executable rule and is available as an independent technology option

#### Scenario: Infrastructure relations are not aliases
- **WHEN** a JD body contains `QEMU` and `libvirt`
- **THEN** the result contains QEMU and libvirt separately, and does not automatically add KVM

### Requirement: Exclusion patterns are explicit and evidence-backed
排除规则 SHALL 仅用于已经在真实 JD 或审计样本中观察到、且可用固定字符串与边界条件复演的明确普通语言反例。排除规则 MUST NOT 要求正向陪跑词，也 MUST NOT 变成未版本化的通用英语语义判断器。

#### Scenario: Observed ordinary phrase suppresses one match
- **WHEN** an approved exclusion pattern exactly matches a phrase such as `react to user input` for the lowercase `react` rule
- **THEN** that occurrence is suppressed and the exclusion rule identity is retained in the auditable dictionary provenance

#### Scenario: Missing positive context does not suppress a match
- **WHEN** a JD body contains `熟悉 Unity` without any registered exclusion phrase
- **THEN** the `Unity` match is retained

#### Scenario: Unobserved broad negative inference is not applied
- **WHEN** an occurrence has no approved exclusion pattern, even if an implementation could imagine an alternative ordinary meaning
- **THEN** the occurrence follows the direct matching rule and is not suppressed by ad hoc semantic inference

### Requirement: Evidence remains field-scoped and replayable
每个正式技术身份 SHALL 关联到 JD 正文允许字段中的原始命中区间、词典规则和规范化正文指纹。身份展开不得制造没有正文区间的虚假证据；同一原文区间产生多个正式身份时，每个身份都必须能通过版本化规则关系复演。

#### Scenario: Expanded identities share a real span
- **WHEN** `UE5` produces base and version identities
- **THEN** both identities reference the stored `UE5` source-field interval and the same normalized field version, without storing fabricated text

#### Scenario: Unsupported source field is rejected
- **WHEN** a caller attempts to create a technology match from title, tags, or another field outside the approved JD body fields
- **THEN** the match is rejected and no evidence is published
