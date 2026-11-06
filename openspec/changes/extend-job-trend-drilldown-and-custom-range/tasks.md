# Tasks

## 0. Contract and TDD preparation

- [x] 0.1 Confirm the custom-range resolver, `TrendDrilldownReader`, historical
      detail selector, dashboard drill-down state, and responsive container
      seams before writing tests.
- [x] 0.2 Inspect the existing trend/detail API routes, SCD2 readers, pagination
      contracts, OpenAPI schemas, and current component test fixtures; record
      any mismatch with `design.md` before the first red test.

## 1. Custom date range

- [x] 1.1 Write red resolver tests for inclusive dates, Asia/Shanghai today,
      retained-history bounds, malformed/repeated parameters, reversed ranges,
      and future end dates.
- [x] 1.2 Implement the normalized custom range in the existing trend reader
      and route; keep all natural date, null/zero, formal-release, zoom, and
      cache semantics unchanged.
- [x] 1.3 Add OpenAPI and frontend state tests for restoring the last custom
      range, cache-key inclusion, and forced refresh/reload behavior.

## 2. Historical node list and status comparison

- [x] 2.1 Write red migration-backed `TrendDrilldownReader` tests for final
      same-day publication selection, condition parity, numeric zero versus
      null, distinct job counting, and unsearched count consistency.
- [x] 2.2 Implement the read-only historical node query using existing
      published memberships and the complete formal interpretation release.
- [x] 2.3 Write red tests for unchanged, changed, confirmed-closed, and
      latest-inventory-absent jobs; verify allowed actions and safe internal
      transport fields.
- [x] 2.4 Implement deterministic status/action mapping, title search,
      `page`/`page_size`, `matched_total`, mismatch notices, and bounded errors.

## 3. Historical/latest detail

- [x] 3.1 Write red detail-reader tests for latest mode, node-date historical
      mode, all structured fields bound to the historical version, and
      unrecoverable history without current fallback.
- [x] 3.2 Implement the version selector through SCD2 and map both modes to
      the existing safe JD detail DTO; do not add a snapshot table or expose
      version IDs.
- [x] 3.3 Add route/OpenAPI tests for `latest`/`historical` validation,
      date bounds, safe unavailable response, and absence of internal fields.

## 4. Dashboard drill-down state and UI

- [x] 4.1 Write red chart/state tests for numeric/null/zero click eligibility,
      same-node close, other-node reuse, 300ms title-search debounce, page
      reset, retry, session cache, and stale response ownership.
- [x] 4.2 Implement the drill-down state and API client without duplicating
      trend predicate logic or persisting JD bodies/IDs in browser storage.
- [x] 4.3 Write red component/browser tests for desktop node anchoring,
      viewport flip, fixed skeleton, mismatch notice, detail close context,
      and no browser-history entries.
- [x] 4.4 Implement the desktop popover, mobile 390px bottom sheet, drag snap,
      overlay/scroll lock, Escape/backdrop/system-back close, and safe-area
      styling with scoped CSS and existing theme tokens.
- [x] 4.5 Render only title, status, and allowed actions; verify IDs, versions,
      parser/dictionary details, audit states, and internal errors never enter
      visible DOM or user-facing tooltip text.

## 5. Verification and measured rollout

- [x] 5.1 Run focused Go/Vitest/browser tests, type-check, build, and
      `git diff --check`; record skipped integration checks explicitly when
      `TEST_DATABASE_URL` is unavailable.
- [x] 5.2 Run `openspec validate extend-job-trend-drilldown-and-custom-range --strict`
      and resolve every finding.
- [x] 5.3 Measure custom-range and one-node drill-down p95 on retained CVM
      data using aggregate/list-only checks. Verify 100ms skeleton, 1s list,
      and 1.5s detail budgets before considering any index.
- [x] 5.4 Verify desktop and 390px views through the fixed
      `http://127.0.0.1:5175/` entry, including zero/null nodes, all statuses,
      search, pagination, historical-unavailable, retry, and context return.
- [x] 5.5 Confirm the installed release after tests and performance evidence;
      future releases use the reviewed local self-deployment workflow rather
      than retired GitHub CI/CD. Update this change, validation notes, and the
      decision record with exact results.
