# Spec Delta

## Purpose

Provide a concise, stable interaction contract for finding current JDs by any
formal job-family hit, thresholding explicit work-experience requirements,
and arranging the resulting list with reader-chosen ordered sorts.

## ADDED Requirements

### Requirement: Unified job-family filter

The JD browser SHALL expose one `岗位族` filter. Its selected values MUST
match any formal job-family hit of a JD, regardless of whether that hit is the
primary family or another matched family. Values in this filter are ORed.

#### Scenario: Selected family is the primary family

- **WHEN** a visitor selects `后端` and a JD has `后端` as its primary family
- **THEN** the JD is included if it satisfies every other selected dimension

#### Scenario: Selected family is another family hit

- **WHEN** a visitor selects `AI 应用` and a JD has `后端` as its primary
  family plus `AI 应用` as another hit
- **THEN** the JD is included if it satisfies every other selected dimension

#### Scenario: Several job-family values are selected

- **WHEN** a visitor selects both `后端` and `运维`
- **THEN** a JD matching either family satisfies the job-family dimension

### Requirement: Primary and other family presentation

The browser SHALL continue to present a returned JD's one primary job family
and its zero or more other matched families separately. The unified filter
MUST NOT relabel the other matches as child categories.

#### Scenario: Unified-filter result is displayed

- **WHEN** a JD was included because of an other matched family
- **THEN** its card and detail retain their primary-family and other-family
  labels from the served data

### Requirement: Minimum work-experience threshold

The JD browser SHALL offer one ordinary `最低工作年限` select with `不限` and
exactly the values `1 年及以上` through `9 年及以上`. Its values MUST NOT be
derived from the currently returned JD inventory. The selected value MUST
match a JD only when its formal experience requirement is configured and its
minimum year is greater than or equal to that value.

#### Scenario: An explicit threshold is selected

- **WHEN** a visitor selects `3 年及以上`
- **THEN** a JD with a configured minimum of five years is included and one
  with a configured minimum of two years is excluded

#### Scenario: The threshold is not restricted

- **WHEN** a visitor chooses the unrestricted state
- **THEN** the browser sends no minimum-experience threshold and JDs without
  a configured experience requirement remain eligible

### Requirement: Ordered multi-level scalar sorting

The list SHALL let a visitor add, remove, and order at most four sort rules for
minimum work experience, education requirement, first-discovered date, urgent
marker, activity state, or title. Each rule SHALL be set through independent
`排序种类` and `排序方式` controls; the first rule has highest priority and
each later rule resolves a tie from all earlier rules.

#### Scenario: Later sort rules resolve a tie

- **WHEN** the rules are minimum work experience descending followed by
  education requirement descending
- **THEN** higher configured experience appears first and equal experience is
  ordered by higher configured education requirement

#### Scenario: Four sort rules are retained in priority order

- **WHEN** a visitor adds four distinct sort rules and applies the filters
- **THEN** the list request contains the same four ordered `sort` values
- **AND** the browser does not offer a fifth rule

#### Scenario: A sort kind and its direction are changed independently

- **WHEN** a visitor chooses `首次发现日期` as a sort kind and `从新到旧` as
  its sort direction
- **THEN** the next request contains `first_discovered_desc` without requiring
  a combined display option such as `首次发现日期：从新到旧`

#### Scenario: Scalar facts offer their business directions

- **WHEN** a visitor adds a rule for urgent marker or activity state
- **THEN** the directions respectively express `紧急优先／非紧急优先` and
  `仍有效优先／已确认下线优先`

#### Scenario: A multi-value field is considered for sorting

- **WHEN** a visitor uses a filter for job family, technology keyword,
  project, or location
- **THEN** that field is not offered as a sort kind

#### Scenario: A rule is reordered

- **WHEN** a visitor moves education requirement before work experience
- **THEN** education requirement becomes the highest-priority sort rule on the
  next applied list request

#### Scenario: A sort field is duplicated

- **WHEN** a visitor tries to add a sort field that is already selected
- **THEN** the browser does not create a duplicate rule

### Requirement: Missing requirement values in multi-level sorting

For every selected education or experience sort rule, JDs without a configured
and available value MUST appear after JDs with a value, regardless of that
rule's direction. Deterministic title and opaque JD identity ordering MUST
resolve any remaining tie. Date, urgent marker, activity state, and title are
always available from the current Serving inventory.

#### Scenario: Descending experience includes unavailable values

- **WHEN** experience descending is the first sort rule
- **THEN** configured experience values appear greatest to least before JDs
  without configured experience, including when later rules differ

#### Scenario: Both selected values tie

- **WHEN** two JDs have equal values for every selected sort rule
- **THEN** their order is stable across repeated requests and page boundaries

### Requirement: Backward-compatible ordered list query

The list endpoint SHALL accept a repeated ordered `sort` query value, a
repeated `job_family` query value, and an optional `minimum_experience_year`
integer from zero through nine. It MUST reject an unsupported or duplicate
sort field, a fifth sort rule, and an out-of-range threshold with a safe client
error. A request
with one existing sort value or legacy primary/other-family parameters MUST
retain its current meaning during this release.

#### Scenario: Ordered sort values are received

- **WHEN** a client sends `sort=experience_desc&sort=education_desc`
- **THEN** the endpoint treats experience as the first rule and education as
  the second rule

#### Scenario: Threshold query is received

- **WHEN** a client sends `minimum_experience_year=3`
- **THEN** the endpoint compares the current configured formal experience
  minimum against three before ordering and pagination

#### Scenario: Legacy shared list URL is received

- **WHEN** a client sends one existing `sort=education_desc` value
- **THEN** the endpoint returns the same single-rule education ordering

#### Scenario: Invalid repeated sort is received

- **WHEN** a client repeats the experience sort field or submits an unknown
  sort value
- **THEN** the endpoint returns its safe invalid-query response without
  executing an unbounded list request

### Requirement: Numbered page navigation

The JD browser SHALL use the existing offset list contract: `page` identifies
the one-based page and `page_size` identifies the bounded number of list items
per page. It SHALL show numbered pages, allow a visitor to choose a supported
page size, and provide a direct page-number jump. It MUST preserve the current
filters and sort-rule order when either value changes, reset to page one when
page size changes, and never send or display a cursor token.

#### Scenario: A numbered page is selected

- **WHEN** a visitor selects page `4`
- **THEN** the next request contains `page=4` and the current `page_size`

#### Scenario: A page size is changed

- **WHEN** a visitor selects a different page size
- **THEN** the next request contains `page=1` and the selected `page_size`

#### Scenario: A direct page jump is used

- **WHEN** a visitor enters a valid page number in the quick jump control
- **THEN** the browser loads that numbered page without using a cursor

### Requirement: Viewport-preserving list refinements

Applying, resetting, or changing query state within the JD-list route SHALL
not force the browser viewport to the top. Returning from a JD detail MUST
continue to restore the browser's saved list position.

#### Scenario: Filters are applied below the page heading

- **WHEN** a visitor applies changed filters while reading lower on the list
- **THEN** the updated result is loaded without an application-initiated jump
  to the page top

#### Scenario: Detail is opened and closed

- **WHEN** a visitor opens a JD detail and uses browser back to return
- **THEN** the prior list query state and saved viewport position are restored
