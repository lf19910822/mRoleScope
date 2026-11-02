# Proposal

## Why

The technology-keyword control currently presents every available term in one
flat, long menu. Its formal dictionary already distinguishes technology
categories, but that meaning is not available to the browser, so visitors
cannot browse a category before choosing a term.

## What Changes

- Enrich the existing filter-options response's technology-keyword entries
  with a controlled, display-safe category. No new endpoint or JD list query
  parameter is introduced.
- Replace the flat technology-keyword menu with the approved A layout: a
  left category list and a right multi-select term list in dictionary order.
- Keep global, case-insensitive keyword search. A search ignores the active
  browse category and returns matching terms across the whole vocabulary.
- Preserve selected technology terms when a visitor changes browse category
  or searches. The existing repeated `technology` list query remains the
  only applied-filter representation.

## Capabilities

### New Capabilities

- `technology-keyword-filter-browsing`: display-safe categorized technology
  vocabulary and category-first, globally searchable keyword selection.

### Modified Capabilities

- `jd-browser-ui`: render the technology-keyword selector using categorized
  filter options while retaining its established filter semantics.

## Impact

- The existing `GET /api/v1/jobs/filter-options` response and OpenAPI schema,
  Serving option projection, Vue API types, and the JD list filter control.
- No crawler, raw JD body, parser result, dictionary version, data migration,
  new public endpoint, or list-filter semantics changes.
