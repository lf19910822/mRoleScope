# Spec Delta

## Purpose

在现有趋势看板折线下方提供与当前曲线同步的事件统计阅读体验，让用户清楚比较三个实际周期，同时保持折线优先、既有日期节点下钻不变，并让未支持维度、加载、失败、零值和不可比状态不互相混淆。

## ADDED Requirements

### Requirement: The dashboard renders one event table for one selected curve

The dashboard SHALL render at most one event-statistics table at a time. It SHALL default to the first applied curve, keep the table selection separate from curve visibility/configuration, and hide the table when no curve is applied.

#### Scenario: Multiple curves are applied
- **WHEN** the user applies multiple curves
- **THEN** the table SHALL default to the first applied curve
- **AND THEN** the chart MAY render all applied curves while the table renders only the selected curve

#### Scenario: No curve is applied
- **WHEN** the dashboard has no applied curve
- **THEN** the event table SHALL not be rendered
- **AND THEN** the existing no-curve state SHALL remain unchanged

#### Scenario: Selected curve is removed
- **WHEN** the table's selected curve is removed
- **THEN** the table SHALL switch to the first remaining applied curve
- **AND THEN** it SHALL not display the removed curve's statistics

### Requirement: Supported dimensions and development state are explicit

The event table SHALL request statistics only for technology, job family, project, and location curves. For education and experience curves it SHALL display `开发中` without empty metrics, zeroes, skeleton loading, event drill-down, or an event-statistics request.

#### Scenario: A supported curve is selected
- **WHEN** the selected curve is technology, job family, project, or location
- **THEN** the dashboard SHALL load and display its four event metrics
- **AND THEN** it SHALL preserve the curve's complete condition, including job-family secondary matching

#### Scenario: An education or experience curve is selected
- **WHEN** the selected curve is education or experience
- **THEN** the table area SHALL display `开发中`
- **AND THEN** it SHALL not display metric rows, zeroes, loading skeletons, or event drill-down actions
- **AND THEN** its line and existing date-node drill-down SHALL remain available

#### Scenario: Arrow navigation leaves a development-state curve
- **WHEN** the user uses the table arrows to select a supported curve
- **THEN** the development message SHALL be replaced by that curve's event table state

### Requirement: The table presents actual periods and comparison values

The table SHALL show the four rows `进入曲线`, `曲线内修改`, `离开曲线`, and `重新上线`. It SHALL show actual date ranges for current, prior, and year-over-year periods, plus comparison counts and rates.

#### Scenario: A current-period table is displayed
- **WHEN** a supported curve has event results
- **THEN** the table SHALL show columns for current period, 环比, and 同比
- **AND THEN** each comparison cell SHALL show its comparison count and rate, such as `7（+28.6%）`
- **AND THEN** the table SHALL expose the actual three date ranges, not only relative labels

#### Scenario: A natural period is unfinished
- **WHEN** the current month or quarter ends after the latest final successful observation
- **THEN** the table SHALL identify the actual cutoff date with `截至 YYYY-MM-DD`
- **AND THEN** the comparison windows SHALL use the same elapsed length

#### Scenario: All history is selected
- **WHEN** the chart range is all history
- **THEN** the event-table area SHALL display `请选择具体时间范围`
- **AND THEN** the chart, zoom, pan, and existing date-node drill-down SHALL remain available

### Requirement: Counts and rates have honest visual states

The dashboard SHALL preserve legal zeroes, render a zero denominator rate as `-`, render a positive-to-zero change as `-100%`, and distinguish unavailable formal interpretation from an event count of zero.

#### Scenario: A metric has no events
- **WHEN** the period is computable and a metric has no event jobs
- **THEN** the metric count SHALL display `0`
- **AND THEN** its zero count SHALL remain clickable for an empty event drill-down

#### Scenario: A comparison denominator is zero
- **WHEN** the comparison count is `0`
- **THEN** the cell SHALL show `0（-）`
- **AND THEN** it SHALL not show infinity or another special numeric value

#### Scenario: Current count is zero and comparison is positive
- **WHEN** the current count is `0` and a comparison count is positive
- **THEN** the cell SHALL show the comparison count and `-100%`

#### Scenario: Formal interpretation is unavailable
- **WHEN** the formal dimension cannot produce a valid result for the requested calculation
- **THEN** the affected cell SHALL show `暂无可比数据`
- **AND THEN** it SHALL not show a fabricated zero or open a drill-down

