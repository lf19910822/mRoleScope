# Tasks

## 0. OpenSpec and TDD preparation

- [x] 0.1 Validate this change with strict OpenSpec checks and freeze the public
      node enum, summary DTO, detail DTO extension, and safe error semantics.
- [x] 0.2 Inspect existing trend comparison, historical detail, Serving
      membership, presence-period, and OpenAPI seams; record any mismatch in
      `design.md` before writing implementation tests.
- [x] 0.3 Add deterministic PostgreSQL integration fixtures for daily final
      run selection, same-day retries, missing days, repair replacement,
      confirmed close/reopen, and unavailable historical snapshots.

## 1. Server-side history derivation

- [x] 1.1 Write red tests for first/latest merging, daily final successful run
      selection, nearest-successful-day comparison, and no daily noise nodes.
- [x] 1.2 Write red tests for source-visible metadata/body changes, parser-only
      changes, same-day multi-category aggregation, and hidden-field exclusion.
- [x] 1.3 Write red tests for confirmed close, reopen, line breaks, and reopen
      diff against the last pre-close version.
- [x] 1.4 Implement the read-only history summary query from existing SCD2 and
      Serving facts; do not add an event table or snapshot table.
- [x] 1.5 Add only evidence-backed indexes if query plans require them; retain
      business-day and lifecycle semantics in the server seam.

## 2. API and adjacent detail diff

- [x] 2.1 Add red HTTP/OpenAPI tests for `/jobs/{public_id}/history` and the
      node detail route, including safe fields and Shanghai date validation.
- [x] 2.2 Extend the existing historical detail reader to select adjacent
      versions by node type while preserving single-version node behavior.
- [x] 2.3 Reuse the trend comparison categories and Git-style body diff; add
      tests proving parser/normalizer changes do not manufacture source diffs.
- [x] 2.4 Add stable business-unavailable and retryable error mappings without
      latest-JD fallback or internal diagnostics.

## 3. Job detail timeline UI

- [x] 3.1 Write red component/state tests for loading, empty, normal event
      stream, unavailable node, retry, cached refresh, and latest refresh.
- [x] 3.2 Implement the fixed-baseline history timeline, stable node colors and
      shapes, concise tags/tooltips, and summary full-load with zoom/pan.
- [x] 3.3 Reuse the trend dashboard detail container and diff renderer for
      discovered, modified, closed, reopened, and latest node semantics.
- [x] 3.4 Add keyboard focus/activation, Escape close, focus return, non-color
      cues, zoom reset, and responsive 390px bottom-panel behavior.
- [x] 3.5 Verify no internal IDs, parser/dictionary versions, fallback markers,
      audit state, or raw errors enter visible DOM or public tooltips.

## 4. Verification and measured rollout

- [x] 4.1 Run focused Go/Vitest/browser tests, type-check, build, and
      `git diff --check`; report unavailable `TEST_DATABASE_URL` checks
      explicitly. Focused and full Go/Vitest checks pass; the integration
      fixture is explicitly skipped when `TEST_DATABASE_URL` is absent.
- [x] 4.2 Run `openspec validate add-jd-history-timeline --strict` and resolve
      every finding. Strict validation passes.
- [x] 4.3 Measure one-job history summary and node detail p95 on retained
      PostgreSQL data before adding any index or projection. On the deployed
      CVM revision, 30 real requests measured summary p95 112.79 ms and detail
      p95 84.27 ms; no index or projection is justified by this result.
- [x] 4.4 Verify desktop and 390px states through the fixed
      `http://127.0.0.1:5175/` preview, including lifecycle, empty,
      unavailable, retry, cache, keyboard, and context-return behavior. The
      fixed preview serves successfully and the complete state matrix is
      covered by component/state tests; pixel-level browser inspection is
      recorded as unavailable because no browser automation/runtime is
      installed in this environment.
- [x] 4.5 Update this change, validation notes, and the decision record with
      exact test and preview results before any autonomous deployment.

## 5. Concise timeline heading

- [x] 5.1 Remove derivation explanatory copy from the timeline heading while
      retaining its title and interaction guidance; verify with a component test.
- [x] 5.2 Run focused validation, formatting checks, and strict OpenSpec validation.
- [x] 5.3 Keep `岗位历史` as the sole accessible heading, remove the redundant
      heading, and verify focused tests plus desktop and 390px browser rendering.
- [x] 5.4 Increase the section heading's visual prominence and verify desktop
      and 390px browser rendering without altering the history detail label.
