# Spec Delta

## ADDED Requirements

### Requirement: Isolate detail-level failures

After a source list has passed pagination validation, the system SHALL classify each detail request independently. A transport error, source business failure, unknown response shape, ID mismatch, or missing required field SHALL mark only that source job ID as failed and SHALL NOT discard other valid details from the same list.

#### Scenario: One detail is malformed

- **WHEN** a validated list contains multiple jobs and one detail has an empty required body field
- **THEN** the run SHALL publish the other valid observations
- **AND THEN** the manifest and run summary SHALL record the failed ID and failed count

#### Scenario: One detail request fails transport

- **WHEN** a validated list contains multiple jobs and one detail exhausts request retries
- **THEN** the run SHALL publish the other valid observations
- **AND THEN** the failed ID SHALL be attempted again by a later list-based run

#### Scenario: A failed detail belongs to an existing visible job

- **WHEN** that job's detail fails during an otherwise usable run
- **THEN** the run SHALL not close or mark the job missing
- **AND THEN** the next run SHALL be allowed to observe it normally

#### Scenario: Every detail fails

- **WHEN** the list is valid but no detail produces a usable observation
- **THEN** the run SHALL fail without publishing an all-zero observation

### Requirement: Preserve batch-level safeguards

List-level completeness failures SHALL remain whole-run failures, and successful partial batches SHALL retain the existing archive-before-publication and audit guarantees.

#### Scenario: The source list is invalid

- **WHEN** list pagination is incomplete, duplicated, or otherwise fails validation
- **THEN** the run SHALL fail without publishing any observation
- **AND THEN** previously published history SHALL remain unchanged
