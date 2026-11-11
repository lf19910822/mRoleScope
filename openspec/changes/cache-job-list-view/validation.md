# Validation — 2026-11-11

## Red to green

The application-shell test first failed because returning from detail caused a
second list request. Adding `KeepAlive` alone still caused requests on both
departure and return because the retained view watched the global route. The
route-scoped watcher and last-query comparison removed those duplicate reads.

## Checks

- `npx prettier --check` for the three changed frontend files passed.
- `npx vue-tsc --build` passed.
- `npx vitest run` passed: 27 files and 175 tests.
- `npm run build` passed, with the existing large-chunk warning.
- Headless Chromium on the existing `http://127.0.0.1:5175/` preview observed
  two initial list/filter requests, no additional request on detail return,
  one filter-options request by the trend view, and no additional request on
  returning to the list from trends. This checks the local preview, not a new
  remote deployment.
