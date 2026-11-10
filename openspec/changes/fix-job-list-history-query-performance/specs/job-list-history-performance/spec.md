# Job List History Performance

## ADDED Requirements

### Requirement: Bounded list history derivation

The job list SHALL derive the latest observed modification dates without
recomputing the entire history transition set for each matching job.

#### Scenario: Multiple matching jobs

- **WHEN** a list request returns or sorts multiple matching jobs
- **THEN** the history transition aggregate is computed at most once for that request
- **AND** the same modification dates and ordering remain available to the reader

#### Scenario: Historical repair or lifecycle changes

- **WHEN** retained published history includes a repair, closure, or reopening
- **THEN** the list uses the same history semantics as before the optimization
- **AND** the query does not depend on a stale persisted cache
