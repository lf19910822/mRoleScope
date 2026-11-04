# Design

## Context

The job browser reads the current Serving inventory. A trend is different: it
must answer how many distinct JDs matched a single formal condition **in each
historical published snapshot**, without treating a later closure, a current
body, or a partially re-parsed dimension as if it had been true in the past.

The CVM read-only audit completed on 2026-11-04 establishes that the current
observation scope has a continuous recoverable Serving history from
2026-10-22 through 2026-11-04. It also found two successful publications on
2026-10-28, so the implementation must select the final successful
publication of each `Asia/Shanghai` date. This is an observed production fact,
not merely a theoretical edge case. The audit did not read JD text, public
identifiers, credentials, or runtime configuration.

Existing immutable facts already provide the source side of this answer:

- `serving_jd_inventory_memberships` preserves every reconciled published
  source membership, observed activity state, locations, and version
  references;
- `run_job_version_references`, body/metadata versions, and SCD2 periods make
  the source observation recoverable;
- deterministic technology, education, experience, project, and job-family
  results are append-only.

The current `serving_jd_current_dimension_resolutions` view deliberately
selects results for the *current* inventory. It is therefore not suitable for
historical trend reads: a trend needs a consistently released interpretation
of each membership's original body or title identity, not the resolution that
happened to be current when a list query ran.

## Goals / Non-Goals

**Goals:**

- Serve one safe, bounded historical curve per request from daily selected
  Serving snapshots.
- Keep snapshot facts immutable, distinguish a trustworthy zero from missing
  history, and evaluate formal dimensions under one complete release.
- Keep the complex source-run selection, SCD2 reconstruction, formal-result
  selection, and gap handling behind one deep Serving module.
- Add a readable Vue dashboard with up to eight independently configurable
  lines, natural date ranges, stable replacements, and true data gaps.
- Reuse the current structured filter vocabulary and existing Vue/Naive UI
  theme; use ECharts plus `vue-echarts` only for chart rendering.

**Non-Goals:**

- No JD drill-down, snapshot links, hover information panel, filter-route
  navigation, or stored node-to-JD payload in this release.
- No copying of full JD snapshots, source payloads, or daily aggregate
  materialization merely for the chart.
- No AI output, free-text query, arbitrary date range, new public network
  exposure, or change to collection/publication transactions.
- No UI disclosure of parser/dictionary versions, coverage/audit state,
  source versions, raw body content, or implementation fallback details.

## Selected module and seam

`jdserving.TrendReader` is the external seam for this change. Its interface is
purposefully small:

```go
type TrendReader interface {
    ReadTrend(context.Context, TrendQuery) (TrendSeries, error)
}
```

`TrendQuery` is a validated public condition plus one named natural range;
`TrendSeries` contains the normalized display condition, ordered date/nullable
count points, and an opaque freshness key. It carries neither an ID for a JD,
run, source version, parser, dictionary, audit record, nor a database key.

The PostgreSQL adapter is the sole implementation initially. It owns all of
the following implementation details:

1. resolving the caller's named range in `Asia/Shanghai` through today;
2. selecting one final published run per scope/day;
3. deciding whether each day is complete, unavailable, or a zero-match day;
4. joining a selected membership to the formal result appropriate to a
   complete trend interpretation release;
5. counting unique jobs after the dimension predicate; and
6. producing the opaque freshness key in one read-consistent transaction.

HTTP is only an adapter: it parses bounded query values, calls `TrendReader`,
and maps known invalid-condition errors to a safe `400`. It must not query
source, SCD2, analysis, or audit tables. Vue is another caller of the same
interface and never reconstructs daily snapshots locally. This depth keeps a
future materialized aggregate adapter possible without changing routes or
client code if measured performance requires it.

## Public contract and route

Add `GET /api/v1/job-trends`. It represents one line, rather than an
unbounded multi-condition report. The client issues at most eight bounded
requests and can therefore refresh or fail one configured line without
blocking readable lines.

The query contract is deliberately aligned to display-safe filter values:

| Parameter | Meaning |
| --- | --- |
| `range` | `week`, `month`, `quarter`, `year`, or `all` |
| `dimension` | `technology`, `job_family`, `education`, `project`, `location`, or `experience` |
| `value` | one structured value from the applicable current selector; education and experience use their bounded canonical values |
| `operator` | required only for `education` / `experience`: `gt`, `gte`, `eq`, `lte`, `lt`; defaults to `eq` only where the UI has not yet selected one |
| `include_secondary` | permitted only for `job_family`, with `true` including primary and secondary family matches |

