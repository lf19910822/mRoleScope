# Spec Delta

## Purpose

Provide a calm, accessible loading signal across the JD browser without
discarding useful screen content or making navigation jump while data changes.

## ADDED Requirements

### Requirement: Shared browser loading indicator
The JD browser SHALL use one application-level loading indicator for initial
route data reads and subsequent list/detail refreshes. It MUST remain visible
until all tracked work for the current view has settled and must provide a
polite accessible loading status.

#### Scenario: Multiple reads overlap
- **WHEN** a route data request overlaps another tracked browser read
- **THEN** the shared indicator remains present until both reads have settled

#### Scenario: A request fails
- **WHEN** a tracked browser read fails
- **THEN** the loading indicator clears and the browser presents its existing
  safe error treatment

### Requirement: Geometry-stable initial loading
The browser SHALL reserve the approximate geometry of the list or detail view
with an Ocean-Breeze-compatible skeleton during its initial data load. It MUST
not replace the route with a small text-only loading line that moves later
content when the response arrives.

#### Scenario: A visitor first opens the JD list
- **WHEN** no list result has been rendered and the initial request is pending
- **THEN** the list area shows a stable skeleton matching the eventual content
  region

#### Scenario: A visitor first opens a JD detail
- **WHEN** no detail has been rendered and the initial request is pending
- **THEN** the detail area shows a stable skeleton matching the detail card

### Requirement: Non-jumping refresh feedback
The browser SHALL retain the most recently successful list or detail content
in its current geometry while a replacement request is pending. It MUST show a
non-blocking loading veil or indicator without resetting scroll position,
unmounting content, or changing the page's reserved height.

#### Scenario: A visitor changes a filter or page
- **WHEN** the list issues a replacement request after an interaction
- **THEN** its last successful result remains geometrically stable until the
  replacement is ready

#### Scenario: A visitor opens another detail
- **WHEN** an already rendered detail begins loading a different JD
- **THEN** the previous detail remains geometrically stable behind refresh
  feedback until the new response is ready

### Requirement: Motion-aware loading treatment
The browser SHALL use the same branded loading treatment in list and detail
views and MUST reduce or stop its nonessential motion when the visitor prefers
reduced motion.

#### Scenario: Reduced motion is enabled
- **WHEN** the visitor has requested reduced motion
- **THEN** the loading treatment remains visible without continuous decorative
  animation
