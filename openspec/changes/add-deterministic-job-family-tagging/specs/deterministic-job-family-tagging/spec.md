# deterministic-job-family-tagging Specification

## Purpose

Provide a provider-free, title-only, versioned job-family result for technical
recruitment JD titles. The capability produces at most one primary job family
per eligible title and retains all selected per-family matches for audit.

## ADDED Requirements

### Requirement: The capability SHALL classify only an eligible canonical title

The system SHALL use only the normalized title stored in the source-metadata
history and reviewed project-title mask ranges when deriving a job family. It
MUST NOT classify from JD body fields, source `projectName` or `objectName`,
technology records, AI inputs, AI outputs, similar jobs, or external data.

#### Scenario: Non-title fields cannot create a family

- **GIVEN** an in-scope source-metadata observation with the canonical title
  `通用工程师`
- **AND** its JD body or source project metadata contains backend-related text
- **WHEN** the system derives the job family
- **THEN** it SHALL use only title matches after reviewed project masking
- **AND** it SHALL NOT classify the title from those non-title fields.

### Requirement: The capability SHALL retain explicit scope and title-quality statuses

An observation with official-category ID `"1"` SHALL be in scope. Another
valid official-category ID SHALL be `out_of_scope`. An absent, null, empty, or
invalid ID SHALL be `scope_unknown`. An in-scope observation without a valid
non-empty normalized title SHALL be `invalid_title`.

#### Scenario: Scope is explicit rather than inferred from a title

- **GIVEN** observations with the same title and respectively official-category
  ID `"1"`, ID `"2"`, and a missing category ID
- **WHEN** formal job-family results are read
- **THEN** the first observation SHALL be eligible for title parsing
- **AND** the second SHALL be `out_of_scope`
- **AND** the third SHALL be `scope_unknown`
- **AND** neither non-eligible observation SHALL receive a primary family.

#### Scenario: Missing title remains a data-quality outcome

- **GIVEN** an in-scope observation without a valid non-empty normalized title
- **WHEN** formal job-family status is read
- **THEN** it SHALL be `invalid_title`
- **AND** it SHALL NOT be counted as `unclassified`.

### Requirement: The capability SHALL use a reviewed one-to-one family dictionary

The system SHALL use an immutable, versioned reviewed dictionary containing
the approved stable family keys: `algorithm_research`, `backend`, `client`,
`data_engineering`, `engineering_management`, `engine_systems`, `frontend`,
`hardware_embedded`, `operations_sre`, `platform_infra`, `security`,
`testing_quality`, and `toolchain`.

Every enabled complete alias SHALL map to exactly one family key. The system
MUST reject a dictionary in which one complete alias maps to more than one
family.

#### Scenario: One alias cannot have context-dependent ownership

- **GIVEN** a proposed dictionary assigns the complete alias `平台` to two
  family keys
- **WHEN** the dictionary is loaded
- **THEN** the system SHALL reject it
- **AND** a more specific reviewed alias is required to express distinct uses.

### Requirement: The capability SHALL match only explicit literal aliases

The system SHALL match listed Chinese aliases as contiguous literal title
fragments. Listed ASCII aliases MAY specify case-insensitive matching and ASCII
identifier boundaries. It MUST NOT use regular expressions, fuzzy matching,
automatic synonym/word-form expansion, or Chinese segmentation.

#### Scenario: Explicit aliases classify without semantic expansion

- **GIVEN** the reviewed aliases `后端` and `服务端` map to `backend`
- **WHEN** titles contain `后端开发工程师` and `服务端运维`
- **THEN** both titles SHALL have a `backend` match
- **AND** an unlisted generalized expression SHALL NOT be classified merely by
  semantic similarity.

### Requirement: The capability SHALL mask only reviewed project fragments before family matching

Before family matching, the system SHALL obtain ranges from the current
reviewed project-title parser and mask only those exact ranges. A project mask
identity SHALL include the project parser and dictionary identities used to
derive it. The system MUST NOT use source-project metadata as a mask or mask a
generic family/technology word merely because it occurs in a project record.

#### Scenario: A full reviewed project phrase is masked while title role remains

- **GIVEN** a reviewed project title range `AI引擎` in `全栈开发-AI引擎`
- **WHEN** the family parser runs
- **THEN** it SHALL ignore only the `AI引擎` range
- **AND** it SHALL retain the `全栈` family match.

#### Scenario: A generic engine role is not masked

- **GIVEN** `引擎` is not itself a reviewed complete project-title entry
- **WHEN** the family parser evaluates `游戏引擎开发`
- **THEN** the engine-family alias SHALL remain eligible to match.

### Requirement: The capability SHALL preserve same- and cross-family matches

