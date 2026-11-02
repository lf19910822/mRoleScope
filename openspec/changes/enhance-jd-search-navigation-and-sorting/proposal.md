# Proposal

## Why

The first JD browser exposes primary and other job-family matches as two
unrelated filters, offers only a single fixed sort choice, and resets the
reader to the top of the page whenever filters are applied.  As the browser
becomes the entry point for more JD and future analysis features, it also
needs a durable navigation shell rather than another page-local header.

## What Changes

- Replace the browser's separate primary/other job-family controls with one
  `岗位族` filter that matches every formal job-family hit.  A returned JD
  continues to show its primary and other matches separately.
- Present the optional minimum-work-experience threshold as the same simple
  select control used by the other fields. It offers `不限` and `1–9 年及
  以上`; it does not expose a row of threshold buttons or implementation
  explanations. The compatible API representation remains
  `minimum_experience_year`.
- Expand ordered multi-level sorting from minimum work experience and
  education requirement to the other single-value facts that have an
  unambiguous order: first-discovered date, urgent marker, activity state,
  and title. The first selected rule wins; later rules resolve ties.
- Make every sort rule a pair of independent controls: a sort kind and a
  sort direction. The UI must not present an opaque combined value such as
  `学历要求：从高到低` as its only editable control.
- Allow up to four ordered sort rules. The first rule wins and later rules
  resolve ties; duplicate sort kinds remain invalid.
- Keep offset pagination explicit and URL-backed with `page` and `page_size`.
  The browser shows numbered pages, page-size selection, and a direct page
  jump; it does not use or expose a cursor continuation.
- Remove system-oriented explanatory copy from the filter, sorting, and result
  areas. Visitors see only controls and short task-oriented labels.
- Extend the list contract to accept ordered repeated sort values and a
  unified job-family value, while retaining single-sort and legacy family
  parameters as compatible API input during this release.
- Preserve the reader's viewport when applying, resetting, or changing list
  query state within the JD list route; keep normal detail-page and
  browser-back navigation behavior.
- Introduce a responsive left navigation shell.  It contains the real
  `岗位筛选` entry now and has a data-driven extension point for later released
  functions; it does not render links to placeholder pages.
- Verify the new family-filter and compound-sort paths against the serving
  read model with representative query plans before adding an index or a
  flattened current-search projection.

## Capabilities

### New Capabilities

- `jd-search-interaction`: unified job-family selection, threshold
  experience filtering, ordered structured sorting, stable paged list
  behavior, and viewport-preserving list updates.
- `application-navigation`: the responsive browser navigation shell and its
  rule that only released destinations are navigable.

### Modified Capabilities

- None. The original JD-browser and serving-search capability changes remain
  active and unarchived; this change extends their public behavior without
  removing their accepted request forms.

## Impact

- Vue list state, list controls, router scroll behavior, application shell,
  component/router tests, visual layout, and the frontend delivery guide.
- OpenAPI and Go list-query parsing, validation, query construction,
  filter-option serving, integration tests, and measured PostgreSQL query
  plans.  The crawler, ODS data, formal parsers, AI workflow, and existing
  immutable JD-modeling facts remain unchanged.
