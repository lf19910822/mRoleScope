# Open Custom Trend Range

## ADDED Requirements

### Requirement: Accept any valid inclusive date window
The trend API SHALL accept any valid Asia/Shanghai calendar dates where the
start is not after the end, including dates outside successful publication
history and dates after the current day.

#### Scenario: Range outside coverage
- **WHEN** a custom range includes dates before the first or after the latest
  successful publication
- **THEN** the response preserves those dates and returns null counts for
  unavailable dates

### Requirement: Explain the coverage envelope
Each trend response SHALL include the earliest and latest successful
publication dates as display-safe coverage metadata. It SHALL NOT enumerate
intermediate missing dates.

#### Scenario: Coverage shows only outer dates
- **WHEN** successful publications span 2026-10-01 through 2026-11-07 with an
  intermediate gap
- **THEN** coverage reports those two outer dates and does not list the gap