For each family independently, the system SHALL retain non-overlapping alias
occurrences and SHALL retain the longest same-family alias where occurrences
overlap. Different-family matches SHALL not suppress one another, including
when their ranges overlap. The result SHALL retain each matched family's count
and evidence.

#### Scenario: Cross-family overlap is not globally suppressed

- **GIVEN** `客户端` maps to `client` and `安全` maps to `security`
- **WHEN** the parser evaluates `客户端安全开发`
- **THEN** it SHALL retain one `client` match and one `security` match
- **AND** it SHALL not choose a winner by globally suppressing one range.

#### Scenario: Same-family overlap is not double counted

- **GIVEN** `后端开发` and `后端` both map to `backend`
- **WHEN** the parser evaluates `后端开发工程师`
- **THEN** it SHALL retain the longer `后端开发` evidence
- **AND** it SHALL count `backend` once for that overlapping range.

### Requirement: The capability SHALL choose at most one deterministic primary family

The system SHALL select a primary family by greatest count, then by the last
selected match end position, then by lexicographically first family key. With
no unmasked family alias, the completed result SHALL be `unclassified` with no
primary family. The system SHALL not create a second primary family,
`conflicted` state, auxiliary direction, or guessed `other` family.

#### Scenario: Later title match resolves an equal count

- **GIVEN** `服务端` maps to `backend` and `运维` maps to `operations_sre`
- **WHEN** the parser evaluates `服务端运维`
- **THEN** it SHALL retain counts of one for both families
- **AND** `operations_sre` SHALL be primary because its selected match ends
  later in the title.

### Requirement: The capability SHALL persist immutable, versioned results and auditable observations

The system SHALL persist immutable source-metadata observations, title
identities, completed title results, match counts, and alias/range evidence.
Result records SHALL reference existing title history rather than duplicate
title text or JD text.

#### Scenario: Metadata-only change reuses a title result

- **GIVEN** two in-scope source-metadata versions share one canonical title
  but have different location metadata
- **WHEN** pending job-family processing runs for the same parser and
  dictionary identities
- **THEN** it SHALL create one title-level result
- **AND** both observations SHALL resolve to that result.

### Requirement: The capability SHALL create a new result for a changed result identity

Each title result identity SHALL include its title identity, job-family parser
and dictionary identity, and project-mask parser and dictionary identity. The
system SHALL retain completed `unclassified` results. A title or either parser
or dictionary identity change SHALL select the relevant title for a new result.

#### Scenario: Dictionary revision catches up known titles

- **GIVEN** completed title results under one family-dictionary identity
- **WHEN** a new family-dictionary identity is released
- **THEN** pending processing SHALL select known eligible title identities for
  the new identity
- **AND** it SHALL retain the earlier immutable results.

### Requirement: The capability SHALL process title results independently and preserve retry audit

The fixed analysis operation SHALL run job-family processing without provider
credentials or outbound model calls. The processor SHALL handle eligible title
identities independently. A safe unexpected per-title failure SHALL retain an
audit event containing identity, stable safe error code, processing time, and
retry history, and SHALL leave only that title pending for a later run or
manual requeue.

#### Scenario: One title failure does not suppress another title

- **GIVEN** two pending eligible titles and one encounters a retryable storage
  failure
- **WHEN** the pending processor runs
- **THEN** it SHALL preserve safe retry audit for the failed title
- **AND** it SHALL still publish the independent successful title result
- **AND** it SHALL not label the failed title `unclassified`.

### Requirement: The capability SHALL isolate observation failures without source-text audit

The processor SHALL create source-metadata observations independently. If an
observation fails before a title identity exists, the system SHALL retain a
separate immutable audit event containing only the source-metadata reference,
processing identity, stable safe error code, and processing time. It SHALL NOT
store title or JD text and SHALL leave only that source-metadata version
pending.

#### Scenario: One observation failure does not suppress another title

- **GIVEN** two source-metadata versions without job-family observations and
  insertion of one observation encounters a retryable storage failure
- **WHEN** the pending processor runs
- **THEN** it SHALL preserve a source-metadata-only safe retry audit for that
  failed observation
- **AND** it SHALL still create and publish the independent successful title
  result
- **AND** the failed source-metadata version SHALL remain unavailable until a
  later run successfully creates its observation.

### Requirement: The capability SHALL not fall back after a title failure

The system MUST NOT convert such a failure to `unclassified`, attach an older
title's family as the current title result, or prevent remaining job-family
items and other deterministic analysis stages from being attempted.

#### Scenario: Title change cannot borrow an old family

- **GIVEN** a job's former title has a completed `frontend` result
- **AND** its later title is pending after an unexpected parse failure
- **WHEN** the formal result for the later metadata version is read
- **THEN** it SHALL be unavailable or pending for the later title
- **AND** it SHALL NOT return the former title's `frontend` result.