### Requirement: Table selection interactions are explicit and non-overlapping

The dashboard SHALL let users select the table curve by clicking the curve itself, its curve card, or the table's left/right arrows. Clicking a concrete chart data point SHALL open only the existing date-node drill-down and SHALL not change the table selection.

#### Scenario: Curve or curve card is clicked
- **WHEN** the user clicks a plotted curve body or its curve card
- **THEN** the table SHALL switch to that curve
- **AND THEN** all three selection methods SHALL update one shared selected-curve state

#### Scenario: A chart point is clicked
- **WHEN** the user clicks a concrete daily data point
- **THEN** the existing date-node drill-down SHALL open
- **AND THEN** the table's selected curve and statistics SHALL remain unchanged

#### Scenario: The drill-down is closed
- **WHEN** the user closes the date-node drill-down
- **THEN** the dashboard SHALL restore the same table selection and table state that existed before opening it

#### Scenario: Arrow reaches an end
- **WHEN** the user presses a table arrow at the first or last applied curve
- **THEN** the selection SHALL remain bounded to applied curves
- **AND THEN** the arrow SHALL not create a new curve or change curve configuration

### Requirement: Event statistics load asynchronously and safely

The dashboard SHALL load the line series before or independently of the event table, request statistics only for the selected supported curve, and isolate event requests from line requests. A stale event response MUST NOT replace the latest selected-curve result.

#### Scenario: Line loads before table
- **WHEN** a range or curve is applied
- **THEN** the line series and existing trend interactions SHALL become usable without waiting for event statistics
- **AND THEN** the event table MAY remain loading independently

#### Scenario: Selected curve changes during a request
- **WHEN** the user changes the table selection before an event request completes
- **THEN** the old request SHALL be ignored or cancelled
- **AND THEN** its response SHALL not overwrite the last selected curve

#### Scenario: Event request fails
- **WHEN** the event-statistics request fails
- **THEN** only the table area SHALL show `统计暂不可用` and a retry action
- **AND THEN** retry SHALL re-request statistics only
- **AND THEN** the line, curve controls, date-node drill-down, and job details SHALL remain usable

#### Scenario: A curve has no event cache
- **WHEN** the user switches to a curve without a valid event result cache
- **THEN** the old table numbers SHALL be cleared
- **AND THEN** the table SHALL show the new curve's loading state rather than old numbers

### Requirement: Event and line caches are isolated and bounded

The dashboard SHALL keep event-statistics cache entries separate from line-series entries. An event cache key SHALL distinguish curve condition, current period, prior period, year-over-year period, event semantics, and the opaque formal-data identity.

#### Scenario: A cached event result matches
- **WHEN** the selected curve and all period/semantics/formal-data inputs match a valid event cache entry
- **THEN** the dashboard MAY restore that event table without a duplicate event request
- **AND THEN** it SHALL not use a line cache entry as an event result

#### Scenario: A cache input changes
- **WHEN** the curve condition, period, effective cutoff, event semantics, or formal-data identity changes
- **THEN** the old event result SHALL not be reused
- **AND THEN** a new event request SHALL be made when the curve is supported

#### Scenario: User explicitly refreshes
- **WHEN** the user explicitly refreshes the dashboard
- **THEN** the event cache SHALL be bypassed or invalidated together with the applicable line cache rules
- **AND THEN** the event table SHALL reflect the latest available trusted observation

### Requirement: Event table styles and failure copy fit the existing dashboard

The event table SHALL use the existing trend dashboard visual hierarchy and responsive layout. Its status copy SHALL be concise and SHALL not expose implementation, audit, repair, parser, dictionary, source, or internal error details.

#### Scenario: Table is loading
- **WHEN** event statistics are loading for the selected supported curve
- **THEN** the table SHALL retain its title, curve context, and period labels
- **AND THEN** it SHALL show a lightweight skeleton without old-curve numbers

#### Scenario: Table is unavailable
- **WHEN** the event request fails
- **THEN** the table SHALL show only the user-facing unavailable message and retry action
- **AND THEN** it SHALL not show endpoint names, request IDs, or backend error text

#### Scenario: Narrow viewport is used
- **WHEN** the dashboard is viewed at the existing 390px acceptance width
- **THEN** the event table, arrows, period labels, status states, and drill-down trigger SHALL remain reachable and readable
- **AND THEN** it SHALL not break the existing responsive chart/configuration composition
