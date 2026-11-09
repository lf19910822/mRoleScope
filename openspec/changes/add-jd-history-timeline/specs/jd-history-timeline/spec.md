# JD History Timeline

## ADDED Requirements

### Requirement: Daily event timeline

The system SHALL expose a display-safe per-job history timeline derived from
existing published SCD2/Serving facts.

#### Scenario: Final successful observation per day

- **WHEN** multiple runs exist for one Shanghai business day
- **THEN** the timeline uses only the latest successful published run for that
  day
- **AND** failed or incomplete attempts do not replace a successful observation

#### Scenario: Missing business day

- **WHEN** no successful run exists for an intermediate day
- **THEN** no missing-day node or warning is generated
- **AND** a later successful day may still be compared with the nearest earlier
  successful business day

#### Scenario: First observation and latest marker

- **WHEN** a job has one or more successful observations
- **THEN** the first observation is represented as `discovered`
- **AND** the most recent successful business day is represented as `latest`
- **AND** same-day same-version semantics are merged into one node

### Requirement: Lifecycle event semantics

The timeline SHALL distinguish source changes from confirmed visibility
lifecycle changes.

#### Scenario: Visible source change

- **WHEN** visible canonical metadata or normalized body content differs from
  the nearest earlier successful observation
- **THEN** the day receives one `modified` node
- **AND** the node includes dimension/body tags using the existing trend
  classification logic

#### Scenario: Parser-only change

- **WHEN** a parser, dictionary, or normalizer version changes but the source
  metadata/body values do not
- **THEN** no `modified` node is generated

#### Scenario: Confirmed close

- **WHEN** existing presence-period evidence confirms the job is closed
- **THEN** the timeline includes `confirmed_closed`
- **AND** the close-to-reopen interval has no connecting line

#### Scenario: Reopen

- **WHEN** a new visible period begins after a confirmed close
- **THEN** the timeline includes `reopened` even if the JD text is unchanged
- **AND** source changes on that day appear as additional tags on the same node

### Requirement: History summary contract

The history summary SHALL return only safe fields needed to render and route a
node.

#### Scenario: Safe summary

- **WHEN** the history endpoint succeeds
- **THEN** each node contains a Shanghai date, stable type, display tags,
  display summary, and historical-view availability
- **AND** no run ID, job ID, source ID, membership ID, parser version,
  dictionary version, or internal error appears

#### Scenario: Historical snapshot unavailable

- **WHEN** a known node has no safely recoverable JD snapshot
- **THEN** the summary retains the node with historical viewing disabled
- **AND** the detail endpoint returns a stable business-unavailable response
- **AND** the service never substitutes the latest JD for that node

### Requirement: Adjacent node detail

The node detail endpoint SHALL reuse the existing safe JD detail and comparison
schemas while selecting versions according to node type.

#### Scenario: Modified node diff

- **WHEN** a `modified` node is requested
- **THEN** the response compares that day with the nearest earlier successful
  business day
- **AND** dimension changes and the three body-section fragments are returned
  in the existing Git-style structure

#### Scenario: Reopened node diff

- **WHEN** a `reopened` node with source changes is requested
- **THEN** the response compares the last pre-close version with the reopened
  version
- **AND** the response identifies the node as reopened without exposing
  internal lifecycle IDs

#### Scenario: Single-version node

- **WHEN** a discovered, confirmed-closed, or latest node is requested
- **THEN** the response returns that node's single JD snapshot
- **AND** no unrelated latest-JD comparison is added

### Requirement: Resilient detail UI

The job detail page SHALL render the history module independently from the main
JD sections and reuse the trend dashboard interaction patterns.

#### Scenario: Loading and empty states

- **WHEN** the history summary is loading or has no nodes
- **THEN** the page keeps the JD body usable
- **AND** loading and empty states are local to the history module

#### Scenario: Cached retry

- **WHEN** a refresh fails and cached history exists
- **THEN** cached history remains visible
- **AND** retry affects only the history request

#### Scenario: Responsive and keyboard interaction

- **WHEN** the page is rendered on desktop or at 390px width
- **THEN** all event nodes remain available through zoom/pan or horizontal
  interaction
- **AND** nodes can be focused and opened with Enter/Space
- **AND** detail close supports Escape and returns focus to the node
