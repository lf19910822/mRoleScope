# Validation

Validated 2026-11-06.

- `GOCACHE=/tmp/mrolescope-gocache go test ./...` passed. Jobsync tests that
  require `TEST_DATABASE_URL` use their existing skip guard, so this run did
  not exercise a real PostgreSQL historical comparison.
- `npm run test:unit -- --run` passed: 25 test files, 153 tests.
- `npm run build` passed, including `vue-tsc --build` and `vite build`.
- `openspec validate compare-historical-jd --strict` passed.
- `git diff --check` passed.
- The existing local Vite process served
  `http://127.0.0.1:5175/job-trends`; the port was already occupied, so no
  second preview process was started. Automated component coverage includes
  collapsed/expanded comparison, diff semantics, retry emission, and narrow
  layout styles; real-data desktop/390px visual capture remains pending a
  database-backed preview.
