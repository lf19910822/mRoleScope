# Spec Delta

## Purpose

Defines a remote-only, reproducible technology-mention analysis capability whose results, evidence, review decisions, and release state can be independently verified.

## ADDED Requirements

### Requirement: Candidate dictionary lifecycle is explicitly approved and isolated
The system SHALL import the fixed v0.1 dictionary only as non-executable candidate or conditional rules. It SHALL require an explicit, durable human approval for each deterministic candidate rule before that rule becomes enabled, and SHALL reject conditional rules from this approval path pending a context-aware policy. It SHALL not enable rules, create analysis work, or publish a configuration merely because a candidate dictionary has been imported.

#### Scenario: The fixed v0.1 dictionary is seeded
- **WHEN** an authorized remote operator imports the fixed v0.1 dictionary
- **THEN** all imported rules SHALL remain candidate or conditional
- **AND THEN** the system SHALL not create an analysis configuration or analysis work item solely from the import

#### Scenario: A deterministic candidate rule is explicitly approved
- **WHEN** an authorized remote operator approves one candidate rule key
- **THEN** the system SHALL enable only that rule and append an immutable approval decision with the authenticated operator identity
- **AND THEN** a repeated approval request for that enabled rule SHALL be idempotent

#### Scenario: A context-dependent rule is submitted to the deterministic approval path
- **WHEN** an operator attempts to approve a conditional rule through the fixed v0.1 approval operation
- **THEN** the system SHALL reject the request
- **AND THEN** it SHALL leave the rule non-executable

#### Scenario: Approved rules are prepared for candidate analysis
- **WHEN** at least one deterministic v0.1 rule has an explicit approval and an operator prepares the fixed candidate configuration
- **THEN** the system SHALL create or reuse a candidate configuration for the fixed parser, dictionary version, and immutable snapshot of the currently approved deterministic rules, and queue every historical body-content version
- **AND THEN** it SHALL not collect from the source, change the daily synchronization timer, or advance the published analysis pointer

#### Scenario: A rule approval changes the effective candidate rule set
- **WHEN** an operator approves an additional deterministic rule after a candidate configuration has been prepared or analyzed
- **THEN** the system SHALL treat the new enabled-rule set as a distinct immutable analysis input
- **AND THEN** the next preparation SHALL create a distinct candidate configuration and queue every historical body-content version without overwriting or reusing results from the prior rule-set snapshot

#### Scenario: Candidate analysis is invoked
- **WHEN** an authorized operator runs the fixed v0.1 candidate-analysis operation
- **THEN** it SHALL process only the isolated candidate configuration through a no-port, backend-only worker service
- **AND THEN** normal pending analysis SHALL continue to resolve only the published configuration

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
The system SHALL identify a technology-mention result by body-content version and an immutable analysis configuration that includes parser version, dictionary version, and its frozen approved-rule set, rather than by source collection run. It SHALL reuse an existing complete result only for that exact identity and process newly available or changed body content independently of the daily source synchronization outcome.

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

#### Scenario: A new approval changes an existing candidate dictionary's executable rules
- **WHEN** an additional deterministic rule in an existing candidate dictionary becomes enabled
- **THEN** the system SHALL create a distinct approved-rule-set snapshot and reanalyze every historical body-content version for that snapshot
- **AND THEN** it SHALL retain the prior candidate configuration, results, evidence, and reviews for audit

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

### Requirement: A released dictionary and its configuration are frozen together
The system SHALL publish a candidate dictionary version and its exact completed analysis configuration together. It SHALL reject rule, alias, boundary, or approval changes to a published dictionary version; such changes SHALL require a new candidate dictionary version and full reanalysis.

#### Scenario: The first complete candidate is published
- **WHEN** an authorized operator releases a candidate whose dictionary, frozen rule set, and required body results satisfy the release gate
- **THEN** the system SHALL atomically mark that dictionary version and analysis configuration published and advance the sole analysis publication pointer to that configuration
- **AND THEN** it SHALL preserve the configuration's exact rule-set fingerprint and all prior candidate evidence

#### Scenario: A change is proposed for a published dictionary
- **WHEN** an operator attempts to add, enable, disable, or alter a rule in a published dictionary version
- **THEN** the system SHALL reject the change
- **AND THEN** it SHALL require a new candidate dictionary version before the changed rules can be analyzed or published

### Requirement: Publication has a durable, controlled release attestation
The system SHALL expose publication only through a fixed, authorized remote operation that rechecks release readiness. It SHALL persist an immutable attestation containing the released configuration and dictionary identities, rule-set fingerprint, release-gate policy version, timestamp, and resolved publisher identity; no raw JD text or arbitrary caller input may enter the attestation.

#### Scenario: A candidate passes the controlled release gate
- **WHEN** the fixed release operation verifies complete and non-failed work, full candidate coverage, configured evidence validation, and the approved review policy
- **THEN** it SHALL atomically create the release attestation and publish the candidate
- **AND THEN** it SHALL record the authenticated remote operator as publisher, or `FAN` only when that identity is unavailable

#### Scenario: A candidate is incomplete or fails validation
- **WHEN** the controlled release operation finds pending, missing, failed, or gate-invalid candidate work
- **THEN** it SHALL leave the dictionary, configuration, and publication pointer unchanged
- **AND THEN** it SHALL return only safe counts and stable error categories

### Requirement: A published first baseline can be safely withdrawn
The system SHALL allow a fixed, authorized withdrawal operation for a published first baseline only when a material result-quality defect or a security/privacy incident is declared using a fixed category. Withdrawal SHALL retain all historical facts, remove the active publication pointer, and leave source synchronization independent of analysis availability.

#### Scenario: A material quality or privacy problem is confirmed
- **WHEN** an authorized operator withdraws the only published baseline with an allowed withdrawal category
- **THEN** the system SHALL retire that dictionary and configuration, append an immutable withdrawal attestation, and clear the publication pointer atomically
- **AND THEN** daily source synchronization SHALL continue while ordinary downstream analysis safely remains unavailable until a replacement is published

#### Scenario: Ordinary downstream processing has a transient failure
- **WHEN** a normal analysis run fails without a confirmed material quality or security/privacy issue
- **THEN** the system SHALL retain the published dictionary and configuration
- **AND THEN** it SHALL keep only the affected work pending or safely failed for later recovery
