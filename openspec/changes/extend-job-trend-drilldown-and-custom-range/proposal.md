# Proposal

## Why

The trend dashboard currently supports named natural ranges but does not let a
reader inspect an arbitrary historical interval or explain the jobs behind a
point on a curve. The existing SCD2 source/body history and published Serving
memberships are sufficient to reconstruct the final successful snapshot for a
node date. The missing capability is a bounded read path that keeps the chart
count, historical job set, current status, and historical JD detail on one
consistent date/version boundary.

## What Changes

- Add an inclusive custom `start_date`/`end_date` range, bounded by the earliest
  recoverable history and today's `Asia/Shanghai` date, with the same chart
  zoom/pan and cache behavior as `all`.
- Add click drill-down from a non-null chart node. The server reconstructs the
  node's curve-conditioned job set from that date's final successful published
  snapshot and returns a compact, paginated, title-searchable list.
- Classify each historical job against the latest inventory as `仍有效`,
  `有改动`, or `确认下线`, while keeping source/version identifiers internal
  to the transport and never rendering them in the page.
- Reuse the current JD detail template for both latest and historical details;
  historical details resolve the node-date SCD2 version and never silently
  fall back to current text.
- Add a desktop node-anchored popover and a mobile bottom sheet with fixed
  skeleton dimensions, responsive drag states, search, pagination, retry, and
  context-preserving detail return.

## Capabilities

### New Capabilities

- `job-trend-drilldown`: historical node job-set listing, status comparison,
  title search, pagination, and historical/latest detail selection.
- `job-trend-custom-range`: inclusive custom date validation and trend-series
  retrieval using the existing formal interpretation release.

### Modified Capabilities

- `job-trend-serving`: expose bounded custom ranges and a historical node
  membership read without adding a Serving snapshot table.
- `job-trend-dashboard`: add node interaction, responsive drill-down, and
  context-aware caching while preserving existing curve and gap semantics.

## Impact

- Go read-only Serving/API contracts and OpenAPI schemas; SCD2 and published
  membership queries only. No ODS/DWD rewrite, collection change, or new
  materialized snapshot table.
- Vue trend state, ECharts click mapping, responsive popover/bottom-sheet
  components, and reuse of the existing JD detail template.
- New focused unit, integration, route, component, and browser tests. The
  5175 loopback preview and existing CVM private API path remain unchanged.
