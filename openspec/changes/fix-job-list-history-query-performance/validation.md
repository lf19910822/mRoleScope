# Validation — 2026-11-10

## Red → Green

`TestListJobsComputesHistoryTransitionsOnce` executes the list repository with
182 synthetic jobs, 22 published days and 166 effective jobs, then explains
the exact captured list query using a forced nested-loop plan. On PostgreSQL
16.4 before the change it failed: the history transition subplan executed 166
times. After the change it passed with one execution. The assertion measures
plan work rather than imposing an environment-dependent wall-clock limit.

## Functional checks

- A disposable PostgreSQL 16.4 fixture ran eight integration tests covering
  the new regression, daily-final history, scope, repair, reopened jobs,
  modification dates, sort directions, pagination and structured filters.
- `GOCACHE=/tmp/mrolescope-go-cache go test ./cmd/... ./internal/... -count=1`
  and `GOCACHE=/tmp/mrolescope-go-cache go build ./cmd/...` passed. Database
  integration tests in the general Go run skip without `TEST_DATABASE_URL`;
  the targeted integration tests above did use a disposable database.
- `gofmt`, `openspec validate fix-job-list-history-query-performance --strict`
  and `git diff --check` passed.

## Query measurements

Before implementation, a read-only CVM PostgreSQL comparison on the then-current
dataset measured the original descending list SQL at 545–552 ms versus 30–32 ms
with the aggregate materialized. The compared first pages had 20 rows and no
differences. This was a database-only experiment, not a deployed end-to-end
latency measurement, and does not prove parity for all filters.

After implementation, an isolated synthetic fixture of 1,000 current jobs and
60,000 history memberships produced a single aggregate execution and a 199 ms
main-query execution (one measurement). This fixture has no modeled dimension
rows and is not a production latency estimate. No CVM data or schema was changed.
