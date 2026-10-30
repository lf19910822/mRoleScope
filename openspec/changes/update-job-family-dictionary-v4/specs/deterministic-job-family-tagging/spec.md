# deterministic-job-family-tagging Specification

## ADDED Requirements

### Requirement: The capability SHALL use a reviewed one-to-one family dictionary

The system SHALL load all job-family keys and aliases from an immutable,
versioned reviewed dictionary. Every enabled complete alias SHALL map to
exactly one family key, and the system MUST reject ambiguous ownership.

#### Scenario: V4 removes only unqualified platform signals

- **GIVEN** V4 no longer lists bare bounded `AI` or bare bounded `agent` for
  `platform_infra`
- **WHEN** the dictionary is loaded
- **THEN** all fifteen stable family keys and every other V3 alias SHALL
  remain available
- **AND** every enabled complete alias SHALL still have exactly one owner.
- **AND** V1, V2, V3, and their previously derived results SHALL remain
  available for audit.

### Requirement: The capability SHALL match only explicit literal aliases

The system SHALL match listed Chinese aliases as contiguous literal title
fragments. Listed ASCII aliases MAY specify case-insensitive matching and ASCII
identifier boundaries. It MUST NOT use regular expressions, fuzzy matching,
automatic synonym/word-form expansion, Chinese segmentation, JD bodies,
metadata, or model output.

#### Scenario: V4 preserves explicit AI work but rejects bare platform inference

- **WHEN** `Agent开发工程师`, `AI全栈开发工程师（AI引擎）`, `AI工程师`, and
  `Agent工程师` are parsed
- **THEN** the first two SHALL retain their explicit `ai_application` matches
- **AND** neither SHALL acquire `platform_infra` evidence only from `AI` or
  `Agent`
- **AND** the latter two SHALL be `unclassified`.

#### Scenario: Explicit platform terms remain available

- **WHEN** `大模型平台开发工程师` and `K8S云原生开发` are parsed
- **THEN** both SHALL retain their reviewed `platform_infra` matches
- **AND** the second SHALL still retain its independently owned
  `operations_sre` match.
