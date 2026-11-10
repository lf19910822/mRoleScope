# Fix job-list history query performance

## Why

The recent-modification list query is executed every time readers return to the
job list. The PostgreSQL plan on the current real dataset repeatedly computes
the historical transition aggregate per matching job, adding about half a
second to the list request.

## What Changes

- Bound historical transition aggregation to one execution per list query.
- Preserve existing date, history, filtering, ordering, pagination and API semantics.
- Add a real PostgreSQL regression test for the query plan and retain history parity tests.

## Capabilities

### New Capabilities

- `job-list-history-performance`: Bounded work when the list derives observed modification dates.

### Modified Capabilities

None.

## Impact

The Serving list SQL and database-backed tests change. No schema, source facts,
API contract, user-visible date semantics, cache or client layout changes.
