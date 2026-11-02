# Design

## Context

See [proposal.md](proposal.md). The Serving query already reads each
technology identity's `technology_kind` while building formal mentions, but
the filter-options contract returns only a value and a label. The Vue page
therefore passes that flat list directly to `NSelect`.

The dictionary kinds are useful browse categories but are data-maintenance
terminology. Their display labels and order must be owned by the Serving
boundary rather than re-created in the browser.

## Goals / Non-Goals

**Goals:**

- Make the existing formal vocabulary browseable by a stable public category.
- Implement the approved A layout without changing the applied technology
  filter, URL state, or endpoint count.
- Preserve the menu's current global type-to-filter behavior, including while
  a category is active.

**Non-Goals:**

- No new technology tagging, dictionary-category authoring UI, synonym or
  typo correction, body-text search, hierarchy below a category, query API
  parameter, migration, or historical recomputation.

## Decisions

### 1. Derive public categories at the Serving boundary

`technology_kind` remains the dictionary-owned source classification. The
Serving module maps each recognized kind to a public category key, Chinese
display label, and fixed display rank. It returns that category together with
each technology option through the existing filter-options response. Unknown
future kinds map to one final `其他技术` category, so a new formal term stays
selectable without exposing raw maintenance values.

The option query reads only current, completed formal technology results. The
response is sorted by category rank, then its existing canonical display-name
and stable technology-key ordering. This makes the left navigation and the
right-pane dictionary order deterministic without asking the browser to infer
business ordering.

### 2. Isolate the selector as a Vue component

`TechnologyKeywordSelect` accepts typed options and selected technology keys
through `v-model`. It owns only transient menu state: whether it is open, the
active browse category, and typed search text. The list page continues to own
draft/applied query state and emits the same `technology` values it does now.

The menu has a focusable trigger, labelled search input, category list, and
checkbox-style term rows. Clicking outside closes the menu; changing category
or searching never clears selected values. A non-empty search has precedence
over category browsing and searches all options. When cleared, the active
category's terms return.

### 3. Preserve query and release compatibility

The filter-options addition is backward compatible: existing consumers can
ignore the new `category` object. The list request remains repeated
`technology` values, so filter OR/AND rules, URLs, pagination, and serving SQL
do not change. An older browser remains usable against the enriched response.

## Risks / Trade-offs

- The category rail may have more entries than the original preview, so it is
  independently scrollable and retains an explicit all-terms option.
- A category with one current term remains visible, because hiding it would
  make a selectable formal technology less discoverable.
- Category mapping does not leak raw dictionary kinds; unknown kinds use the
  controlled `其他技术` fallback.

## Migration Plan

1. Add Serving/HTTP contract tests and Vue component tests.
2. Add the category mapping and the additive response field.
3. Replace only the technology field with the selector; preserve existing
   repeated `technology` list-query behavior.
4. Deploy through the existing private remote-development release workflow,
   then verify the real filter options and both desktop and 390px UI states.
