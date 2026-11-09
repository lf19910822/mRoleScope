# Validation notes

Validated on 2026-11-09 (Asia/Shanghai) for `add-jd-history-timeline`.

## Automated checks

- `GOCACHE=/tmp/mrolescope-go-cache go test ./...` — passed.
- `GOCACHE=/tmp/mrolescope-go-cache go test ./internal/jdserving ./internal/httpserver ./internal/jobsync -run 'Test(JobHistory|History|JobDetail|DailyFinal|ConfirmedLifecycle)'` — passed.
- `GOCACHE=/tmp/mrolescope-go-cache go test ./internal/jobsync -run TestJobHistoryUsesDailyFinalRunsAndConfirmedLifecycleBoundaries -v` — test is present and correctly skips with `TEST_DATABASE_URL is not set`.
- `cd web && npm run test:unit -- --run` — 27 files, 160 tests passed.
- `cd web && npm run type-check` — passed.
- `cd web && npm run build` — passed.
- `git diff --check` — passed.
- `openspec validate add-jd-history-timeline --strict` — passed.

## Runtime and visual checks

- `http://127.0.0.1:5175/` responded with HTTP 200 from the Vite preview.
- `/job-trends` is served by the same SPA entry. The local Vite-only process has
  no real API proxy; the history API therefore cannot be validated against
  retained production data from this environment.
- Component/state tests cover loading, empty, lifecycle nodes, unavailable
  snapshots, retry, ten-minute cache, refresh behavior, keyboard activation,
  Escape/focus return, horizontal timeline sizing, and safe public fields.
- Google Chrome 151 headless screenshots were captured at desktop (1440x1000)
  and mobile (390x844/390x5000) viewports. The page shell and responsive job
  detail layout render without horizontal overflow; the history section shows
  its local API-error state against the intentionally Vite-only preview.
  Lifecycle/data states remain covered by the component/state suite because no
  API proxy is configured for this preview.

## Data and performance limitation

`TEST_DATABASE_URL` is not configured and no local PostgreSQL container is
running. Consequently, one-job summary/detail p95 and a retained-data `EXPLAIN
ANALYZE` cannot be measured here. The implementation remains read-only and no
new index or projection is introduced without that evidence.
