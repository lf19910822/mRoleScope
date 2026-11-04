# Tasks

## 0. TDD seams and implementation preparation

- [x] 0.1 Present the agreed public test seams below to the product owner and obtain confirmation **before writing any test**, per the repository TDD rule:
  - `TrendReader.ReadTrend`: catches daily final-publication selection, historical activity facts, distinct counting, zero versus `null`, all six predicates, and complete-release selection; it does not test SQL statement spelling.
  - `GET /api/v1/job-trends`: catches query validation/status mapping and the safe response boundary; it does not test PostgreSQL history reconstruction.
  - trend state module: catches one-row application, abort/generation race safety, coherent replacement, cache/reload/expiry rules, and configuration restoration; it does not test ECharts rendering internals.
  - chart-option factory: catches `null` gaps, zero baseline, fixed versus all-history controls, and point-click boundary; it does not test canvas pixels.
  - browser/component seam: catches route/navigation, desktop rail, and 390px configuration-panel reachability; it does not reproduce every ECharts gesture implementation.
- [x] 0.2 After seam confirmation, inspect existing migration/test helpers, router construction, current deterministic result identities, and filter vocabulary to name fixtures after the actual domain terms; record any mismatch with this design before the first red test.

## 1. Formal interpretation release and historical trend reader

- [x] 1.1 Write a red integration test at the `TrendReader` seam for two successful same-day publications, verifying that only the final `Asia/Shanghai` publication supplies the day's distinct count and historical activity state.
- [x] 1.2 Implement the minimal named range/calendar resolver and selected-run query needed for task 1.1; keep the query read-only and verify the focused test turns green.
- [x] 1.3 Write red `TrendReader` tests that distinguish complete no-match `0` from missing selected run, unrecoverable membership, and unavailable formal result `null`; include today-before-publication and preserve calendar date positions.
- [x] 1.4 Implement whole-day completeness/gap evaluation and response points needed for task 1.3; verify no current membership/body or partial count is substituted.
- [x] 1.5 Write red migration-backed tests for append-only trend formal-release header/component metadata, component digest stability, immutable records, and rejection of an incomplete candidate publication.
- [x] 1.6 Add additive up/down migrations plus the minimal `jdanalysis` release module to create, validate, publish, supersede, and read complete formal interpretation releases; verify task 1.5 is green without copying source/JD snapshots.
- [x] 1.7 Write red release-gate integration tests for all recoverable historical body/title identities, including a pending/failed/missing dimension result that keeps the prior complete release active.
- [x] 1.8 Implement full-history typed identity coverage validation, advisory-lock publication, safe aggregate outcome, and fixed deterministic catch-up/release orchestration; verify task 1.7 passes and no partial global release becomes readable.
- [x] 1.9 Write red `TrendReader` tests for canonical technology aliases, job-family primary/secondary behavior, formal education/experience comparisons excluding missing requirements, projects, multi-location distinct counting, and no cross-day accumulation.
- [x] 1.10 Implement only the condition joins/predicates required by task 1.9 against the selected complete release; verify each curve remains a single normalized condition and returns no JD identifiers.
- [x] 1.11 Measure the real retained-history access path before adding indexes. Add an index only if a repeatable aggregate-only measurement demonstrates an absent path; the deployed eight-curve result is below the acceptance target, so no index is warranted.

## 2. Safe HTTP and OpenAPI boundary

- [x] 2.1 Write red route tests at `GET /api/v1/job-trends` for each valid dimension/query shape, invalid/incompatible/repeated values, safe 400 versus unavailable 503 mapping, and the absence of internal provenance fields.
- [x] 2.2 Add typed public trend contracts, a narrow `JDTrendReader` router seam, parameter parsing/validation, and the route registration needed for task 2.1; verify focused HTTP tests pass without a live database.
- [x] 2.3 Update `openapi/openapi.yaml` with the bounded trend route, enums, nullable day count, opaque freshness field, and safe error responses; add a contract regression test or validation that prevents public source/parser/audit/JD-ID fields.

