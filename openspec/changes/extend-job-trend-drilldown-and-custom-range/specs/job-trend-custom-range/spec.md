# Spec Delta

## Purpose

Define a safe, inclusive custom historical date window for trend curves while
retaining the existing final-publication, gap, formal-release, zoom, and cache
semantics.

## ADDED Requirements

### Requirement: Custom range is an inclusive bounded natural-date window

The trend API SHALL accept `range=custom` with `start_date` and `end_date` in
`YYYY-MM-DD`. Both dates SHALL be included, interpreted in `Asia/Shanghai`,
and `end_date` SHALL NOT be later than today's local date. The range SHALL
reject `start_date > end_date`, malformed dates, repeated date parameters, and
an interval outside the retained recoverable history.

#### Scenario: Valid custom window

- **WHEN** a client requests `2026-10-01` through `2026-10-15`
- **THEN** the response contains one point for every natural date in that
  inclusive interval
- **AND THEN** each point uses the same final-successful-publication and
  zero-versus-null rules as `range=all`

#### Scenario: Invalid order or future end

- **WHEN** `start_date` is after `end_date`, or `end_date` is after the local
  current date
- **THEN** the API returns a safe 400 and performs no historical read

### Requirement: Custom ranges use the existing curve semantics

The custom range SHALL evaluate the same validated curve condition and latest
published complete formal interpretation release as named ranges. It SHALL
not reparse bodies, use current membership to fill history, sum stock counts
across days, or turn an unavailable day into zero.

#### Scenario: Missing day inside a custom window

- **WHEN** no trustworthy final published snapshot exists for a date inside
  the selected interval
- **THEN** that date remains present with `count: null`
- **AND THEN** later dates retain their real positions and values

### Requirement: Custom ranges share zoom and cache behavior

The dashboard SHALL allow the same in-chart zoom/pan controls for `custom` as
for `all`. Its ordinary-return cache key SHALL include start date, end date,
curve condition, and the opaque formal-data publication marker. Refresh,
browser reload, reapply, and expired entries SHALL force a new request under
the existing dashboard cache rules.

#### Scenario: Re-enter a custom range

- **WHEN** a reader leaves and returns to custom range within the valid cache
  window without refreshing
- **THEN** the dashboard may reuse the cached curve without requesting again
- **AND THEN** a browser refresh or explicit refresh requests fresh data
