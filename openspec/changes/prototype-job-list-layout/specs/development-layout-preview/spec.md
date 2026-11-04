# Spec Delta

## ADDED Requirements

### Requirement: Production query-canvas JD list

The production JD-list route SHALL use the selected query-canvas layout with
the application's desktop left function rail, a one-column result collection,
and a right query console containing the real existing filter and sort
controls. The page SHALL retain its existing Serving request, URL query state,
filter semantics, sorting priority, and page/page-size pagination.

#### Scenario: A reader scans the current collection

- **WHEN** a desktop reader opens `/jobs`
- **THEN** the result collection has one reading column and the real query console is
  available at the right side

### Requirement: Desktop query-canvas page scrolling

At desktop widths, the left rail and query console SHALL remain in the viewport
while a reader uses the browser's global page scroll to scan the result
collection. The result collection SHALL NOT create an independent vertical
scrollbar.

#### Scenario: A reader scans beyond the first screen of roles

- **WHEN** a desktop reader scrolls the page through the result collection
- **THEN** the left rail and query console remain visible, while the role cards
  move with the document and have no independent vertical scrollbar

### Requirement: Compact result count heading

The matching-role count SHALL be presented as a compact structural list heading
rather than a large standalone panel.

#### Scenario: A reader reaches the result collection

- **WHEN** matching roles are available
- **THEN** the list heading presents the count without taking the space of a
  standalone card or panel

### Requirement: Wide-desktop query-rail alignment

At wide desktop widths, the 428px query console SHALL be fixed to the
viewport's far-right and top edges, use the viewport height, and expose an
internal form scroll only when necessary. The role workspace SHALL reserve the
fixed rail's width so role cards remain visible and usable. Concise related
filters SHALL share two-column rows: education / experience, project /
location, activity / urgency, and discovery-from / discovery-to; title, job
family, and technology remain full width.

#### Scenario: A reader opens the query canvas on a wide desktop

- **WHEN** the viewport has room for the left function rail, role workspace,
  and query console together
- **THEN** the query console begins at the page top, ends at the far right,
  shows controls through the viewport height, and does not cover role cards;
  its ordinary no-sort filter state does not require an internal vertical scroll

#### Scenario: A reader inspects a result record

- **WHEN** a role appears in the result collection
- **THEN** its first band shows a role glyph, untruncated title, and primary
  job-family tag at left with activity state and urgency once in top-line pills
  at right; its second band shows location, work experience, education, and
  first-discovered date as wrapping inline facts; and its third band shows all
  available technology keywords as wrapping tags
- **AND THEN** each project label is emphasized only over its exact matching
  characters in the title with a bright-yellow bold highlighter, rather than
  repeated as a separate title tag; other
  matched job families remain visible in a row labelled “副岗位族” when present,
  title and keywords are never hidden behind a `+N` affordance, fields without
  an explicit education or work-experience requirement show `-`, and “首次发现”
  retains its existing meaning

#### Scenario: A role has no technology keywords

- **WHEN** a role has no available technology keywords
- **THEN** its technology band is absent
- **AND THEN** the four-fact overview ends directly, or meets its supplementary
  “副岗位族” row through one adjacent divider without an empty gap or duplicate
  separator

#### Scenario: A reader uses a narrow role column

- **WHEN** the card is narrower than the desktop reading column or appears on
  a phone
- **THEN** title, facts, cities, and keyword tags wrap naturally without a
  fixed card height or data truncation

### Requirement: Contained technology-keyword browser

The technology-keyword browser SHALL fit within the available query-console
width when opened. It SHALL retain its category and term panels without being
horizontally clipped at the viewport edge.

#### Scenario: A reader opens technology-keyword filtering in the right rail

- **WHEN** a reader opens the technology-keyword selector from the fixed query
  console
- **THEN** the complete category and term browser is visible within the rail's
  available width, category labels remain on one line, and it remains usable
  through its panel scroll areas

#### Scenario: A reader wheels an open query option menu to its edge

- **WHEN** a reader scrolls an open Naive UI option menu or either technology
  browser pane until that list reaches its boundary
