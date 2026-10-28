# Spec Delta

## ADDED Requirements

### Requirement: Automate definitive source-unavailable closures
For the fixed miHoYo detail request, only three sequential successful
responses with the reviewed top-level semantic `success=false` SHALL confirm
absence for a known job missing from a complete current list.

#### Scenario: A list-absent job is explicitly unavailable in every attempt
- **WHEN** all three sequential detail recheck attempts return HTTP-successful
  JSON with top-level `success=false` for a known missing job
- **THEN** the system SHALL record first-missing evidence and automatically
  close the visible period and its active content periods in the current
  publication transaction
- **AND THEN** its closure reason SHALL be
  `detail_definitively_unavailable` with a safe stable recheck code

### Requirement: Preserve availability proved by detail revalidation
A structurally valid detail response matching a list-absent job SHALL preserve
the open period and clear its missing marker.

#### Scenario: A list-absent job remains available through detail revalidation
- **WHEN** any bounded recheck attempt returns a structurally valid detail
  response matching the missing job ID
- **THEN** the system SHALL keep its visible period open and clear any
  first-missing marker
- **AND THEN** it SHALL append safe `detail_available` lifecycle evidence
- **AND THEN** it SHALL not create a list observation or a new content version

### Requirement: Keep ambiguous recheck sequences automatic and non-closing
An ambiguous detail recheck sequence SHALL preserve the open period and remain
eligible for later successful scheduled rechecks without a manual queue.

#### Scenario: A list-absent job has an ambiguous recheck sequence
- **WHEN** any detail recheck attempt times out, is rate-limited, fails at the
  transport/HTTP layer, is malformed, or has an unrecognized body
- **THEN** the system SHALL classify that sequence as indeterminate
- **AND THEN** it SHALL not close the visible period or require a manual
  confirmation
- **AND THEN** a later complete scheduled run SHALL be allowed to recheck it
