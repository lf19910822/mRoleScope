# Spec Delta

## Purpose

Defines independent, reproducible source-metadata and JD-body versions so each successful observation can be reconstructed and safely consumed by downstream analysis.

## ADDED Requirements

### Requirement: Stable identity has independently versioned observation facts
The system SHALL retain `(source_company, source_job_id)` as the stable job identity. Each job observed in a successfully published source run SHALL reference both the source-metadata version and the JD-body-content version that were current for that observation, without changing existing presence history or COS evidence.

#### Scenario: An unchanged job is observed in a later successful run
- **WHEN** a job appears in two successfully published runs with equivalent source metadata and body content
- **THEN** both observations SHALL reference the same source-metadata version and body-content version
- **AND THEN** no duplicate version payload SHALL be created

#### Scenario: A previously absent job ID reappears
- **WHEN** a stable job ID is observed after its earlier visible period closed
- **THEN** the new observation SHALL preserve that stable job identity and reference the versions observed at reappearance
- **AND THEN** prior presence and version history SHALL remain available

### Requirement: Source metadata versions use the approved business whitelist
The system SHALL create or reuse a source-metadata version from the approved metadata whitelist: title; location ID/display-text pairs; official category, work-nature and hire-type ID/name pairs; hurry state; source project ID/name; source object ID/name; and deduplicated source tags. It SHALL preserve absent, null, empty and invalid values as distinct states unless an equivalence rule is later versioned explicitly.

#### Scenario: Only a title changes
- **WHEN** a job has the same body content but a different title
- **THEN** the system SHALL create or reuse a different source-metadata version and retain the prior metadata version
- **AND THEN** it SHALL reuse the existing body-content version

#### Scenario: Location order changes without a location-set change
- **WHEN** the source returns the same location ID/display-text pairs in a different array order
- **THEN** the system SHALL reuse the existing source-metadata version

#### Scenario: An excluded volatile field changes
- **WHEN** only a field outside the approved metadata whitelist changes, including visitor state, status code, request/channel metadata, collection time or archive digest
- **THEN** the system SHALL not create a source-metadata or body-content business version solely for that change
- **AND THEN** the complete source batch SHALL remain recoverable from its private COS archive

### Requirement: Body-content versions have deterministic three-field inputs
The system SHALL derive a body-content version only from normalized `description`, `jobRequire`, and `addition` values. Its normalization rules and each field's normalized-text hash SHALL be versioned with the body content so the exact analysis input is determinable in the remote environment.

#### Scenario: A body field changes
- **WHEN** any one of the three normalized body fields differs for the same stable job
- **THEN** the system SHALL create or reuse a different body-content version
- **AND THEN** it SHALL retain the previous body-content version for history and downstream-result reuse

#### Scenario: Source-only metadata changes
- **WHEN** all three normalized body fields are unchanged but an approved source-metadata value changes
- **THEN** the system SHALL not create a new body-content version
- **AND THEN** downstream body analysis SHALL not be required to rerun for that observation

### Requirement: Migration preserves the initialized baseline without local raw data
The system SHALL migrate the existing synchronized baseline into the separated version model without deleting or rewriting existing job identity, run, presence, content-period, or COS-archive facts. Migration and verification SHALL operate on the remote database/COS environment; repository fixtures and local tests SHALL contain synthetic text only.

#### Scenario: The initialized production baseline is upgraded
- **WHEN** the additive migration is applied to the existing initialized database
- **THEN** every existing published observation SHALL have reconstructable source-metadata and body-content version references
- **AND THEN** existing history queries and archive references SHALL remain valid

#### Scenario: A test validates the migration
- **WHEN** the migration or its tests execute outside the remote production environment
- **THEN** they SHALL use synthetic fixtures and SHALL not require, copy, or emit real JD text
