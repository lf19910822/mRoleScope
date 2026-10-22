# Spec Delta

## Purpose

Defines a remote-only, reproducible technology-mention analysis capability whose results, evidence, review decisions, and release state can be independently verified.

## ADDED Requirements

### Requirement: Only approved deterministic dictionary rules create mentions
The system SHALL create technology mentions only from dictionary entries marked enabled with an approved canonical identity, explicit aliases, case treatment, and testable boundary rule. It SHALL classify every other entry as conditional or candidate and SHALL not let it create a mention or statistic.

#### Scenario: An approved alias is encountered
- **WHEN** normalized body text matches an enabled alias under that entry's boundary rule
- **THEN** the system SHALL emit the entry's canonical technology identity and retain the matched alias as evidence metadata

#### Scenario: A conditional or candidate term is encountered
- **WHEN** text or a source tag contains a term whose dictionary state is conditional or candidate
- **THEN** the system SHALL not emit a technology mention from that term
- **AND THEN** it MAY retain a remote-only review candidate without body text

#### Scenario: Similar technology names overlap
- **WHEN** text contains a longer registered technology name that includes a shorter distinct name
- **THEN** the system SHALL apply the enabled entries' explicit boundary and overlap rules
- **AND THEN** it SHALL not infer the shorter technology merely from its substring

### Requirement: Technology mentions have a deliberately narrow semantic scope
The system SHALL analyze only `description`, `jobRequire`, and `addition` of a body-content version. It SHALL report the result as a technology mention, not a required skill, qualification, job family, or technology trend. It SHALL exclude unapproved broad directions and source tags from this result.

#### Scenario: A source-metadata-only change is observed
- **WHEN** a later source observation changes title, location, source tag, or another metadata value while its three body fields are unchanged
- **THEN** the system SHALL not create a new technology-mention result solely for that observation

#### Scenario: A body sentence lists alternatives
- **WHEN** an enabled technology name appears in a sentence that describes alternatives or preferences
- **THEN** the system SHALL record only that the technology was mentioned
- **AND THEN** it SHALL not classify the mention as required, optional, or selected from an alternative group

### Requirement: Evidence locations are reproducible and complete
The system SHALL retain a separate evidence record for every technology match, including its body source field, normalization/text identity, and zero-based Unicode-code-point half-open interval `[start, end)`. It SHALL validate that the interval is in range and corresponds to the approved alias in the exact normalized field text.

#### Scenario: One technology appears in two positions
- **WHEN** the same canonical technology is matched more than once in one body-content version
- **THEN** the system SHALL retain every distinct valid evidence interval
- **AND THEN** it SHALL produce only one job-level mention for that canonical technology in that analysis result

#### Scenario: An evidence interval cannot be validated
- **WHEN** a proposed interval is out of bounds or does not correspond to the registered alias in its normalized field text
- **THEN** the system SHALL reject that analysis result as invalid
- **AND THEN** it SHALL not publish a partial replacement result

### Requirement: Analysis results are reusable and separate from source collection
The system SHALL identify a technology-mention result by body-content version, parser version, and dictionary version, rather than by source collection run. It SHALL reuse an existing complete result for that identity and process newly available or changed body content independently of the daily source synchronization outcome.

#### Scenario: An unchanged body is seen in later daily runs
- **WHEN** a body-content version already has a complete result for the active parser and dictionary versions
- **THEN** later observations of that body SHALL reuse the complete result
- **AND THEN** the system SHALL not create duplicate job-level mentions or evidence records

#### Scenario: Analysis fails after a source run is published
- **WHEN** downstream technology analysis fails for an already published source observation
- **THEN** the system SHALL retain the source run, source versions, and visibility history as published
- **AND THEN** it SHALL record a safe analysis failure without re-collecting the source batch

### Requirement: Dictionary and parser upgrades are fully reanalyzed before publication
The system SHALL treat each dictionary or parser version as a distinct analysis version. It SHALL reanalyze all available historical body-content versions for a new version and SHALL publish that version for downstream use only after the entire reanalysis and release validation complete successfully.

#### Scenario: An approved dictionary adds a new alias
- **WHEN** a new dictionary version adds or changes an enabled alias or matching boundary
- **THEN** the system SHALL reanalyze historical body-content versions with the new version
- **AND THEN** it SHALL identify resulting differences as analysis-rule coverage changes rather than source-demand changes

#### Scenario: Reanalysis is incomplete or fails
- **WHEN** a reanalysis version fails, is cancelled, or has not completed validation
- **THEN** the system SHALL keep the previously published analysis version available
- **AND THEN** it SHALL not expose a mixture of old and new version results as one current result set

### Requirement: Review, audit, and logs preserve result provenance without body exposure
The system SHALL retain remote-only review decisions separately from automated results, including correct, incorrect, or uncertain status; error category; reviewer and time; and associated parser/dictionary versions. Task audit and error output SHALL contain only safe identifiers, versions, counts, coordinates, and stable error categories, never source body text, snippets, raw responses, credentials, or tokens.

#### Scenario: A reviewer finds a false positive
- **WHEN** a reviewer marks an automated mention as incorrect
- **THEN** the system SHALL retain the automated result and its evidence unchanged
- **AND THEN** it SHALL add an independent review decision that does not automatically apply to another body, dictionary, or parser version

#### Scenario: A first analysis version is released
- **WHEN** a candidate analysis version is considered for publication
- **THEN** the system SHALL require remote verification of evidence coordinates, idempotent rerun behavior, redaction behavior, and the defined human review sample
- **AND THEN** each enabled entry that produced a real match SHALL have at least one reviewed remote evidence record
