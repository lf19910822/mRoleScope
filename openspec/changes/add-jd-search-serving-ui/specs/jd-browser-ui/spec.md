# Spec Delta

## Purpose

Provide a clear, read-only browser experience for finding and reading current
JDs through their formal modeled dimensions, without turning internal
processing quality or source-collection mechanics into user-facing concepts.

## ADDED Requirements

### Requirement: JD list experience

The browser SHALL provide a paginated JD list with composable structured
filters and experience/education sort controls. It MUST initialize to
currently effective jobs and preserve selected list state while the user
inspects a JD detail and returns to the list.

#### Scenario: Default visit

- **WHEN** a visitor opens the JD browser without query state
- **THEN** the page displays currently effective JDs and no restrictive
  structured filter is selected

#### Scenario: Composed filters

- **WHEN** a visitor selects values in more than one filter dimension
- **THEN** the list displays only JDs satisfying every selected dimension

#### Scenario: Same-dimension selection

- **WHEN** a visitor selects several technology or project values
- **THEN** the list includes a JD matching any one of those values while still
  applying other selected dimensions

### Requirement: Model-field presentation

The browser SHALL present education as `学历要求`, not `最低学历`, and SHALL
display the normalized levels 专科及以下、本科、硕士、博士 with an upper-bound
expression where applicable. It MUST present one main job family and any other
matched job families as separate concepts.

#### Scenario: Multiple job-family hits

- **WHEN** a JD has one main job family and additional family matches
- **THEN** the interface labels the unique main family separately from its
  other matched job families and does not display a parent-child taxonomy

#### Scenario: No configured formal requirement

- **WHEN** a served formal education or experience value is explicitly not
  configured
- **THEN** the interface displays an understandable no-explicit-requirement
  label instead of numeric zero

### Requirement: JD detail experience

The browser SHALL let a visitor open a read-only JD detail view from the list.
The detail MUST display the served job fields, formal modeled dimensions,
responsibility text, requirement text, and supplemental text when present.

#### Scenario: Detail is opened

- **WHEN** a visitor selects a JD from the list
- **THEN** the interface shows that JD's title, metadata, modeled dimensions,
  and available body sections

#### Scenario: No supplemental text exists

- **WHEN** a JD has no supplemental section
- **THEN** the detail omits that section without treating the JD as erroneous

### Requirement: Viewer-facing information boundary

The browser SHALL not display parser versions, source/body version IDs,
coverage rates, processing states, fallback status, audit records, model
responses, credentials, or a historical text-difference view.

#### Scenario: Older result is served internally

- **WHEN** the serving API transparently uses a previous completed formal value
- **THEN** the interface displays the served value without a data-delay or
  fallback indicator

#### Scenario: Viewer inspects a JD

- **WHEN** a visitor opens list filters or a JD detail
- **THEN** no internal analysis audit or provider information is rendered
