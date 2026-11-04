# Spec Delta

## Purpose

提供只读、可复查的历史岗位趋势数据，使客户端能按正式维度读取单条曲线，而不重建 SCD2
快照或接触 JD 正文、解析与审计内部事实。

## ADDED Requirements

### Requirement: Daily snapshot selection

The system SHALL calculate every trend point from the last successful published
run for its observation scope and `Asia/Shanghai` calendar date. It MUST count
distinct JDs from that run only and preserve the activity state observed then.

#### Scenario: A day has multiple successful publications

- **WHEN** one observation scope has more than one successful publication on a
  single `Asia/Shanghai` date
- **THEN** the trend query SHALL use only the last successful publication of
  that date
- **AND THEN** it SHALL not sum or duplicate memberships from earlier runs

#### Scenario: A later closure is observed

- **WHEN** a JD was effective in a historical selected run and is confirmed
  closed in a later selected run
- **THEN** the historical point SHALL retain the earlier effective membership
- **AND THEN** the later point SHALL use the later observed state

### Requirement: Daily series distinguishes zero from unavailable history

The system SHALL return an ordered calendar-day series through today for the
requested natural range. It MUST return `0` only for a complete selected
snapshot with no matching JD, and MUST return a missing point when the date has
no trustworthy selected snapshot or cannot be fully interpreted.

#### Scenario: A complete day has no matching JD

- **WHEN** a selected snapshot is complete and no distinct JD satisfies a
  curve condition
- **THEN** the response SHALL contain that date with a count of `0`

#### Scenario: A historical day is unavailable

- **WHEN** the selected snapshot, required source version, or required formal
  interpretation is unavailable for a date
- **THEN** the response SHALL contain that date as missing
- **AND THEN** it SHALL not substitute a current JD version, partial count, or
  artificial zero

#### Scenario: Today's run is not yet available

- **WHEN** the requested range includes today and no trustworthy selected run
  has been published for today
- **THEN** the response SHALL include today as a missing point

### Requirement: One normalized condition defines one curve

The system SHALL accept one validated curve condition per query for technology
identity, job family, education threshold, project, location, or minimum work
experience. It MUST apply the dimension-specific comparison and job-family
secondary-family option exactly as configured.

#### Scenario: A technology identity is queried

- **WHEN** a client requests a canonical technology identity
- **THEN** the series SHALL count a JD at most once per day when its formal
  technology result contains that identity
- **AND THEN** approved aliases and case variants of that identity SHALL not
  create separate curves

#### Scenario: A job family includes secondary matches

- **WHEN** a client requests one job family with the secondary-family option
  enabled
- **THEN** the series SHALL include a JD whose primary or secondary family
  matches the requested family
- **AND THEN** it SHALL count that JD only once in that curve on that day

#### Scenario: Education or experience is compared

- **WHEN** a client requests an education or minimum-experience condition with
  `>`, `<`, `>=`, `<=`, or `==`
- **THEN** the system SHALL evaluate the formal minimum requirement using that
  operator
- **AND THEN** JDs with an unconfigured requirement SHALL not match

#### Scenario: A multi-location JD matches a location curve

- **WHEN** a JD has multiple structured locations including the requested one
- **THEN** it SHALL count once in that location's curve
- **AND THEN** it MAY independently count in another location curve

### Requirement: Trend interpretation uses a complete current formal release

The system SHALL evaluate each formal dimension using the latest published
release that fully covers every recoverable historical body version required by
the queried series. It MUST NOT expose or mix a partially completed release.

#### Scenario: A newer interpretation release is incomplete

- **WHEN** a newer dictionary or deterministic parsing release has pending,
  failed, or uncovered recoverable historical body versions
- **THEN** trend queries SHALL continue to use the prior complete published
  release
- **AND THEN** they SHALL not expose a mixture of old and new results

#### Scenario: A complete release is published

- **WHEN** a newer complete formal release is published
- **THEN** a subsequent trend query SHALL evaluate all recoverable historical
  dates under that release
- **AND THEN** it SHALL preserve the original source membership and body facts

### Requirement: Trend responses are safe and bounded

The system SHALL return only validated curve metadata, dates, nullable counts,
and a safe opaque freshness identity. It MUST reject invalid dimensions, values,
or operators with a safe client error and MUST NOT expose JD identifiers,
正文、source versions, parser or dictionary versions, coverage, audit state, or
credentials.

#### Scenario: A curve request is invalid

- **WHEN** a client submits an unsupported dimension, unavailable value, or
  incompatible comparison option
- **THEN** the system SHALL return a safe client error
- **AND THEN** it SHALL not return a partial or unbounded series

#### Scenario: A trend response is returned

- **WHEN** a valid curve query completes
- **THEN** its public response SHALL omit internal provenance, parser,
  dictionary, audit, source-content, and credential fields

