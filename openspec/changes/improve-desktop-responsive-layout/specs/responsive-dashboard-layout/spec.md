# Responsive Dashboard Layout

## ADDED Requirements

### Requirement: Desktop layout preserves a readable trend workspace

The trend dashboard MUST choose a desktop layout that preserves a usable chart and event workspace instead of allowing the fixed configuration rail to reduce the main content to an unreadable width.

#### Scenario: Wide desktop keeps the established split layout

- **WHEN** the viewport is at least 1440 CSS pixels wide
- **THEN** the trend dashboard MUST keep the chart workspace and the persistent right configuration rail visible together
- **AND** the chart workspace MUST remain at least 640 CSS pixels wide

#### Scenario: Standard desktop reduces density without collapsing the workspace

- **WHEN** the viewport is between 1180 and 1439 CSS pixels wide
- **THEN** the trend dashboard MUST keep the two-column desktop layout
- **AND** the configuration rail and surrounding spacing MUST use the standard desktop density
- **AND** the chart workspace MUST remain at least 520 CSS pixels wide

#### Scenario: Compact desktop releases the configuration rail space

- **WHEN** the viewport is between 761 and 1179 CSS pixels wide
- **THEN** the trend dashboard MUST NOT reserve the full configuration rail width in the main workspace by default
- **AND** the chart and event table MUST use the released horizontal space
- **AND** the user MUST have an always-visible control to open curve configuration

### Requirement: Compact desktop configuration remains fully operable

On compact desktop, the curve configuration MUST open as a bounded overlay or drawer that does not make the underlying page unusable and MUST preserve all existing curve editing, scrolling, deletion, and batch-apply actions.

#### Scenario: User opens configuration on compact desktop

- **WHEN** the user activates the compact desktop configuration control
- **THEN** the configuration panel MUST become visible with an accessible close control
- **AND** the panel MUST have a readable width without causing horizontal page overflow
- **AND** the underlying chart MUST remain available after the panel is closed

#### Scenario: Configuration list is longer than the viewport

- **WHEN** configured curves exceed the available vertical space
- **THEN** only the curve list MUST scroll inside the configuration panel
- **AND** the panel footer actions MUST remain reachable

#### Scenario: Desktop breakpoint changes while configuration is open

- **WHEN** the viewport crosses between compact desktop and standard desktop widths
- **THEN** the configuration panel MUST settle into the layout appropriate to the new width
- **AND** no horizontal page overflow or inaccessible configuration state MUST remain

### Requirement: Responsive typography and spacing preserve hierarchy

The trend dashboard MUST adjust spacing and bounded component dimensions by breakpoint while keeping body text, data values, labels, controls, and keyboard focus readable.

#### Scenario: Standard desktop uses compact but readable spacing

- **WHEN** the viewport is between 1180 and 1439 CSS pixels wide
- **THEN** card padding, chart height, event table padding, and configuration card spacing MAY reduce from the wide layout
- **BUT** the primary headings, event counts, rates, form controls, and focus indicators MUST remain readable and usable

#### Scenario: Trend detail overlays fit the available workspace

- **WHEN** a trend drilldown or job detail overlay is opened on any desktop breakpoint
- **THEN** the overlay MUST stay within the viewport and the active trend workspace
- **AND** its content region MUST scroll independently when content exceeds the available height

### Requirement: Responsive verification covers desktop and mobile viewports

The implementation MUST be verifiable through observable viewport behavior at representative wide, standard, compact, and mobile sizes.

#### Scenario: Desktop viewport matrix has no horizontal overflow

- **WHEN** the dashboard is checked at 1440, 1280, 1100, and 900 CSS pixel widths
- **THEN** the document MUST NOT have horizontal overflow
- **AND** the chart workspace, configuration entry, event table, and navigation MUST be reachable

#### Scenario: Mobile layout remains available at 390 CSS pixels

- **WHEN** the dashboard is checked at 390 CSS pixels wide
- **THEN** the mobile navigation and configuration drawer MUST remain usable
- **AND** the event table and trend content MUST remain readable through the existing mobile scrolling behavior