Repeated, unknown, incompatible, blank, or out-of-vocabulary values are safe
client errors. The handler never silently changes a condition into another
condition and never returns a partial, unbounded series for an invalid query.

The response shape is intentionally small:

```json
{
  "curve": {
    "dimension": "technology",
    "label": "Go",
    "value": "go"
  },
  "points": [
    { "date": "2026-11-02", "count": 3 },
    { "date": "2026-11-03", "count": null }
  ],
  "freshness": "opaque-safe-marker"
}
```

For education/experience, `curve` also returns the accepted comparison
operator and display-safe value; for job family it returns the accepted
secondary-family flag. The marker is an opaque, non-secret consistency value,
not a source or interpretation version. It permits the client to identify a
new coherent result without learning its internal provenance.

The existing filter-options route remains the vocabulary source for the six
condition pickers. The dashboard may add a display-safe bounded list of
comparison choices locally because their semantics are fixed, but it must not
invent selectable technology, family, project, or location values.

## Historical selection and condition evaluation

### Calendar and source facts

The reader generates every natural calendar date from the selected range start
to today's `Asia/Shanghai` date, inclusive. Week starts Monday; month,
quarter, and year start on their natural calendar boundaries. None include a
future date.

For every date and observation scope, a `DISTINCT ON` (or equivalent indexed
window) selects the latest `collection_runs.status = 'published'` run in that
local date. It then reads only its immutable Serving memberships. The count is
`COUNT(DISTINCT membership.job_id)`, so a multi-location membership never
doubles within one location curve and multiple observations are never summed
within a day.

`activity_state`, locations, and membership source identities come from that
selected membership. A later current state must never revise it. Location is
therefore evaluated directly from the membership's immutable location array;
it has no formal-parser release dependency.

### Zero versus gap

For a date, the reader returns a numeric zero only after it has selected a
trustworthy final run and all memberships required to evaluate that curve have
the selected formal interpretation. It returns `null` if there is no published
run for the scope/date, the Serving member is unrecoverable, or any required
formal result is unavailable under the selected complete release. It does not
fill a gap with zero, a partial count, current membership, or a current body.

This whole-day rule intentionally favors an honest break over a visually
complete but unverifiable line. Dates preceding the earliest retained
recoverable observation and today's date before publication are likewise
returned as `null` when included by the requested natural range.

### Dimension predicates

All predicates use the selected release's formal result for the observed
version identity, except locations as described above.

- Technology compares the canonical technology identity. Aliases and case
  variants are one identity and a membership matches at most once.
- Job family compares its primary match by default; `include_secondary=true`
  adds secondary matches but still counts that job once.
- Education compares the explicit formal lower bound using the chosen
  operator. Missing / non-explicit formal education does not match.
- Experience compares the explicit formal lower bound similarly. Missing /
  non-explicit formal experience does not match.
- Project matches the formal project assignment from the observed title
  identity, once per job.
- Location is membership containment, once per job and independently across
  different location curves.

The route must not accidentally import list-view semantics that, for example,
turn a missing requirement into a user-facing dash. Missing requirements are
not comparisons and cannot produce a match.

## Complete formal interpretation release

### Why a release is required

Each deterministic dimension already records an immutable result identity, but
they are not published as one cross-dimension selection. Choosing "latest
row" independently would let an updated project dictionary affect some
historical titles while an updated experience parser remained incomplete. That
would make one chart silently mix formal semantics.

Introduce an append-only trend interpretation release owned by
`jdanalysis`. A candidate records the immutable, typed identity selected for
technology, education, experience, project, and job-family interpretation,
including the project-mask identity required by job-family processing. It also
records a deterministic digest of that complete component set. Release rows
and their component set are immutable; a release is either a published
complete selection or a candidate/superseded selection. The public reader sees
only the newest published complete selection and the opaque digest-derived
freshness marker.

This is metadata about *which existing results are valid together*; it does
not copy JD bodies, source snapshots, per-day memberships, or result payloads.
The concrete migration uses an append-only release header plus one typed
component record per formal dimension, with foreign-keyed technology
configuration/dictionary identities where available and checked identity
fields for the deterministic parser/dictionary families. The Go release
module is the only writer/validator, so callers cannot create an incoherent
component combination by direct ad-hoc use.

