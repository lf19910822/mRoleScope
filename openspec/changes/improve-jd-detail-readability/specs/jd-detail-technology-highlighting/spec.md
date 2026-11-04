# Spec Delta

## Purpose

Let a reader see exactly where the currently served technology keywords occur
in a JD body without exposing analysis internals or falsely highlighting text
from another version of the job description.

## ADDED Requirements

### Requirement: Current-body technology highlight ranges

The existing JD-detail response SHALL provide display-safe character ranges for
deterministic technology mentions only when their evidence belongs to the
served current body and falls within the corresponding displayed section. It
MUST omit unsafe, stale, malformed, or overlapping ranges without failing the
detail response.

#### Scenario: A current technology mention has valid evidence

- **WHEN** a deterministic technology mention has an in-bounds range in the
  served current responsibility, requirement, or supplemental text
- **THEN** the detail response includes that range and its displayed section

#### Scenario: Only an older technology result is available

- **WHEN** technology vocabulary is served through an older same-job fallback
- **THEN** the detail response retains its existing technology keyword display
  but does not provide a range for the current body

#### Scenario: Evidence cannot be safely rendered

- **WHEN** an evidence range is outside its displayed section or conflicts
  with another returned range
- **THEN** the detail response omits that range and still returns the JD

### Requirement: Safe technology emphasis in detail text

The browser SHALL emphasize every supplied technology range in its matching
body section while preserving the original text order and accessible text
content. It MUST render source text as text fragments and MUST NOT inject it
as HTML or perform a second heuristic keyword search.

#### Scenario: A range is supplied for a listed requirement

- **WHEN** a recognized body item contains one or more supplied ranges
- **THEN** the matching characters are visibly emphasized within that item

#### Scenario: No range is supplied

- **WHEN** a body section has no valid highlight range
- **THEN** its text is rendered normally without a speculative highlight

### Requirement: Compact technology-keyword disclosure

When a JD has more than eight technology keywords, the browser SHALL initially
render only the first eight labels, constrained to a two-row preview, and
provide a compact native button to reveal or collapse the complete set at
every viewport width. The button SHALL read `+ 展开` while collapsed and
`− 收起` while expanded; it MUST expose its state and controlled keyword
collection through the existing ARIA attributes and MUST NOT expose an
implementation count or explanation.

#### Scenario: A reader expands a dense keyword set

- **WHEN** a JD has more than eight technology keywords and the reader selects
  `+ 展开`
- **THEN** all technology keywords become visible
- **AND** the same button reads `− 收起`

#### Scenario: A reader collapses a dense keyword set

- **WHEN** the complete technology keyword set is visible and the reader
  selects `− 收起`
- **THEN** the first-eight-label, two-row preview is restored without losing
  keyword data

### Requirement: Public detail information boundary for highlighting

Highlight data SHALL contain only the displayed section and character bounds
needed to render emphasis. It MUST NOT expose a source/body identifier,
dictionary or parser version, matching rule, matched alias, audit state, or
analysis fallback metadata.

#### Scenario: A client inspects the detail payload

- **WHEN** the client receives technology highlight data
- **THEN** it can identify the rendered text interval but receives no internal
  analysis or provenance fields
