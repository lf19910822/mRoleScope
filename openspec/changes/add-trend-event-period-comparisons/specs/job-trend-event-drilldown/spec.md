# Spec Delta

## Purpose

让用户从事件统计数量进入周期内真实发生该事件的岗位列表，并继续使用现有趋势下钻、岗位详情、分页和返回体验，同时保持历史事实、事件原因和公开数据边界清晰。

## ADDED Requirements

### Requirement: Event drill-down accepts a complete event query

The drill-down service SHALL accept one complete validated curve condition, an inclusive period, an event type, page parameters, and an optional title search. It SHALL return only jobs that genuinely satisfy that event within the requested period.

#### Scenario: Open a current-period event count
- **WHEN** a user opens the current-period count for `进入曲线` on `technology = C++` from `2026-11-01` through `2026-11-12`
- **THEN** the service SHALL return jobs that entered that curve during that period
- **AND THEN** it SHALL not return every job merely matching C++ at the period end

#### Scenario: Open a comparison-period event count
- **WHEN** a user opens the prior-period or year-over-year count
- **THEN** the service SHALL query that comparison period's event facts
- **AND THEN** it SHALL not reuse the current-period list

#### Scenario: Open an unsupported or invalid event query
- **WHEN** a client submits an unsupported dimension, invalid period, unsupported event type, or event statistics unavailable for the selected dimension
- **THEN** the service SHALL return a safe client result
- **AND THEN** it SHALL not return a partial or misleading job list

### Requirement: Event drill-down reuses the existing job-reading surface

The dashboard SHALL present event results in the existing trend drill-down container, job list, pagination, detail actions, and return path. It MUST NOT create a second event-specific job-detail structure.

#### Scenario: Open an event list
- **WHEN** a user clicks a legal event count
- **THEN** the existing drill-down container SHALL open with event context
- **AND THEN** the list SHALL preserve existing title search, pagination, historical detail, latest detail, and close behavior

#### Scenario: Event list is opened from a zero count
- **WHEN** a user clicks a legal zero event count
- **THEN** the existing container SHALL open an empty result state
- **AND THEN** it SHALL display “该周期没有符合条件的岗位” without treating zero as an error

#### Scenario: Event cell is not comparable
- **WHEN** a cell is marked “暂无可比数据”
- **THEN** the cell SHALL not open an event drill-down
- **AND THEN** no fabricated empty result shall be shown

### Requirement: Event context is visible without internal provenance

The drill-down SHALL identify the event, curve, period, and result total using user-facing labels. It SHALL not expose run IDs, job IDs, source version IDs, parser versions, dictionary versions, audit state, repair provenance, or internal errors.

#### Scenario: Enter-curve context is shown
- **WHEN** the user opens a `进入曲线` list for `技术关键词 = C++`
- **THEN** the container SHALL identify the event, curve condition, actual period dates, and total such as `共 9 个岗位`
- **AND THEN** it SHALL not display implementation identifiers

#### Scenario: A baseline entry is listed
- **WHEN** a returned entry was counted from the first trusted baseline observation
- **THEN** the list MAY show the user-facing reason `历史基线进入`
- **AND THEN** it SHALL not show why the baseline exists internally or expose collection metadata

### Requirement: Enter and leave reasons are recoverable

The event list SHALL preserve the public reason distinction required by the metric without changing the four-row summary table. Enter events SHALL distinguish baseline entry from modification entry; leave events SHALL distinguish modification removal from confirmed closure.

#### Scenario: Enter reasons are mixed
- **WHEN** a period contains both baseline entries and jobs modified into the curve
- **THEN** the event result SHALL allow those two reasons to be distinguished
- **AND THEN** both remain part of the single `进入曲线` metric

#### Scenario: Leave reasons are mixed
- **WHEN** a period contains both jobs modified out of a curve and jobs confirmed closed while matching
- **THEN** the event result SHALL allow `修改后移出条件` and `确认下线` to be distinguished
- **AND THEN** both remain part of the single `离开曲线` metric

#### Scenario: Reactivation remains independent
- **WHEN** a period contains a confirmed-closed job that returns to effective status
- **THEN** it SHALL appear under `重新上线`
- **AND THEN** it SHALL not be relabeled as `进入曲线`

### Requirement: Event lists are deduplicated and deterministic

The event drill-down SHALL return each job at most once per event type and period, use a deterministic order, and keep the displayed total equal to the unfiltered event count for an unsearched request.

#### Scenario: A job repeats the same event
- **WHEN** one job enters the same curve multiple times in a period
- **THEN** the event list SHALL contain that job once
- **AND THEN** its total SHALL match the period-level distinct-job metric

#### Scenario: Search is applied
- **WHEN** a user searches a title within an event list
- **THEN** the service SHALL filter only the period event result set
- **AND THEN** the displayed total SHALL be the filtered result total

#### Scenario: The event count changed after the table was loaded
- **WHEN** an unsearched drill-down total differs from the table count
- **THEN** the list SHALL remain usable and show the existing lightweight retryable consistency notice
- **AND THEN** it SHALL not pad, truncate, or invent jobs to force equality

### Requirement: Event historical details preserve the observed event context

The drill-down SHALL use the event observation context when opening historical detail and SHALL retain the existing latest-versus-historical availability behavior for changed or closed jobs.

#### Scenario: An event job has changed since the event
- **WHEN** a user opens a job that is no longer identical to its event observation
- **THEN** the list SHALL offer the existing historical and latest detail actions according to current comparison semantics
- **AND THEN** historical detail SHALL use the event observation rather than silently falling back to current content

#### Scenario: An event job is confirmed closed now
- **WHEN** a job from an event period is currently confirmed closed
- **THEN** the list SHALL retain the event item
- **AND THEN** it SHALL offer the permitted historical detail action without removing the historical event
