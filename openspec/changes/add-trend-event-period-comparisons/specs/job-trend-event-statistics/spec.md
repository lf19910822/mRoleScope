# Spec Delta

## Purpose

为趋势曲线提供按周期计算的岗位状态转移统计，使用户能够区分岗位进入、曲线内修改、离开和重新上线，并以相同长度的环比和同比周期比较这些事件，而不为每个筛选条件建立物化快照。

## ADDED Requirements

### Requirement: Event statistics use a bounded independent query

The service SHALL provide a separate event-statistics read capability that accepts one complete validated curve condition and a supported current period. It SHALL return current, prior-period, and year-over-year event counts without changing the existing line-series response.

#### Scenario: Request statistics for a supported curve
- **WHEN** a client requests a supported technology, job-family, project, or location curve for a valid week, month, quarter, year, or custom period
- **THEN** the service SHALL return the four event metrics for the current period and their corresponding equal-length comparison periods
- **AND THEN** the response SHALL identify the actual dates used for all three periods

#### Scenario: Request statistics for all history
- **WHEN** a client requests event statistics for the all-history range
- **THEN** the service SHALL return a safe indication that a concrete period is required
- **AND THEN** it SHALL not invent a current, prior, or year-over-year period

#### Scenario: Request an invalid condition or period
- **WHEN** a client submits an unsupported dimension, incompatible operator, invalid value, malformed custom dates, or a future end date
- **THEN** the service SHALL return a safe client error
- **AND THEN** it SHALL not return partial event statistics

### Requirement: Periods use final successful observations and equal-length comparisons

The service SHALL resolve natural dates in `Asia/Shanghai` and use the most recent final successful observation as the effective current cutoff. An unfinished current day SHALL not create events or count as a complete observation.

#### Scenario: Current month is still in progress
- **WHEN** the latest final successful observation is `2026-11-11` while the calendar date is `2026-11-12`
- **THEN** the current month period SHALL be `2026-11-01` through `2026-11-11`
- **AND THEN** the prior comparison SHALL use the equal-length window `2026-10-01` through `2026-10-11`
- **AND THEN** the year-over-year comparison SHALL use `2025-11-01` through `2025-11-11`

#### Scenario: Current month has a final observation through the current date
- **WHEN** the latest final successful observation is `2026-11-12`
- **THEN** the current month period SHALL be `2026-11-01` through `2026-11-12`
- **AND THEN** the comparison windows SHALL use the same twelve-day length

#### Scenario: Custom period is inclusive
- **WHEN** a client requests a custom period from `2026-10-01` through `2026-10-15`
- **THEN** both endpoints SHALL be included in the current period
- **AND THEN** the prior and year-over-year periods SHALL each contain fifteen calendar dates with the same relative positions

### Requirement: Historical baseline does not block event statistics

The service SHALL treat the first trusted historical observation as a legal statistical baseline. It SHALL count matching effective jobs on that baseline date as entering the curve and SHALL treat dates before the baseline as having zero events rather than as unavailable history.

#### Scenario: Baseline date creates curve entries
- **WHEN** `2026-10-22` is the first trusted observation and an effective job matches `technology = C++` on that date
- **THEN** that job SHALL count as `进入曲线` on `2026-10-22`
- **AND THEN** the reason SHALL be available to event drill-down as a historical-baseline entry

#### Scenario: Comparison period is before the baseline
- **WHEN** the current period is `2026-10-22` through `2026-10-31` and the prior period is `2026-10-12` through `2026-10-21`
- **THEN** all four event counts for the prior period SHALL be legal `0`
- **AND THEN** the prior period SHALL not be reported as an error or unavailable comparison

#### Scenario: A period crosses the baseline
- **WHEN** a period contains dates before and after `2026-10-22`
- **THEN** only baseline-day entries and later trusted transitions SHALL contribute events
- **AND THEN** the pre-baseline portion SHALL contribute legal zeroes without making the entire period unavailable

### Requirement: Missing dates are silently skipped

The service SHALL omit missing dates from event computation without filling them with zeroes, creating events, or making the whole period unavailable. A transition between trusted observations SHALL be assigned to the later trusted observation date.

#### Scenario: A small gap occurs inside a period
- **WHEN** trusted observations exist on `2026-10-22` and `2026-10-24` but `2026-10-23` is missing
- **THEN** the service SHALL compute the period from the trusted observations that exist
- **AND THEN** it SHALL not create a `2026-10-23` event, gap warning, or unavailable-period state

#### Scenario: A transition is observed after a gap
- **WHEN** a job changes curve membership between the last trusted observation before a gap and the next trusted observation after it
- **THEN** the event SHALL be assigned to the later trusted observation date
- **AND THEN** it SHALL not claim that the event happened on a missing date

#### Scenario: An entire comparison period has no trusted observations
- **WHEN** every date in a comparison period is missing
- **THEN** all four event counts for that period SHALL be `0`
- **AND THEN** the period SHALL not be reported as unavailable or require manual repair before other periods can be viewed

