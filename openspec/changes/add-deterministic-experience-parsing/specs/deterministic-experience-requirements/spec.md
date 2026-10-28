# Spec Delta

## Purpose

This capability provides a reproducible, provider-free formal baseline for
candidate work-experience requirements while preserving source provenance and
keeping conditional or preferred experience out of the universal result.

## ADDED Requirements

### Requirement: Formal candidate experience baseline

The system SHALL derive a formal positive integer `base_year` only from
unconditional, required candidate work-experience lower bounds in a normalized
JD body version. A required general, role, domain, or technology experience
lower bound is eligible; the result SHALL be the maximum of all eligible
bounds for that version.

#### Scenario: Required general experience is configured

- **WHEN** a qualification clause requires at least three years of relevant
  work experience
- **THEN** the result status is `configured` with `base_year` equal to `3`

#### Scenario: Required technology experience contributes to the baseline

- **WHEN** a qualification clause requires five years of a named technology
  experience and no higher eligible bound exists
- **THEN** the result status is `configured` with `base_year` equal to `5`

#### Scenario: Several independent required bounds use the maximum

- **WHEN** a JD has required lower bounds of three and five years in separate
  qualification clauses
- **THEN** the result status is `configured` with `base_year` equal to `5`

### Requirement: Explicit year-expression normalization

The system SHALL recognize reviewed Arabic and Chinese numeral expressions for
candidate experience, including explicit lower-bound markers and ranges. A
range SHALL contribute its lower endpoint; Chinese numeral forms SHALL use
their reviewed numeric mapping. The parser SHALL not infer a lower bound from
an expression without an approved experience form.

#### Scenario: Range uses its lower endpoint

- **WHEN** a required candidate-experience clause specifies a three-to-five
  year range
- **THEN** the eligible lower bound is `3`

#### Scenario: Chinese numeral form is normalized

- **WHEN** a required candidate-experience clause specifies three years using
  a reviewed Chinese numeral
- **THEN** the eligible lower bound is `3`

#### Scenario: Explicit minimum marker is recognized

- **WHEN** a qualification clause says at least or no fewer than two years of
  candidate experience
- **THEN** the eligible lower bound is `2`

### Requirement: Local modality and condition scope

The system SHALL classify each recognized candidate-experience expression from
its local clause as `required`, `preferred`, `conditional`, or `uncertain`.
Priority markers SHALL affect only their local clause or explicitly attached
phrase; they SHALL NOT retroactively alter a prior clause separated by a
strong boundary. Preferred and conditional expressions SHALL NOT contribute to
the formal `base_year`.

#### Scenario: Trailing priority marker excludes its local year expression

- **WHEN** a clause states that candidates with three years of a domain
  experience are preferred
- **THEN** that expression is classified `preferred` and produces no formal
  `base_year`

#### Scenario: Separate priority condition does not erase required experience

- **WHEN** a required three-year experience clause is followed by a separate
  clause stating that a domain background is preferred
- **THEN** the three-year expression remains `required` and the formal
  `base_year` is `3`

#### Scenario: Degree-conditioned branch is not universal experience

- **WHEN** a clause gives different experience years for bachelor and master
  candidates
- **THEN** its year expressions are classified `conditional` and no formal
  `base_year` is produced from that branch

### Requirement: Non-blocking absence and ambiguity handling

The system SHALL complete deterministic processing for every eligible body
version without a model request. When no eligible required lower bound exists,
including explicit no-experience wording or preferred-only evidence, the
result SHALL be `not_configured` with no `base_year`. When a recognized
candidate year expression cannot be deterministically classified, the result
SHALL be `uncertain` with no `base_year`; neither status SHALL fail the JD
processing lifecycle.

#### Scenario: Explicitly unrestricted experience has no baseline

- **WHEN** a qualification clause explicitly says experience is unrestricted
- **THEN** the result is `not_configured` with no `base_year`

#### Scenario: Ambiguous local scope remains non-blocking

- **WHEN** a recognized candidate year expression has conflicting local
  required and priority or conditional cues that the parser cannot resolve
- **THEN** the result is `uncertain` with no `base_year` and processing
  completes successfully

### Requirement: Immutable deterministic provenance

The system SHALL persist one immutable deterministic result per body-content
version and parser/lexicon identity. Every retained recognized expression
SHALL retain its rule key, modality, source field, field-normalization
identity, and Unicode code-point range, without storing a duplicate JD
snippet, prompt, provider credential, or model response.

#### Scenario: Evidence is replayable against the body version

- **WHEN** a deterministic result is read with its source body version
- **THEN** every evidence range resolves to the same matched expression and
  carries its persisted modality and rule key

#### Scenario: Parser or lexicon revision creates a new result identity

- **WHEN** the parser or number/modality lexicon identity changes
- **THEN** the same body version is eligible for one new immutable result
  without changing earlier results

### Requirement: Incremental processing and formal read boundary

The fixed daily analysis lifecycle SHALL process body versions missing the
current deterministic experience identity, independently of AI and technology
processing. Repeated runs with unchanged inputs SHALL be idempotent. The
formal experience read SHALL use only the current deterministic result and
SHALL report unavailable when absent; it SHALL NOT fall back to an AI
experience projection.

#### Scenario: Historical catch-up then repeat run

- **WHEN** the deterministic experience stage first runs after deployment
- **THEN** it creates results for all existing unprocessed body versions, and
  an immediate repeat run creates none

#### Scenario: New body version alone is processed incrementally

- **WHEN** a later synchronization creates one new normalized body version
- **THEN** the next fixed analysis run processes that version without
  reprocessing unchanged versions or making a model request

#### Scenario: Missing deterministic result has no AI fallback

- **WHEN** the formal experience read has no result for the current identity
- **THEN** it reports the value as unavailable rather than returning an AI
  experience value
