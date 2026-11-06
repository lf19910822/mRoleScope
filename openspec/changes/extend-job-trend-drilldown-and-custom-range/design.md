# Design

## Context and constraints

The existing trend reader already selects one final successful published run
per `Asia/Shanghai` day and evaluates curves through the latest complete
formal interpretation release. The extension must reuse that seam. A node
count is a historical fact, so the drill-down list cannot query today's
inventory and cannot accept a client-provided list of job IDs as its source of
truth.

The current JD browser already has a public detail response and a readable
detail template. Historical detail is a version-selection mode of that same
template, not a second presentation or a copied snapshot table.

## Goals

- Add inclusive custom date windows with validation, natural date spacing,
  existing `null`/zero semantics, and existing cache/zoom behavior.
- Reconstruct one node's curve-conditioned historical job set from its final
  published run, then compare each item with the latest inventory.
- Keep list rendering compact: title, three-state status, and permitted action.
- Resolve history through existing SCD2 body/metadata periods and preserve the
  exact historical structured fields.
- Provide a fixed-size desktop anchored popover and a 390px-friendly mobile
  bottom sheet without browser-history pollution or page jumps.
- Measure the agreed performance budgets before considering any new serving
  materialization.

## Non-goals

- No new Serving snapshot table, daily drill-down materialization, or copied JD
  body payload.
- No hover-triggered drill-down, previous/next item navigation, comparison
  diff, or node information tooltip beyond the list container.
- No new port, public database/API exposure, or change to collection runs.
- No frontend reconstruction of SCD2, formal predicates, status comparison, or
  historical membership sets.
- No display of IDs, parser/dictionary/release versions, audit states, or
  internal errors.

## Server seams

### Custom trend range

Extend the existing trend query contract with `range=custom`,
`start_date`, and `end_date`. The calendar resolver validates bounds once and
passes a normalized inclusive range to the existing `TrendReader`.

### Historical node list

Add a read-only `TrendDrilldownReader` seam:

```go
type TrendDrilldownReader interface {
    ReadNode(context.Context, NodeDrilldownQuery) (NodeDrilldownPage, error)
}
```

`NodeDrilldownQuery` contains the validated curve condition, node date,
expected numeric count, page, page size, and trimmed title query. The adapter:

1. selects the final published run for the local node date;
2. joins only its immutable Serving memberships;
3. evaluates the same formal-release predicate as the trend reader;
4. applies title search and deterministic ordering;
5. compares source body/metadata version identities with the latest inventory;
6. returns safe status/action flags plus internal detail locators; and
7. reports `matched_total` and an unsearched count mismatch without exposing
   database provenance.

The count comparison is done in the same read-consistent transaction as the
list query. If no final run exists, the reader returns an unavailable-node
error rather than an empty zero.

### Historical detail

Extend the existing detail reader with a version selector:

```go
type DetailVersion struct {
    Mode       string // latest | historical
    SnapshotDate *time.Time
}
```

The HTTP layer exposes only `latest` or the validated node date. The adapter
resolves the selected run and SCD2 periods, then maps the result into the
existing safe detail DTO. It never returns source-version IDs or fallback
markers. An unrecoverable historical version maps to a stable unavailable
response, not a current-detail fallback.

## HTTP contracts

The new route names are intentionally bounded and REST-readable:

- `GET /api/v1/job-trends/drilldown`: list one node's jobs. It accepts the
  same condition parameters as `/api/v1/job-trends`, plus `date`, `count`,
  `page`, `page_size`, and `title`.
- `GET /api/v1/jobs/{job_id}/detail?version=latest`: existing latest detail
  behavior.
- `GET /api/v1/jobs/{job_id}/detail?version=historical&date=YYYY-MM-DD`:
  historical detail resolved from the node date. The URL carries no run ID or
  version ID.

The response list contains a stable internal job reference, title, status,
allowed action flags, and an opaque detail locator if the existing API needs
one. The frontend maps only title/status/action to visible text. OpenAPI
schemas must mark nullable counts and safe unavailable errors explicitly.

## Frontend state and visual composition

The chart option factory emits a click payload only for numeric points and
keeps the selected node's curve color. The dashboard state owns one
`drilldown` object containing node identity, loading/error state, search,
page, result, and a ten-minute session cache. Search changes debounce around
300ms and reset page to 1; stale responses are ignored by generation/abort
ownership just like curve requests.

Desktop uses a portal/fixed popover (maximum 360–420px wide and about 60vh
high) with a short leader pointing at the chart node. It flips within the
viewport and does not live inside the ECharts canvas. At 600px and below it
becomes a bottom sheet with collapsed/expanded snap points, a drag handle,
scroll lock, overlay, Escape/backdrop/system-back close, and safe-area
padding. Closing detail restores the drilldown object unchanged.

The list skeleton reserves title/search/list/pagination space. Its visible
rows contain only title, one status chip, and the permitted latest/history
buttons. A count mismatch notice is concise and retryable. No node click writes
browser history.

## Testing seams

1. **Custom range resolver** — validation, inclusive dates, Asia/Shanghai
   bounds, and cache-key normalization without querying a database.
2. **TrendDrilldownReader** — final-run selection, condition parity, status
   comparison, title filtering, matched totals, zero/gap behavior, and
   historical restoration failure using synthetic migration fixtures.
3. **HTTP/OpenAPI** — bounded parameters, safe errors, no internal fields, and
   historical/latest detail selector mapping.
4. **Dashboard state** — click eligibility, same/other node behavior, debounce,
   page reset, cache/retry/generation ownership, and context restoration.
5. **Component/browser** — desktop anchored popover, 390px sheet, fixed
   skeleton, scroll lock, detail close return, and no history entries.

## Risks and mitigations

- **History read is slow:** measure aggregate/list/detail budgets first; add an
  index or a separately approved serving projection only after evidence.
- **SCD2 row ambiguity:** fail closed with historical-unavailable and retain
  the list item; never choose an arbitrary row or latest fallback.
- **Chart/list count drift:** surface a retryable mismatch notice and retain
  the service result; do not silently edit either count.
- **Small-screen overlay conflicts:** use portal/fixed positioning, a sheet
  breakpoint, safe-area padding, and explicit background scroll locking.
