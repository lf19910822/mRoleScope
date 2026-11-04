# Spec Delta

## Purpose

提供可配置、可读且响应稳定的岗位趋势看板，使用户在同一张图中比较最多八条正式历史曲线，
同时保持日常浏览快速、历史事实诚实且不暴露内部处理细节。

## ADDED Requirements

### Requirement: Trend dashboard configuration

The dashboard SHALL start with the current `Asia/Shanghai` week and no preset
curve. It MUST let users configure up to eight unique curves from structured
technology, job-family, education, project, location, and experience values.

#### Scenario: A user adds a valid curve

- **WHEN** a user chooses a dimension and a compatible structured value
- **THEN** the dashboard SHALL add one independently configurable curve row
- **AND THEN** it SHALL display an unambiguous label for that curve condition

#### Scenario: A dimension changes

- **WHEN** a user changes a curve from one dimension to another
- **THEN** the dashboard SHALL clear incompatible conditions
- **AND THEN** it SHALL show only controls applicable to the new dimension

#### Scenario: The curve limit or a duplicate is reached

- **WHEN** a user tries to add a ninth curve or a condition identical to an
  existing curve
- **THEN** the dashboard SHALL prevent the addition
- **AND THEN** for a duplicate it SHALL direct the user to the existing row

### Requirement: Configuration and range application behavior

The dashboard SHALL require explicit application after a user changes a curve
condition. Selecting a time-range control SHALL take effect immediately and
update all existing curves to that range without resetting an all-history view
solely because one curve changes.

#### Scenario: A curve condition is edited

- **WHEN** a user changes a curve's dimension, value, comparison, or
  job-family inclusion option
- **THEN** the dashboard SHALL not query that changed condition until the user
  applies the configuration

#### Scenario: A natural range is selected

- **WHEN** a user selects this week, month, quarter, year, or all history
- **THEN** the dashboard SHALL immediately request or restore every existing
  curve for that selected range

### Requirement: Independent curve updates are race-safe

The dashboard SHALL load and replace each affected curve independently while
keeping unaffected curves readable. It MUST retain the last valid line on an
individual failure and MUST ignore any older response that no longer matches
the current configuration of that curve.

#### Scenario: One curve is applied

- **WHEN** a user applies a change only to the Go curve
- **THEN** only the Go curve and its configuration row SHALL show loading
- **AND THEN** other curves SHALL remain visible and unchanged

#### Scenario: An earlier response returns late

- **WHEN** a user changes one curve from Go to Python to Rust before prior
  requests finish
- **THEN** the dashboard SHALL render only the Rust result
- **AND THEN** it SHALL not let an earlier Go or Python response overwrite it

#### Scenario: A single curve update fails

- **WHEN** a curve refresh fails after it previously returned data
- **THEN** the dashboard SHALL retain that curve's prior data and show an
  error only on its configuration row

### Requirement: Historical chart reading preserves fact boundaries

The chart SHALL render missing daily values as real line breaks and complete
zero values at zero. Its Y axis MUST always start at zero; smooth rendering
MUST NOT interpolate across missing dates or change underlying integer counts.

#### Scenario: A daily point is missing

- **WHEN** the trend response contains a missing date between two counts
- **THEN** the chart SHALL retain its date position and break the line at that
  date

#### Scenario: A complete day has zero matches

- **WHEN** the trend response contains a count of zero
- **THEN** the chart SHALL render the point at the zero baseline rather than
  treating it as a missing date

### Requirement: Time-range and chart interaction boundaries

The dashboard SHALL offer this week, month, quarter, year, and all history.
The first four MUST display their natural start through today with fixed chart
coordinates. Only all history MAY permit smooth in-chart X/Y span zoom and X
pan; Y MUST remain anchored at zero and node clicks provide only light feedback.

#### Scenario: A user views all history

- **WHEN** a user opens all history
- **THEN** the user MAY zoom the displayed X and Y spans and pan the X span
- **AND THEN** the chart SHALL retain its loaded series without re-querying for
  each zoom or pan gesture

#### Scenario: A user views a preset range

- **WHEN** a user views this week, month, quarter, or year
- **THEN** the chart SHALL not expose zoom or pan that changes the fixed view

#### Scenario: A node is clicked

- **WHEN** a user clicks a chart node
- **THEN** the dashboard SHALL provide only a light in-chart click animation
- **AND THEN** it SHALL not open an information layer, navigate to jobs, or
  create a snapshot or filtering payload

### Requirement: Legend and current formal-release consistency

The dashboard SHALL allow a legend click to toggle a curve and hover to focus
one curve. When a newly published complete formal release affects displayed
derived curves, it MUST replace those affected curves together rather than
rendering a mixed-release comparison.

#### Scenario: A legend item is toggled

- **WHEN** a user clicks a curve in the legend
- **THEN** the dashboard SHALL hide or show that curve without changing its
  configured condition or source data

#### Scenario: A complete formal release supersedes displayed curves

- **WHEN** a refresh detects a newer complete formal release affecting several
  displayed derived curves
- **THEN** the dashboard SHALL update the affected curves in the background
- **AND THEN** it SHALL replace them together after all new results arrive

### Requirement: Short-lived result caching and restored state

The dashboard SHALL preserve a user's configuration and legend state across
navigation and browser reload. It MUST cache only aggregate curve results for
ten minutes, reuse them for ordinary returns, and re-request on browser reload,
explicit refresh, or configuration reapplication.

#### Scenario: A user returns within the cache window

- **WHEN** a user returns from another application page within ten minutes
- **THEN** the dashboard SHALL restore the saved configuration and cached
  aggregate curve results without issuing a curve request

#### Scenario: A user explicitly refreshes

- **WHEN** a user reloads the browser, presses the dashboard refresh control,
  or reapplies a curve configuration
- **THEN** the dashboard SHALL request fresh results
- **AND THEN** it SHALL keep the existing chart stable until replacements arrive

#### Scenario: A mounted chart reaches cache expiry

- **WHEN** the ten-minute cache window expires while the dashboard remains open
- **THEN** the dashboard SHALL keep the existing chart static
- **AND THEN** it SHALL not automatically refresh or show a cache-status notice

### Requirement: Responsive dashboard layout and empty state

The desktop dashboard SHALL retain the application navigation, a main chart,
and a fixed right-side curve configuration area. On narrow screens the chart
MUST use available width and configuration MUST be reachable through a dedicated
panel; no-curve state MUST remain a concise prompt without sample data.

#### Scenario: A desktop user reads trends

- **WHEN** the viewport has desktop width
- **THEN** the chart SHALL be the main reading surface
- **AND THEN** the curve configuration area SHALL remain available beside it

#### Scenario: A narrow-screen user configures curves

- **WHEN** the viewport is narrow
- **THEN** the chart SHALL occupy the content width
- **AND THEN** the user SHALL be able to open the curve configuration panel

#### Scenario: No curves are configured

- **WHEN** the dashboard has no configured curves
- **THEN** it SHALL show a concise invitation to add a curve
- **AND THEN** it SHALL not render mock data or a preset all-jobs line