#### Scenario: Manual repair fills a missing date
- **WHEN** an authorized history repair publishes a trusted observation for a previously missing date
- **THEN** the repaired observation SHALL participate exactly like a normal synchronized observation
- **AND THEN** the service SHALL not expose repair provenance, a repair warning, or a special error

### Requirement: Event classification reuses existing history semantics

The service SHALL classify each distinct job within a period using the existing historical modification, effective-state, confirmed-closure, and reactivation semantics. It SHALL not infer modification from a new event-specific rule.

#### Scenario: A new or newly matching job enters
- **WHEN** a job is first observed as effective and matching, or changes from non-matching to matching after a JD modification
- **THEN** it SHALL count once as `进入曲线`
- **AND THEN** a modification that newly matches SHALL not also count as `曲线内修改`

#### Scenario: A matching job changes within the curve
- **WHEN** a matching job has a JD modification and remains matching after the change
- **THEN** it SHALL count once as `曲线内修改` for the period

#### Scenario: A job leaves the curve
- **WHEN** a matching job changes and becomes non-matching, or changes from effective to confirmed closed
- **THEN** it SHALL count once as `离开曲线`
- **AND THEN** drill-down SHALL distinguish `修改后移出条件` from `确认下线`

#### Scenario: A closed job returns
- **WHEN** a job changes from confirmed closed to effective at a trusted observation
- **THEN** it SHALL count once as `重新上线`
- **AND THEN** it SHALL not count as `进入曲线`, regardless of its curve membership

#### Scenario: A job has repeated events
- **WHEN** the same job has multiple changes of one event type within a period
- **THEN** that event metric SHALL count the job only once
- **AND THEN** the same job MAY appear once in multiple different event metrics when it genuinely has different event types in the same period

### Requirement: Curve membership uses the published trend semantics

The service SHALL use the same historical formal dimension results and condition semantics as the trend line. It SHALL not replace historical interpretation with the current job result.

#### Scenario: Technology curve uses a formal identity
- **WHEN** a historical formal technology result contains the requested canonical technology identity
- **THEN** the job SHALL match that technology curve at that historical observation
- **AND THEN** aliases and case variants SHALL follow the existing canonical identity behavior

#### Scenario: Job-family secondary matching differs
- **WHEN** two curves use the same job family but different `include secondary family` settings
- **THEN** the service SHALL evaluate them as distinct conditions
- **AND THEN** the event counts SHALL follow the selected setting exactly

#### Scenario: Project or location curve matches a historical observation
- **WHEN** a historical observation contains the requested formal project key or standardized location
- **THEN** the job SHALL match that curve once for that observation
- **AND THEN** multi-valued locations or projects SHALL not multiply the job count

#### Scenario: A formal release changes
- **WHEN** a newer complete formal interpretation release becomes published
- **THEN** subsequent line and event reads SHALL use that same release consistently
- **AND THEN** the service SHALL not mix releases or expose release metadata to the client

### Requirement: Event counts and comparison rates have explicit numeric semantics

The service SHALL return counts as distinct jobs per event type and compute each comparison rate as `(current - comparison) / comparison`. It SHALL preserve legal zeroes and represent a zero denominator without an infinite rate.

#### Scenario: Comparison has a nonzero denominator
- **WHEN** the current event count is `9` and the comparison count is `7`
- **THEN** the response SHALL contain comparison count `7` and rate `28.6%` after one-decimal rounding

#### Scenario: Comparison denominator is zero
- **WHEN** the current event count is `9` and the comparison count is `0`
- **THEN** the response SHALL contain comparison count `0` and no numeric rate
- **AND THEN** the public presentation SHALL render that rate as `-`

#### Scenario: Current count is zero and comparison is positive
- **WHEN** the current event count is `0` and the comparison count is greater than `0`
- **THEN** the response SHALL calculate and present `-100%`

#### Scenario: Formal dimension results are unavailable
- **WHEN** the requested formal dimension cannot produce a valid result under the published release
- **THEN** the affected statistic SHALL be marked as having no comparable data
- **AND THEN** the client SHALL not treat it as a legal zero or open its drill-down

### Requirement: Public event responses are safe and bounded

The public response SHALL contain only curve labels, actual period dates, four event metrics, counts, rates, supported-state information, and an opaque freshness identity if needed for cache validation. It MUST omit job IDs, source versions, parser or dictionary versions, audit state, internal errors, credentials, and repair provenance.

#### Scenario: A valid event response is returned
- **WHEN** a valid supported request completes
- **THEN** the response SHALL include all four metric rows even when a row is zero
- **AND THEN** it SHALL contain no internal provenance or JD content

#### Scenario: Statistics are computed on demand
- **WHEN** a client requests a new curve condition or period
- **THEN** the service SHALL compute from existing trusted history on demand
- **AND THEN** it SHALL not require a per-condition snapshot, materialized event table, or precomputed form snapshot
