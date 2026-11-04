# Selected detail-reading layout

## ADDED Requirements

### Requirement: Selected C layout on the normal detail route

The existing JD detail route SHALL render the selected C — 边注档案 — layout
from the already-loaded public detail response in every build. The Serving
request SHALL remain unchanged.

#### Scenario: A reader opens a real JD

- **WHEN** a reader opens a current detail route
- **THEN** the selected C structure renders the loaded role facts, source body,
  and safe technology highlights
- **AND THEN** it does not expose internal parser, source, audit, or model
  metadata

### Requirement: C reading structure at wide and narrow widths

At wide widths, the selected layout SHALL show a margin index for actual
visible body regions beside source-faithful editorial rows. At narrow widths,
that index SHALL become the inline horizontal section list from the accepted
preview. The layout SHALL retain the preview's formal facts and complete public
metadata tag collections.

#### Scenario: A reader changes viewport width

- **WHEN** the selected detail layout is displayed at desktop or 390px width
- **THEN** its C reading structure remains usable without hiding public tags
- **AND THEN** source rows wrap without horizontal overflow

### Requirement: Margin index does not consume page-navigation history

Activating a margin-index region SHALL scroll its existing body region into
view without writing a fragment URL or browser-history entry. The detail
page’s “返回岗位列表” action SHALL therefore return to the list in one navigation
step regardless of prior margin-index activations.

#### Scenario: A reader scans multiple JD regions before returning

- **WHEN** a reader activates one or more margin-index regions and then selects
  “返回岗位列表”
- **THEN** the reader returns to the list without stepping back through those
  region activations

### Requirement: Source-faithful editorial body and distinct keyword emphasis

The selected layout SHALL reuse the deterministic body-section presentation and
safe range highlighter. Ordered source items, bullet source items, lead-in
prose, child items, and ambiguous fallback text SHALL remain visible without
inventing requirements or list order. Its C source prose SHALL use the selected
deeper Slate contrast and its verified technology terms SHALL use the selected
Sea Mint, deep-teal, Lagoon-underlined emphasis.

#### Scenario: A source section cannot be structurally recognised

- **WHEN** a displayed JD body section is an ambiguous fallback
- **THEN** the layout shows its complete source text as prose rather than
  fabricating source items

### Requirement: No comparison controls in the selected layout

The normal detail route SHALL NOT render a variant query control, a fixed A/B/C
switcher, or preview explanation. Obsolete preview query parameters SHALL NOT
alter the selected production layout.

#### Scenario: A reader follows an obsolete preview link

- **WHEN** a reader opens a detail URL carrying the former `prototype` or
  `variant` query parameter
- **THEN** the normal selected C layout renders without a comparison control
