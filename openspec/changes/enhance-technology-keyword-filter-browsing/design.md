# Design

## Context

See [proposal.md](proposal.md).  The Serving query currently reads each
technology identity's `technology_kind` while building formal mentions, but
the filter-options contract returns only a value and a label.  The Vue page
therefore passes that flat list directly to `NSelect`.

The existing dictionary has 25 populated kinds in the current Serving
inventory.  They are useful browse categories but are data-maintenance
terminology, so their display labels and order must be owned by the Serving
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

`technology_kind` remains the dictionary-owned source classification.  The
Serving module maps each recognized kind to a public category key, Chinese
display label, and fixed display rank.  It returns that category together
with each technology option through the existing filter-options response.
Unknown future kinds map to one final `其他技术` category, so a new formal term
stays selectable without exposing raw maintenance values.

The option query continues to read only current, completed formal technology
results.  The response is sorted by category rank, then canonical display
name and stable technology key.  This makes the left navigation and the
right-pane dictionary order deterministic without asking the browser to infer
business ordering.

Alternative considered: map kinds in Vue.  That duplicates dictionary
taxonomy in a consumer, forces every client to redeploy for a label change,
and would make an unknown kind either disappear or leak raw terminology.

### 2. Isolate the selector as a Vue component

A `TechnologyKeywordSelect` component accepts the typed options and the
selected technology keys through `v-model`.  It owns only transient menu
state: whether it is open, active browse category, and typed search text.
The list page continues to own draft/applied query state and emits the same
`technology` values it does today.

The open menu has a focusable trigger, labelled search input, category list,
and checkbox-style term rows.  Clicking outside or pressing Escape closes
the menu; changing category or a search result never clears selected values.
Search has precedence over browsing: non-empty text searches all options and
the right pane displays the global matching list.  When cleared, the prior
active category's terms return.

Alternative considered: force the layout into `NSelect` option rendering.
That component can customize individual options, but cannot safely own the
two independent panes, global-search precedence, and selected-value behavior
without coupling to undocumented menu structure.

### 3. Preserve query and release compatibility

The filter-options addition is backward compatible: existing consumers can
ignore the new `category` object.  The list request remains repeated
`technology` values, so filter OR/AND rules, URLs, pagination, and serving
SQL do not change.  Server deployment must precede the browser deployment;
an older browser remains usable against the enriched response.

## Risks / Trade-offs

- [The category rail has more entries than the original preview] → make the
  rail independently scrollable and retain an explicit all-terms option.
- [A category only has one current term] → retain it, because hiding it would
  make a selectable formal technology less discoverable.
- [An option's label contains non-ASCII letters] → normalize only for
  case-insensitive matching; do not introduce spelling correction.

## Migration Plan

1. Add contract and Vue component tests that fail on uncategorized options,
   category browsing, search across categories, and selection preservation.
2. Implement the Serving category mapping and API/OpenAPI response extension.
3. Implement the standalone selector and replace only the technology field in
   the list page; update the delivery guide with the approved interaction.
4. Deploy the compatible API before the browser.  Roll back the browser first
   if needed; older browser builds ignore the additive category property.
