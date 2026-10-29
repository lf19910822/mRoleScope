# deterministic-job-family-tagging Specification

## ADDED Requirements

### Requirement: The capability SHALL use a reviewed one-to-one family dictionary

The system SHALL use an immutable, versioned reviewed dictionary containing
the approved stable family keys: `algorithm_research`, `backend`, `client`,
`data_engineering`, `engineering_management`, `engine_systems`, `frontend`,
`hardware_embedded`, `operations_sre`, `platform_infra`, `security`,
`testing_quality`, and `toolchain`. Every enabled complete alias SHALL map to
exactly one family key. The system MUST reject a dictionary in which one
complete alias maps to more than one family.

#### Scenario: Reviewed V2 aliases retain one-to-one ownership

- **GIVEN** dictionary V2 adds `AI`, `agent`, and `大模型` to
  `platform_infra`, `Agent开发` and `Agent研发` to `backend`, and `k8s` and
  `Kubernetes` to `operations_sre`
- **WHEN** the dictionary is loaded
- **THEN** every complete alias SHALL have exactly one owning family
- **AND** V1 records and the V1 asset SHALL remain available for history and
  audit.

### Requirement: The capability SHALL match only explicit literal aliases

The system SHALL match listed Chinese aliases as contiguous literal title
fragments. Listed ASCII aliases MAY specify case-insensitive matching and ASCII
identifier boundaries. It MUST NOT use regular expressions, fuzzy matching,
automatic synonym/word-form expansion, or Chinese segmentation.

#### Scenario: V2 classifies reviewed audit titles but preserves a domain-only no-match

- **GIVEN** the V2 aliases reviewed from the 2026-10-29 unclassified audit
- **WHEN** titles `AI存储资深研发工程师（大模型）`, `LLM Posttrain研究员`,
  `网络工程师`, and `游戏AI研发工程师（AIGC方向）-绝区零` are parsed
- **THEN** they SHALL respectively have primary families `platform_infra`,
  `algorithm_research`, `operations_sre`, and `platform_infra`
- **AND** `云游戏技术专家` SHALL remain a completed `unclassified` result.

#### Scenario: Bounded V2 ASCII aliases preserve reviewed overlap behavior

- **GIVEN** `agent` is a bounded case-insensitive `platform_infra` alias,
  `Agent开发` is a bounded case-insensitive `backend` alias, `k8s` is a
  bounded case-insensitive `operations_sre` alias, and `云原生` maps to
  `platform_infra`
- **WHEN** titles `Agent开发工程师`, `K8S云原生开发`, and `AIOps工程师` are parsed
- **THEN** the first SHALL retain a backend and platform-infrastructure match
  with `backend` primary
- **AND** the second SHALL retain an operations and platform-infrastructure
  match with `platform_infra` primary
- **AND** the third SHALL not acquire a platform-infrastructure match from the
  `AI` prefix inside `AIOps`.
