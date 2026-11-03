# Spec Delta

## ADDED Requirements

### Requirement: Serving technology data uses only the active publication

The Serving projection SHALL select a technology-analysis result only when its
configuration equals the sole configuration referenced by
`technology_analysis_publication_state`.  Completed candidate, superseded, or
otherwise non-active configurations SHALL not affect current JD technology
keywords, technology filters, or public filter options.

#### Scenario: A newer candidate has not been published

- **WHEN** a JD has both an active published technology result and a newer
  complete candidate result
- **THEN** Serving SHALL resolve the active published result only

#### Scenario: A published upgrade advances the pointer

- **WHEN** the active technology publication pointer advances to a complete
  upgraded configuration and Serving reconciliation runs
- **THEN** Serving SHALL resolve that upgraded configuration and expose its
  canonical technology identities through the existing filter-options API

### Requirement: v0.3 release refreshes Serving after commit

The fixed v0.3 publish-upgrade worker operation SHALL reconcile the Serving
inventory after its immutable release transaction succeeds or is safely
reused.  A reconciliation failure SHALL not roll back that release and SHALL
be retried by the same fixed operation.

#### Scenario: Serving reconciliation cannot complete

- **WHEN** v0.3 release has committed but Serving reconciliation fails
- **THEN** the operation SHALL return a safe failure
- **AND THEN** rerunning the fixed v0.3 publish-upgrade operation SHALL retry
  the reconciliation without creating another release attestation
