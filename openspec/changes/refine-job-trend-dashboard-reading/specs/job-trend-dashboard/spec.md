# Spec Delta

## ADDED Requirements

### Requirement: Time-range and chart interaction boundaries

The dashboard SHALL offer this week, month, quarter, year, and all history.
The first four MUST display their natural start through today with fixed chart
coordinates. Only all history MAY permit smooth in-chart X/Y span zoom and X
pan; Y MUST remain anchored at zero. It SHALL provide concise in-context
guidance for those gestures without opening a separate information layer.

#### Scenario: A user views all history

- **WHEN** a user opens all history with a rendered curve
- **THEN** the user SHALL see the available zoom and pan gestures near the
  chart controls
- **AND THEN** zooming or panning SHALL retain loaded series without making a
  data request

#### Scenario: A chart point is clicked

- **WHEN** a user clicks a chart point
- **THEN** the dashboard SHALL not announce a false focus state, navigate to
  jobs, open an information layer, or create a snapshot/filtering payload

### Requirement: Stable legend and curve identity

The dashboard SHALL allow a legend click to toggle a curve and hover to focus
one curve. Every configured curve SHALL retain a stable observation colour in
its configuration row, plotted series, and legend even when another row is
pending, hidden, or removed. The legend MAY shorten its display text but the
configuration row MUST retain an unambiguous full condition label.

#### Scenario: A sibling curve has no rendered result

- **WHEN** one configured curve is still a draft or has failed to render
- **THEN** an already rendered sibling SHALL keep its observation colour
- **AND THEN** its concise legend entry SHALL still identify the same curve

### Requirement: Responsive dashboard layout and empty state

The desktop dashboard SHALL retain the application navigation, a main chart,
and a fixed right-side curve configuration area. On narrow screens the chart
MUST use available width and configuration MUST be reachable through a
dedicated panel; no-curve state MUST remain a concise prompt without sample
data. With no configured curve, the dashboard SHALL expose exactly one action
that creates the first draft condition; it SHALL not duplicate that action in
the rail or mobile trigger.

#### Scenario: No curves are configured

- **WHEN** the dashboard has no configured curves
- **THEN** it SHALL show one concise invitation to configure the first curve
- **AND THEN** it SHALL create a first draft when invoked and open the narrow
  configuration panel when applicable

- **AND THEN** it SHALL not render mock data or a preset all-jobs line

### Requirement: Curve summaries preserve historical fact boundaries

For an applied curve, the configuration row SHALL show the latest numeric
observation and, when another prior numeric observation exists, its signed
difference from that prior observation. It SHALL skip unavailable (`null`)
days, preserve zero as numeric, and show no fabricated summary when a series
contains no numeric observation.

#### Scenario: Latest point follows a gap

- **WHEN** a curve has points `2`, `null`, then `5`
- **THEN** its row SHALL display the latest observation as `5`
- **AND THEN** it SHALL display a positive difference of `3` from the prior
  available observation

#### Scenario: A curve has only unavailable days

- **WHEN** every point in an applied curve is unavailable
- **THEN** its row SHALL not display a count or difference as if data existed
