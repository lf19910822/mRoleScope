# Job Trend Repair

## ADDED Requirements

### Requirement: Repair a missing observation from a successful source run
The Worker SHALL create one immutable published observation for a missing
target date only when the source date has a successful published run in the
same fixed observation scope.

#### Scenario: Repair 2026-11-06 from 2026-11-07
- **WHEN** the repair operation is invoked with those dates
- **THEN** the target trend node contains the source run's job membership and
  historical detail is available through the normal date-based drill-down
  APIs

#### Scenario: Existing target publication
- **WHEN** the target date already has a successful publication
- **THEN** the operation rejects the request without changing that publication

#### Scenario: Repeat the same repair
- **WHEN** the same source and target are requested after a successful repair
- **THEN** the operation returns the existing repair result without duplicating
  memberships or audit facts

### Requirement: Keep repair provenance internal
The system SHALL audit the source and target relationship internally while
public trend, drill-down, and detail responses SHALL omit repair markers and
storage identifiers.

#### Scenario: Public response remains display-safe
- **WHEN** a repaired date is read through trend or drill-down APIs
- **THEN** the response has the same shape as an ordinary formal observation
  and contains no repair label, run ID, or version ID
