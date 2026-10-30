# Spec Delta

## Purpose

This capability deterministically derives auditable job-family facts from an
in-scope normalized title and a reviewed, versioned dictionary without reading
JD bodies or invoking an external model.

## ADDED Requirements

### Requirement: The capability SHALL publish V3 with fifteen stable job-family keys

The system SHALL publish immutable dictionary V3 containing the existing
thirteen stable family keys plus `ai_algorithm` and `ai_application`. Every
enabled complete alias SHALL have exactly one owning family key. V1 and V2
assets and previously derived results MUST remain available for audit.

#### Scenario: V3 accepts two approved AI family keys

- **WHEN** the V3 dictionary is loaded
- **THEN** it SHALL include enabled `ai_algorithm` with display name `AI 算法`
- **AND** it SHALL include enabled `ai_application` with display name `AI 应用`
- **AND** no complete alias SHALL map to more than one family key.

### Requirement: The capability SHALL distinguish reviewed AI algorithm and AI application title phrases

The system SHALL map reviewed model training, evaluation, model research, and
recommendation-algorithm phrases to `ai_algorithm`; it SHALL map reviewed AI
application, Agent delivery, and game-AI delivery phrases to `ai_application`.
It MUST retain broad algorithms, backend, and platform-infrastructure aliases
in their existing families when no reviewed complete phrase owns them.

#### Scenario: Reviewed AI algorithm phrases classify model-capability work

- **WHEN** titles `LLM Evaluation算法研究员-基模`, `LLM Pretraining Data Technical Staff (Agentic Models)`, and `多模态算法工程师（audio方向）` are parsed
- **THEN** each SHALL have an `ai_algorithm` match
- **AND** each SHALL have primary family `ai_algorithm`.

#### Scenario: Reviewed AI application phrases classify delivery work without losing overlapping evidence

- **WHEN** titles `Agent开发工程师（数据方向）`, `AI全栈开发工程师（AI引擎）`, and `后端开发工程师（AI 游戏方向）- 星布谷地` are parsed
- **THEN** each SHALL have an `ai_application` match and primary family `ai_application`
- **AND** matching backend or platform-infrastructure evidence SHALL remain when its separately owned alias matches outside a project mask.

### Requirement: The capability SHALL preserve literal matching and primary-family selection contracts

The system SHALL continue to match only listed contiguous literal aliases in
the normalized title, applying existing ASCII case and identifier-boundary
rules. It SHALL continue to preserve cross-family overlap and choose one
primary family by match count, last effective end position, then family-key
lexical order. It MUST NOT infer AI family membership from a bare AI-related
term, JD body, metadata, or model output.

#### Scenario: Broad platform and domain-only titles retain their boundaries

- **WHEN** `大模型平台开发工程师`, `AI Agent引擎系统开发工程师 -UE5写实射击预研`, and `云游戏技术专家` are parsed
- **THEN** the first two SHALL have primary family `platform_infra`
- **AND** `云游戏技术专家` SHALL remain `unclassified`
- **AND** no title SHALL acquire `ai_algorithm` or `ai_application` solely from `AI`, `LLM`, `大模型`, `Agent`, `AIGC`, `RAG`, `MCP`, `Prompt`, or `工作流`.

### Requirement: The capability SHALL expose the reviewed V3 dictionary to maintainers

The repository SHALL maintain a readable current dictionary at
`Docs/开发指导/JD建模/岗位族词典_latest.md`. It SHALL state its V3 identity,
all fifteen family keys and enabled aliases, migrations from V2, and the
literal-matching and primary-selection boundaries that govern its use.

#### Scenario: A maintainer reviews the current dictionary

- **WHEN** a maintainer opens the latest dictionary document
- **THEN** they SHALL be able to identify each family key, display name, and enabled alias
- **AND** they SHALL be able to identify which approved V2 aliases moved to either new AI family
- **AND** the document SHALL not claim that an unlisted term is matched.
