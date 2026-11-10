# Validation — 2026-11-10

## Functional checks

- `GOCACHE=/tmp/mrolescope-go-cache go test ./internal/jdserving ./internal/httpserver ./internal/jobsync -count=1` passed. Without `TEST_DATABASE_URL`, database-dependent tests in this general run skip.
- Three history/list integration tests passed against an isolated disposable PostgreSQL 16.4 container, with synthetic observations. They cover daily-final selection, gaps, unchanged-source reopening, historical repair, nulls, both sort directions, pagination, multiple sort rules and the history-selected scope. No production database was accessed.
- `npm run test:unit -- --run src/lib/__tests__/list-state.spec.ts src/views/__tests__/JobListView.spec.ts` passed (24 tests); `npm run build` passed, including Vue type checking.
- Prettier, ESLint and Oxlint checks passed on the five changed frontend files; Go changes were formatted with `gofmt`.
- `openspec validate add-job-last-modified-sort --strict` and `git diff --check` passed.

## Browser

The existing loopback preview at `http://127.0.0.1:5175/` was tested in headless Chrome at 1440 × 900 and 390 × 844. Playwright intercepted list/filter requests with synthetic data based on an existing unit-test fixture; these are not production job histories. Screenshots were inspected: the date and `-` fit the existing card metadata layout, the selected sorting label and direction are visible, and neither viewport had document-level horizontal overflow.

## Query cost

An isolated PostgreSQL 16.4 container was populated with one scope, 1,000 synthetic current jobs and 60 daily publications each (60,000 history memberships), two body versions per job and no modeled dimension rows. `EXPLAIN (ANALYZE, BUFFERS)` ran the actual list SQL with the default effective-state filter, descending modification sort and a 20-item page. One run of the final query took **232.357 ms** execution and **6.309 ms** planning. The first version took 389.686 ms; removing a redundant body-version join and grouping by the selected job reduced the measured cost. Results are a single local synthetic measurement, not a production latency estimate or capacity guarantee. The query still processes history for current jobs before pagination; growth in job count and retained dates may warrant a rebuildable projection or index after measurement on representative production data. No production plan or timing was collected.