- **THEN** the menu or pane remains visible and the browser document and query
  console do not scroll as a consequence

### Requirement: Readable sort-choice menus

Sorting-kind and sorting-direction menus SHALL use their natural content width
rather than being constrained to the narrower select trigger. They MUST remain
anchored to their trigger and within the viewport while showing every available
choice label without ellipsis.

#### Scenario: A reader opens a sorting menu in the query console

- **WHEN** a reader opens either a sorting-kind or sorting-direction menu
- **THEN** every available option label is fully readable without widening the
  fixed query console itself

### Requirement: Narrow query-canvas layout

At a 390px viewport, the production query canvas SHALL use a one-column role
collection and initially show roles before the complete filter form. A visible
filter trigger SHALL open the existing query console as a contained mobile
sheet; paired filter rows SHALL stack to preserve usable controls.

#### Scenario: A reader uses a narrow phone viewport

- **WHEN** the viewport is 390px wide
- **THEN** role cards do not form a horizontally compressed two-column grid,
  the reader can reach the collection without passing the full filter form,
  and opening the filter control presents the real existing controls

### Requirement: Compact workbench query clarity

The JD-list route SHALL present a compact workbench context rather than a
decorative landing-page masthead. It SHALL show the current display-safe
filter conditions adjacent to the result collection, retaining the existing
URL query model and allowing a reader to remove an individual condition.

#### Scenario: A reader returns to a filtered collection

- **WHEN** a reader opens a URL with one or more supported JD filters
- **THEN** the result collection identifies each active reader-recognisable
  condition without exposing internal implementation data
- **AND WHEN** the reader removes one condition
- **THEN** only that condition is removed through the existing route state

### Requirement: Progressive query controls

The desktop query console SHALL keep common identity and qualification filters
available while placing lower-frequency contextual controls and multi-rule
sorting in collapsed sections. Its apply/reset controls SHALL remain reachable
while the console itself scrolls. No control, query parameter, sorting
priority, or pagination behavior may be removed or changed.

#### Scenario: A reader needs additional conditions or sorting

- **WHEN** a reader expands contextual conditions or sorting
- **THEN** the existing controls and values are available in the same query
  model and the reader can still apply or reset the draft without scrolling
  past the action controls

### Requirement: Complete Ocean Breeze colour schemes

The application shell and all query-canvas surfaces SHALL use matching Ocean
Breeze light or dark theme tokens. The card-only dark override SHALL NOT leave
the page, navigation, or Naive UI controls in a contradictory light scheme.

#### Scenario: A reader uses the dark appearance

- **WHEN** the application selects its dark colour scheme
- **THEN** the shell, navigation, controls, collection and cards present one
  coherent dark Ocean Breeze surface with the existing information hierarchy

### Requirement: Mouse-only production query-canvas card focus

In the production query canvas, a desktop mouse user SHALL be able to focus a
single role card by hovering it. On pointer leave it SHALL return to its
resting state. Touch pointer events SHALL not enable this motion.

#### Scenario: A mouse focuses one role card

- **WHEN** a mouse pointer enters a query-canvas role card
- **THEN** only that card enters focused motion state and may visually cover
  adjacent cards or rails
- **WHEN** the mouse pointer leaves the card
- **THEN** it leaves focused motion state and returns smoothly to its resting
  position

#### Scenario: A touch user browses role cards

- **WHEN** a touch pointer enters a query-canvas role card
- **THEN** the card remains at its resting size

### Requirement: Production focused-card presentation

A focused query-canvas role card SHALL smoothly scale to 1.15 times its
resting size and layer above its neighbours and adjacent layout surfaces.

#### Scenario: A focused card needs reading space

- **WHEN** a mouse focuses a query-canvas role card
- **THEN** the card scales to 1.15 times its resting size above adjacent
  components

### Requirement: Sharp resting production cards

A query-canvas role card SHALL have no persistent transform, GPU-promotion
hint, or 3D translation while it is not focused, so resting text remains
sharply rendered.

#### Scenario: A reader scans resting cards

- **WHEN** no mouse pointer focuses a query-canvas role card
- **THEN** the card remains untransformed and its text is rendered at rest