### Full-history publication gate

The release operation obtains an advisory lock and reads the full set of
recoverable observed body/title identities referenced by all retained Serving
memberships. It verifies, per relevant dimension, that every identity has
exactly one completed result under the candidate component identity and that
the status is suitable for formal evaluation (including definitive
"not-configured" / "unclassified" results where those are complete formal
answers). It fails closed if any identity is missing, pending, failed, or
ambiguous under rules that cannot produce a formal answer.

The fixed deterministic analysis operation gains a release-preparation path:
it first catches up all recoverable historical versions for a candidate
component set, then attempts publication. A failure records only safe
aggregate operator diagnostics and leaves the earlier complete release active.
No trend request can see a candidate or a partially caught-up identity.

When a complete release publishes, it supersedes the prior release
atomically. New source versions continue to be processed using the same
published identities; until their formal results are complete, only days
depending on those new memberships become `null`. There is no per-JD
exception and no partial global parser promotion.

### Current-list projection remains separate

`serving_jd_current_dimension_resolutions` remains the current job-list
projection and is not repurposed. A trend query joins an immutable membership
to result tables through its observed body/title identity and the selected
release component identities. This preserves the existing list behavior while
giving history one coherent interpretation.

## PostgreSQL query shape and performance

The adapter performs the following set-oriented operations in a read-only,
repeatable-read transaction:

1. resolve the active complete release and opaque marker;
2. generate the named date series in `Asia/Shanghai`;
3. choose the final successful run per scope/day;
4. join selected runs to immutable memberships;
5. join only the dimension tables necessary for the requested condition;
6. compute per-day completeness and `COUNT(DISTINCT job_id)` in SQL; and
7. return all expected dates, including `null` gaps, in order.

Queries get purpose-built additive indexes only after `EXPLAIN (ANALYZE,
BUFFERS)` proves the existing membership/run/version/result indexes are
insufficient. Measurements use safe aggregate queries on CVM or an integration
database and never print JD text, public IDs, connection strings, or runtime
configuration. The performance acceptance target is a measured **under two
seconds** for `all` history with eight independently requested curves against
the real retained history. Only if that measurement fails will this change
return to the user with the evidence and a proposal for a compact aggregate
Serving projection; no daily JD snapshot or materialized trend store is
pre-authorized here.

## HTTP integration

`httpserver.Router` gains a narrow `JDTrendReader` interface and route
registration parallel to its list/detail/filter readers. Validation translates
only known enum/value errors to `{ "error": "invalid_query" }`; unavailable
storage or no usable published release remains `{ "error":
"service_unavailable" }`. Internal wrapped errors are logged by the existing
server policy but never serialized.

OpenAPI documents all enum restrictions, nullable point count, and the
prohibition on provenance/source/JD fields. The response does not accept a
client-supplied release marker and no trend route writes state.

## Vue application design

### Page composition

Add a `JobTrendView` route and a matching permanent left-navigation entry.
Wide layouts keep the current application navigation, a generous central
chart-reading surface, and a right fixed configuration rail. At narrow width,
the chart occupies content width and a clearly labelled control opens the
configuration panel. The rail/panel contains only task-relevant controls; it
does not explain releases, cache behavior, parser coverage, or internal
fallbacks.

Each row represents one normalized condition. It shows one dimension selector,
only that dimension's legal structured value control, comparison selector only
for education/experience, and the secondary-family switch only for job family.
Changing dimension clears incompatible pending values. A row is committed only
through its own application action. The add control prevents a ninth row and
focuses the existing row for a duplicate normalized condition. Labels are
complete conditions, for example `学历 ≥ 本科`, `最低年限 = 3 年`, and `岗位族：运维
（含副岗位族）`.

The empty chart remains structurally sized and says only that the reader can
add a line. It has no sample line, mocked total, or large explanatory card.

### Request ownership and stable replacements

A small trend state module owns persisted configuration, legend visibility,
request sequence numbers, abort controllers, cached aggregates, and line
replacement. Components ask it to apply one row, change range, refresh, or
restore; they do not make independent fetches.

Every request is associated with immutable normalized condition + range + a
per-row generation. A changed/reapplied row aborts its older request when
possible and ignores a late result whose generation no longer matches. A
single-row failure retains its old series and shows a short row-local error;
unaffected rows never become loading placeholders.

