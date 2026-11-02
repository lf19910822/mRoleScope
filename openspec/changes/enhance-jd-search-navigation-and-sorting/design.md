# Design

## Context

See [proposal.md](proposal.md) for motivation and the two capability specs for
the observable behavior. The current list reads a version-aware Serving
projection, but its query interface exposes `MainJobFamilies`,
`OtherJobFamilies`, and one scalar sort. The browser mirrors those fields in
its URL state and applies a default router scroll position after every route
change. `App.vue` only provides theme configuration, so there is no product
navigation shell.

Current JD job-family facts already persist one `job_family_matches` row for
every matched family, including the primary family. The primary key is a
display decision derived from those same matches. Education and experience
ranks are currently computed in the Serving query; no source or AI relation
may enter this change's read path.

## Goals / Non-Goals

**Goals:**

- Give callers one small list-query interface for all-family matching and
  ordered requirement sorting.
- Keep the ordering globally correct before pagination, stable across requests,
  and bounded by validated input.
- Preserve the reader's viewport for in-list refinements while keeping
  browser-back restoration and detail navigation natural.
- Introduce a responsive application shell whose navigation can gain real
  destinations later without restructuring every page.

**Non-Goals:**

- No current-search table, historical-data backfill, source schema change,
  crawler change, parser rerun, AI result, body-text search, chart route, or
  placeholder navigation entry.
- No arbitrary sort expression, client-side sorting, unlimited sort rules,
  sorting of multi-value dimensions, or change to the existing
  education-rank interpretation.

## Decisions

### 1. Make the Serving list-query module own structured list semantics

`jdserving.ListJobs` remains the external seam between HTTP and the Serving
read model. Its interface gains a single `JobFamilies []string` field, an
optional `MinimumExperienceYears` threshold, and an ordered
`SortRules []SortRule` field. A `SortRule` has a constrained field
(`experience`, `education`, `first_discovered`, `urgent`, `activity_state`,
or `title`) and direction (`asc` or `desc`); it is parsed from repeated
public `sort` values such as `experience_desc`.

The module accepts zero through four structured sort rules; zero means the
existing stable title/relevance order. A field cannot occur twice regardless
of direction. `sort=title` remains accepted only as the old equivalent of
zero structured rules. The parser rejects an unknown value, a duplicate
field, more than four rules, a title rule combined with another rule, or a
request mixing new `job_family` values with legacy main/other-family values.
The latter makes cross-generation shared links unambiguous rather than
silently intersecting unrelated selector styles.

The threshold is sent as one optional `minimum_experience_year` value from
zero through nine. It is a separate current browser control; old repeated
`experience_year` exact-match parameters remain accepted for compatibility.
The threshold predicate requires an experience result whose status is
`configured` and whose base year is greater than or equal to the selected
value. Thus `minimum_experience_year=0` means “an explicit requirement is
available”, not “show jobs whose requirement is unknown”.

`main_job_family`, `other_job_family`, and one existing sort value remain
valid compatibility input for this release. The new browser only emits
`job_family` and ordered sort values. This lets the server ship before the
browser and lets an old browser continue to work after the server is updated.

This produces a deep module: callers supply declared filter values and sort
rules, while validation, defaulting, null handling, deterministic tie-breaking
and safe SQL selection stay inside the module. The HTTP handler is only an
adapter for query strings; the Vue client is another adapter for URL state.

Alternative considered: retain two browser filters and OR their results in the
browser. It loses the requested single-filter meaning, fails before pagination
and duplicates data semantics outside the Serving seam.

### 2. Match unified job families through the complete match fact set

The unified family predicate uses `job_family_matches.family_key` joined to the
current selected job-family result. The predicate is an `EXISTS` condition
with `family_key = ANY($job_family_values)`, so values are ORed inside the
dimension and remain ANDed with every other list dimension. Filter options use
the same complete match set and return each key once.

The list/detail DTO remains unchanged: it still exposes a separate primary
family and other matched families. This separates the search meaning (“any
formal hit”) from the display meaning (“the deterministic primary decision”).

Alternative considered: combine primary keys and non-primary keys with a SQL
union at query time. That represents the same facts twice and can drift when a
new family result is written; `job_family_matches` is the single complete
relation already used to derive other matches.

### 3. Assemble threshold predicates and multi-sort SQL only from validated fixed fragments

The PostgreSQL adapter keeps education and experience rank derivation inside
the Serving query. Its experience threshold is a parameterized `EXISTS`
predicate over the current configured experience result. It appends a fixed
SQL fragment for each validated sort rule, in received order:

1. configured-value marker (`NULL` ranks last) for education and experience,
2. the fact in the requested direction,
3. each later rule in received order, up to the four-rule limit,
4. `lower(title), public_id` as the stable final tie-breaker.

The browser never sorts a received page: that would make page totals and later
pages incorrect. Raw query text, field names, and directions are never added
to SQL; only the module's fixed sort-fragment map may be selected.

The initial implementation keeps the current Serving schema. It adds an index
only if representative `EXPLAIN (ANALYZE, BUFFERS)` proves it necessary. The
first candidate is a reverse lookup index on
`job_family_matches (family_key, job_family_result_id)`. If real growth makes
rank derivation or full-result counting the bottleneck, a separately designed
current-search projection can flatten only current membership IDs and rank
columns. It must remain rebuildable from immutable Serving facts and cannot
become a new source of truth.

