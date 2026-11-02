# Spec Delta

## Purpose

Provide a read-only, version-aware serving contract for people to search and
inspect the latest JD inventory without exposing collection internals or
requiring a client to reconstruct the JD-modeling history.

## ADDED Requirements

### Requirement: Latest serving inventory

The system SHALL serve the latest successfully published source inventory. It
MUST default to currently effective jobs and MUST also make confirmed-closed
jobs available when requested. A source-list absence awaiting recheck MUST NOT
be represented as a confirmed closure.

#### Scenario: Default active list

- **WHEN** a client requests the job list without an activity-state filter
- **THEN** the response contains only currently effective jobs from the latest
  serving inventory

#### Scenario: Confirmed closures are requested

- **WHEN** a client requests the confirmed-closed activity state
- **THEN** the response contains only jobs with a durable confirmed-closure
  fact and excludes jobs merely awaiting a later source recheck

### Requirement: Formal field serving with transparent fallback

For every served JD dimension, the system SHALL use the formal result for the
current source version when it exists. When it does not exist, it SHALL use the
same job's newest completed formal result for that dimension. The public
response MUST NOT expose fallback, parser, audit, or processing-state details.

#### Scenario: Current formal result exists

- **WHEN** the current JD version has a completed formal technology result
- **THEN** the list and detail response use that result

#### Scenario: Current formal result is not yet available

- **WHEN** the current JD version has no completed formal education result and
  the same job has an older completed education result
- **THEN** the response serves the older formal education value without a
  processing or fallback marker

#### Scenario: No formal result exists for a dimension

- **WHEN** neither the current nor an earlier version has a completed formal
  result for a requested dimension
- **THEN** the response returns that field as unavailable rather than inventing
  a value or using an AI semantic projection

### Requirement: Structured JD list query

The system SHALL provide a paginated, deterministically ordered JD-list API.
It MUST support title, primary-job-family, other-job-family, formal technology,
education requirement, minimum work experience, project label, location,
activity state, first-discovered date, and urgent-status filters.

#### Scenario: Cross-dimension filtering

- **WHEN** a client selects a title condition and one or more technology
  conditions
- **THEN** each returned JD matches the title condition and at least one
  selected technology condition

#### Scenario: Multiple values within one filter

- **WHEN** a client selects multiple values within the same filter dimension
- **THEN** a JD matching any selected value in that dimension satisfies that
  dimension

#### Scenario: Case-insensitive title search

- **WHEN** a client supplies a non-empty title query
- **THEN** the API searches titles only, ranks an ASCII case-insensitive exact
  or substring match before a fuzzy-only match, and does not search JD-body
  text or infer synonyms

#### Scenario: Fuzzy title search

- **WHEN** a client supplies a title query with a small typographical error and
  no exact or substring match is available
- **THEN** the API MAY return titles meeting its documented, fixed fuzzy-match
  threshold, with a deterministic ordering and without expanding the query
  into synonyms or JD-body text

#### Scenario: Unknown project label is selected

- **WHEN** a client selects the unrecognized-project option
- **THEN** the API returns JDs with no deterministic title-project assignment

### Requirement: List ordering and field semantics

The list API SHALL support ascending and descending ordering by minimum work
experience and education requirement. It MUST sort missing or unavailable
values after all configured values in either direction and preserve the
direction of an education bound in its display value.

#### Scenario: Experience descending sort

- **WHEN** experience ordering is descending
- **THEN** configured values appear from greatest to least and JDs without a
  configured or available value appear after them

#### Scenario: Upper-bound education requirement

- **WHEN** a JD formally requires an upper education bound such as
  `associate_or_lower`
- **THEN** the response represents it as an upper-bound requirement rather than
  mislabelling it as a minimum education level

### Requirement: JD detail response

The system SHALL provide a read-only detail response for a JD returned by the
list API. The response MUST include its display metadata, served formal
dimensions, and its current JD responsibility, requirement, and supplemental
sections. It MUST NOT provide historical text comparison or internal audit
records.

#### Scenario: Open job detail

- **WHEN** a client requests the detail for a currently effective listed JD
- **THEN** it receives the JD's current display and body sections together with
  its served formal fields

#### Scenario: Invalid list query

- **WHEN** a client submits an invalid filter, sort key, or pagination value
- **THEN** the API returns a safe client error and does not return a partial
  unbounded result set