The previous chart is retained as the stable visual base while a new series is
loading. Returned points fade/animate into the existing chart without changing
the container's measured height. If a returned opaque freshness marker differs
from the currently displayed coherent marker, the state module refreshes all
affected derived lines in the background and publishes that group to the chart
only after each latest request succeeds with the same marker. Lines unaffected
by formal interpretation remain readable. This prevents an old/new parser
comparison from appearing in one chart.

### Cache and restoration

Persist only normalized row configuration, legend visibility, date-range
choice, chart window, and aggregate `{date, count}` point arrays in browser
storage. Never store JD IDs, titles, bodies, raw filter payloads, source
identities, or release internals. Cache keys include normalized condition and
range; entries expire ten minutes after fetch and use a bounded LRU eviction
policy within that ten-minute window.

- Ordinary route navigation back to a live entry restores valid cached data
  without a request.
- Browser reload (F5), the page's `刷新数据` action, and a row's explicit
  reapply immediately request fresh data. They may retain a cached line as the
  stable base until the request replaces it.
- An already-open page does nothing when its cache age passes ten minutes.
- After expiry, leaving and returning restores configuration and immediately
  requests data; it does not render an expired cache as a newly loaded result.
- A failed network refresh quietly preserves existing cached/readable lines;
  there is no cache/freshness notice in the product UI.

### ECharts mapping and interaction

The chart option factory receives only public `TrendSeries` values and
view-state. It maps a missing count to ECharts `null`, sets
`connectNulls: false`, uses a restrained smooth curve only between adjacent
numeric points, and leaves integers unmodified. The numeric Y axis has
`min: 0` in every state.

This week/month/quarter/year use fixed category/time extents ending today and
install no zoom/pan handler. All-history keeps loaded points in memory and
uses ECharts inside data-zoom for X zoom plus drag-pan; its controlled Y-span
interaction can only change the visible upper span and always clamps the lower
axis bound to zero. Zoom/pan changes chart view state only—never a trend
request or date query—and keeps the current series. A reset-view control
restores the full loaded all-history extent.

Legend click toggles visibility; legend hover focuses the corresponding line.
A point click supplies only a transient in-chart emphasis/ripple. It creates
no router state, information overlay, data fetch, or drill-down payload.

`vue-echarts` is registered with the precise ECharts charts/components used,
and `ResizeObserver`/the wrapper's autoresize maintains chart size without
layout shifts. The global request loader is not used for per-line update
states: this view's stable chart plus row-local indicators better preserves
reading context.

## Migration, deployment, and rollback

1. Add additive up/down migrations for formal trend-release metadata, its
   immutable component selection, supporting validation/indexes, and any
   measured-required query indexes. Existing source snapshots and current list
   projection are neither rewritten nor deleted.
2. Deploy schema and analysis/Serving code before enabling the navigation
   route. Run the fixed deterministic catch-up and candidate release gate over
   recoverable history; use only safe aggregate success/failure counts.
3. Publish only after the full-history gate succeeds. Reconcile regular
   Serving inventory as today; the source collection/publication transaction
   does not acquire trend responsibilities.
4. Deploy the API and web change through existing CI/CD. The database remains
   on its CVM Docker private network; no database/API port, DNS, firewall, or
   secret configuration is changed.
5. Roll back the web/API to the previous image if needed. The trend-release
   rows and deterministic results are append-only facts; the previous complete
   release remains selectable as the safe fallback and no source snapshot
   needs restoration.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| A current list result is mistaken for a historical fact | Trend joins only selected daily memberships and the active complete release, never the current-resolution view. |
| A new parser is visible halfway through catch-up | The immutable global release gate promotes only fully covered component sets. |
| Missing history looks like no demand | Date points are explicitly nullable and `connectNulls` stays false. |
| Eight curves make the page jump or one failure blank the graph | Independent generation-safe rows retain last successful lines and the chart container stays fixed. |
| Full-history query is slow | Measure set-oriented SQL against real aggregate facts before introducing any storage; request approval if the two-second threshold is missed. |
| Client storage grows or leaks JD data | Cache only aggregate points/normalized controls, expires in ten minutes, and has bounded LRU cleanup. |
| ECharts gesture permits misleading Y baselines | The view-state controller locks Y minimum at zero and rejects/pins any nonzero lower bound. |
