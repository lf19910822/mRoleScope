# Spec Delta

## Purpose

Provide a deterministic, versioned and evidence-backed education constraint
for each JD body version, so product-facing JD modelling does not depend on a
model provider for explicit education requirements.

## ADDED Requirements

### Requirement: Normalize recognized education terms
The system SHALL normalize recognized degree terms to one ordered level:
`associate_or_lower`, `bachelor`, `master`, or `doctor`. `研究生` SHALL map
to `master`; an explicit doctorate expression SHALL take precedence over a
generic nested term. The parser SHALL retain the exact matched source range
and the versioned rule that performed the mapping.

#### Scenario: Vocational and associate terms map below bachelor
- **WHEN** an eligible qualification sentence requires `大专及以上` or `职高及以上`
- **THEN** its lower-bound level is `associate_or_lower` with the matching rule and source evidence

#### Scenario: Postgraduate and doctoral terms do not collide
- **WHEN** an eligible qualification sentence contains `博士研究生及以上`
- **THEN** its lower-bound level is `doctor`, not `master`

#### Scenario: Product-defined generic postgraduate mapping
- **WHEN** an eligible qualification sentence requires `研究生及以上`
- **THEN** its lower-bound level is `master`

### Requirement: Preserve education constraint direction and modality
The system SHALL separately represent an education lower bound, an education
upper bound, and its modality. Required terms create product-facing
constraints; preferred terms SHALL not increase a required lower bound; an
unrestricted expression SHALL not create a required lower bound.

#### Scenario: Required lower bound with preferred higher degree
- **WHEN** a qualification sentence states `本科及以上，硕士优先`
- **THEN** the result has required lower bound `bachelor` and does not promote it to `master`

#### Scenario: Upper-bound expression is not misreported as a minimum
- **WHEN** a qualification sentence states `本科及以下`
- **THEN** the result records upper bound `bachelor` and no invented required lower bound

#### Scenario: Unrestricted education
- **WHEN** a qualification sentence states `学历不限，博士优先`
- **THEN** the result has no required lower or upper bound and records no preferred term as a minimum

### Requirement: Resolve alternatives and ambiguity conservatively
The system SHALL select the lowest permitted level for an explicit required
alternative. It SHALL return `uncertain` rather than invent one result when
separate hard requirements conflict or cannot be assigned to one job scope.

#### Scenario: Explicit degree alternative
- **WHEN** a qualification sentence states `本科或硕士均可`
- **THEN** the required lower bound is `bachelor`

#### Scenario: Unresolvable role-specific requirements
- **WHEN** one JD states separate, unscoped hard requirements for `算法岗硕士` and `工程岗本科`
- **THEN** the result status is `uncertain` with evidence for the competing rules

### Requirement: Produce immutable incremental deterministic results
The system SHALL produce at most one deterministic education result for a
body-content version and immutable parser/lexicon version pair. It SHALL
process only unprocessed new or changed body versions and SHALL be idempotent
for an unchanged version. Processing SHALL not require model credentials or
issue provider requests.

#### Scenario: Repeated daily processing
- **WHEN** deterministic education processing runs twice with unchanged body versions and the same parser and lexicon versions
- **THEN** it creates no second result and does not enqueue or call an AI provider

#### Scenario: Changed JD body
- **WHEN** a job receives a new body-content version after its education result was completed
- **THEN** processing creates a result for the new version without changing the historical result

### Requirement: Use deterministic education for formal JD modelling
The product-facing JD-model education value SHALL be read from the completed
deterministic education result for the applicable body version. Existing AI
education projections and their audits SHALL remain readable historical data
and SHALL not be overwritten or used as the formal deterministic result.

#### Scenario: Deterministic result supersedes AI projection for display
- **WHEN** a body version has both an AI education projection and a completed deterministic education result
- **THEN** the formal JD-model education read returns the deterministic result and leaves the AI projection unchanged