## 3. Trend dashboard state and chart mapping

- [x] 3.1 Write a red state-module test for an initially empty current-week dashboard, adding up to eight unique normalized rows, rejecting a ninth/duplicate, and clearing incompatible controls when dimension changes.
- [x] 3.2 Implement the minimal typed trend API client and persisted configuration state needed for task 3.1, reusing existing structured filter options and not creating mock JD data.
- [x] 3.3 Write red state tests for explicit one-row application, independent loading, abort/generation handling of late responses, old-line retention on one-row failure, and immediate global range changes.
- [x] 3.4 Implement request ownership and stable per-row replacement for task 3.3; verify unaffected lines do not reload and stale responses cannot overwrite the latest row configuration.
- [x] 3.5 Write red state tests for opaque-marker mismatch grouping, group replacement only after coherent latest results, ten-minute ordinary-return cache reuse, F5/refresh/reapply forced request, expiry while mounted, and expired re-entry behavior.
- [x] 3.6 Implement aggregate-only bounded browser cache, restore behavior, silent failed-refresh retention, and coherent group replacement required by task 3.5; ensure no JD IDs, bodies, titles, or internal release metadata are persisted.
- [x] 3.7 Write red option-factory tests for ordered `null` gap points, zero values, `connectNulls: false`, Y minimum zero, fixed preset ranges, all-history zoom/pan bounds, legend behavior, and click feedback without navigation/drill-down.
- [x] 3.8 Add `vue-echarts`, register only used ECharts primitives, and implement the option/view-state factory plus all-history controlled zoom interactions needed for task 3.7; verify Y never moves above zero as its lower bound and no zoom issues a data request.

## 4. Vue route, responsive composition, and visual verification

- [x] 4.1 Write a red browser/component test that the new left navigation opens the empty dashboard and that it renders a concise no-curve prompt with no mock line.
- [x] 4.2 Implement `JobTrendView`, router entry, navigation label, central stable chart region, and fixed desktop configuration rail needed for task 4.1; preserve existing application layout and theme.
- [x] 4.3 Write red component/browser tests for complete readable labels, row-local loading/error, legend state restoration, desktop rail availability, and a 390px layout where the full-width chart can open/reach the configuration panel.
- [x] 4.4 Implement the responsive configuration panel, constrained controls, stable-size chart loader/replacement, and existing-theme styling needed for task 4.3. Use scoped CSS and verify no internal implementation explanations are rendered.
- [ ] 4.5 Manually verify desktop and 390px views at `http://127.0.0.1:5175/` using real Serving responses when available: zero point, missing-point break, long labels, eight lines, one-line failure, cache restore, all-history zoom/pan/reset, and no node drill-down.

## 5. Verification, rollout, and performance gate

- [x] 5.1 Run focused Go unit/router tests, migration-backed trend integration tests, worker release-gate tests, focused Vitest suites, `npm run type-check`, `npm run build`, applicable Go test suites, and `git diff --check`; record exact commands/results and separately state any unavailable integration database in `validation.md`.
- [x] 5.2 Run `openspec validate add-job-trend-dashboard --strict` and resolve every finding before implementation review.
- [x] 5.3 Measure real retained-history all-range performance for eight configured curves using safe aggregate-only CVM checks. If it is under two seconds, record the timing and keep the non-materialized reader. If it is not, stop before adding a daily/materialized trend projection and present the measurement plus a separate design for approval.
- [x] 5.4 Deploy through the existing reviewed CI/CD path only after verification. Run the formal historical catch-up/release gate with aggregate-only operator output; verify the public route through the existing API path without exposing the database/API publicly or reading/writing credentials.
- [x] 5.5 Update task checkboxes and the decision record with completed verification, published-release outcome, measured performance, and any deliberately deferred drill-down work.