`first_discovered`, `urgent`, `activity_state`, and `title` are scalar Serving
facts and can be ordered unambiguously. Job family, technology, project, and
location are deliberately excluded because each can have several values; an
invented minimum/maximum ordering would be misleading.

Alternative considered: add a generic query-to-SQL sorter or Elasticsearch.
The former expands the interface beyond the approved scalar fields and risks
SQL injection; the latter adds operational cost without evidence that
PostgreSQL cannot meet the bounded current-inventory workload.

### 4. Keep ordered list state in the URL and suppress only in-list scroll resets

The Vue list state replaces main/other family arrays with `jobFamilies`, the
legacy exact experience selection with `minimumExperienceYear`, and the scalar
sort with `sortRules`. Serializing repeated values preserves their order in
the query string, so copied URLs retain both filter and priority meaning. The
work-experience control is one ordinary select labelled `最低工作年限`, with
`不限` plus fixed `1 年及以上` through `9 年及以上` choices; it is not populated
from the database's incidental current values. The API retains zero as a
compatible input for pre-existing shared links, but the browser never offers
it as a new selection.

The sort-builder UI presents every selected rule as a priority label followed
by separate `排序种类` and `排序方式` selects, plus accessible move-earlier,
move-later, and removal controls. It permits at most four rules. The add row
uses the same two selectors. Changing a kind retains the chosen direction
unless that direction has no meaning for that kind. A kind already present in
another rule cannot be selected. The list view uses a quieter two-surface
composition: a concise search heading, a filter card with grouped fields, and
a visually distinct sort card. It keeps Ocean Breeze colours and keyboard
focus but removes system-explanatory copy and the dense threshold-button row.

Pagination remains offset based. The browser serializes `page` and
`page_size`, displays numbered page controls, lets the visitor choose a
bounded page size, and supplies a direct page-number jump. It never accepts
or emits a cursor token. Changing page size returns to page one; changing page
preserves the selected filters and ordered sort rules.

The router returns no new scroll position for a navigation whose source and
destination are both the JD-list route; Vue consequently leaves the viewport
where the reader placed it. Saved browser positions take precedence for a
back/forward restoration. Navigation to another route, including a detail,
retains normal top-of-page behavior.

Alternative considered: scroll the list result into view after each apply.
That is still a forced jump and contradicts the requirement for a reader who
applies a refinement while examining lower results.

### 5. Add a released-destination navigation shell at the application seam

`App.vue` becomes the application-shell module around the routed page. A small
data list describes released navigation entries; today it contains only
`岗位筛选 → /jobs`. Route grouping treats both `/jobs` and `/jobs/:id` as that
entry's active state. The list deliberately contains no planned chart or
analysis destinations.

On desktop, a left Ocean Breeze `Layout` sider remains visible beside the
routed content. On narrow screens it becomes a named menu control and
dismissible overlay, keeping content full-width when closed. The visual
direction is a calm “tide rail”: a compact three-line cyan mark at the top,
the one active destination as the current channel, and restrained utility
copy. This uses the supplied sider, menu, filled-control and rounded-corner
tokens without presenting a generic admin dashboard.

Alternative considered: copy a sidebar into the list page only. It would
disappear on detail pages and force every future function to recreate
navigation, losing locality at the application-shell seam.

## Verification Record

On the isolated Serving integration database, the 200-JD deployment-scale
fixture ran `EXPLAIN (ANALYZE, BUFFERS, COSTS OFF)` for the unified
`job_family=backend` predicate with experience-descending then
education-descending ordering. The bounded 20-row plan completed in about
0.01 seconds and referenced `job_family_matches` while avoiding all source
payload and AI relations. The existing current-resolution indexes were
sufficient at this scale, so the candidate reverse-match index was not added.

## Risks / Trade-offs

- [A compound filter/sort plan scans more current rows than expected] → run
  real representative plans before indexing; add only a measured current-read
  index and retain page-size validation.
- [An old shared URL and a new query form have ambiguous combined meaning] →
  preserve either request style independently and reject mixed styles safely.
- [A viewport-preserving refresh leaves a visitor below a much shorter result]
  → let normal browser layout clamping apply; do not add a new forced scroll.
- [A desktop sider crowds narrow content] → switch at a tested responsive
  breakpoint, expose a named menu control, and test keyboard dismissal.
- [A later feature is not ready but is shown in navigation] → derive the menu
  only from released route descriptors; no disabled future entries exist.

## Migration Plan

1. Add red HTTP/repository and browser/router tests for unified family
   matching, threshold experience filtering, ordered sort validation,
   null-last ordering, scroll preservation, and responsive navigation.
2. Implement the Serving query interface and OpenAPI extension; run the
   existing single-sort and legacy-family tests unchanged.
3. Run representative compound-filter/sort `EXPLAIN (ANALYZE, BUFFERS)` on
   deployment-scale data. Add the candidate match index only if the plan
   demonstrates a need; no historical facts need migration or recomputation.
4. Implement the URL-backed sort builder, threshold experience control,
   unified filter, and application shell; build and test narrow and desktop
   navigation states.
5. Deploy the compatible server before the new browser. If rollback is needed,
   restore the browser first, then the server; old browser requests remain
   accepted throughout.
