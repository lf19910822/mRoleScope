# Spec Delta

## ADDED Requirements

### Requirement: Unified calendar date-boundary controls

The JD browser SHALL render every editable first-discovery date boundary with
the shared button-triggered calendar control. It MUST NOT render a native
`input[type="date"]` for these filters. The control SHALL let a visitor select
one calendar day or clear its own selected day.

#### Scenario: Select the start boundary

- **WHEN** a visitor selects 2026-10-02 in `首次发现日期（起）`
- **THEN** its filter value is `2026-10-02`
- **AND** the calendar closes without changing `首次发现日期（止）`

#### Scenario: Clear one boundary

- **WHEN** a visitor clears a selected `首次发现日期（止）`
- **THEN** that boundary is unset
- **AND** the start boundary remains unchanged

### Requirement: Date filter query compatibility

The calendar controls SHALL preserve the existing ISO calendar-date and
query-state contract. Applying a selected boundary MUST serialize it as the
existing `first_discovered_from` or `first_discovered_until` parameter; an
unset boundary MUST omit only its own parameter.

#### Scenario: Apply both boundaries

- **WHEN** a visitor selects 2026-10-01 as the start and 2026-10-31 as the end
  and applies filters
- **THEN** the list URL and Serving request retain
  `first_discovered_from=2026-10-01` and
  `first_discovered_until=2026-10-31`

#### Scenario: Use a narrow viewport

- **WHEN** the browser is 390px wide and a visitor opens either calendar
- **THEN** the trigger remains operable in the normal single-column filter
  flow and the calendar is anchored to that control

### Requirement: Calendar has one legible surface

An open calendar SHALL present one visible calendar card. The positioning
Popover MUST NOT add a second background, padded frame, or shadow around that
card. When a visitor opens the month/year quick selector, its selector panel
MUST be opaque and visually layer above the date grid.

#### Scenario: Open a calendar

- **WHEN** a visitor opens either first-discovery date picker
- **THEN** the calendar appears as one bordered and shadowed Ocean Breeze card
- **AND** no second Popover frame surrounds it

#### Scenario: Open the month/year quick selector

- **WHEN** a visitor activates the calendar month/year heading
- **THEN** the year and month selector is readable on an opaque surface above
  the calendar's date grid
