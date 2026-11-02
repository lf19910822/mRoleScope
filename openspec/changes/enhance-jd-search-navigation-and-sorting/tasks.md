# Tasks

## 1. Serving query interface and compatibility

- [x] 1.1 Write failing HTTP and repository/integration tests for one unified
  `job_family` filter matching both primary and other job-family hits, while
  proving legacy main/other parameters retain their existing behavior.
- [x] 1.2 Add the bounded ordered sort-rule interface, repeated query parsing,
  safe rejection of duplicate/mixed/unknown rules, and OpenAPI contract;
  verify legacy single-sort URLs and all invalid-query tests pass.
- [x] 1.3 Implement parameterized complete-match filtering and fixed-fragment
  multi-level education/experience ordering before pagination; verify
  null-last behavior, stable page boundaries, and no raw client sort text in
  generated SQL through focused integration tests.
- [x] 1.4 Make filter options expose one deduplicated job-family vocabulary for
  the new browser; verify it includes a primary and an other-family-only
  candidate without exposing internal IDs.
- [x] 1.5 Run and record representative compound-filter and two-rule
  `EXPLAIN (ANALYZE, BUFFERS)` checks against deployment-scale data; add a
  Serving-only reverse match index only when the plan demonstrates need, and
  verify no source or AI relation enters the plan.

## 2. URL-backed list interaction

- [x] 2.1 Write failing Vue state/API-client tests for repeated ordered sort
  values and the unified `job_family` filter, including copied-URL
  round-tripping and legacy query interpretation where retained.
- [x] 2.2 Replace the two family selectors with one searchable `岗位族`
  selector and implement an accessible add/remove/reorder sort-rule builder;
  verify repeated selections are prevented and the emitted request priority is
  visible in component tests.
- [x] 2.3 Refine list-route scroll behavior so in-list filter, reset, and page
  state changes preserve the viewport while detail navigation and browser-back
  restoration still work; verify both behaviors in router tests.
- [x] 2.4 Update the frontend delivery guide's filter and ordering vocabulary;
  verify it describes unified family matching, priority sorting, and keeps
  primary/other distinction only as returned-JD presentation.

## 3. Application navigation shell

- [x] 3.1 Write failing application-shell tests for the one released
  `岗位筛选` destination, its active state on list and detail routes, and the
  absence of placeholder future destinations.
- [x] 3.2 Implement the Ocean Breeze responsive application shell with a
  desktop left sider, an accessible narrow-viewport menu/overlay, and
  data-driven released destinations; verify desktop and narrow navigation
  flows in component tests.
- [x] 3.3 Review the shell and list layouts at desktop and narrow viewport
  widths for content width, keyboard focus, reduced motion, and closed-menu
  non-obstruction; record the verification in the delivery guide.

## 4. Integrated verification

- [x] 4.1 Run focused Go unit/integration, Vue unit/router/browser-flow,
  OpenAPI/static, frontend build, `openspec validate
  enhance-jd-search-navigation-and-sorting --strict`, and `git diff --check`
  checks; verify failures do not alter crawler, ODS, AI, or immutable formal
  JD-modeling behavior.

## 5. Threshold filtering, scalar sort expansion, and filter-page refinement

- [x] 5.1 Write failing HTTP and Serving integration tests for the bounded
  `minimum_experience_year` threshold and expanded scalar sort values;
  verify configured-only threshold semantics, null-last requirement sorts,
  stable tie breaks, and safe invalid-query rejection.
- [x] 5.2 Extend the bounded Serving list-query interface, HTTP/OpenAPI
  contract, and fixed SQL fragments for the threshold and approved scalar
  sort kinds; preserve the compatible exact experience and legacy sort forms.
- [x] 5.3 Write failing Vue URL/API and component interaction tests proving
  the fixed 0–9 threshold, unrestricted state, independent sort-kind and
  sort-direction controls, two-rule priority ordering, and exclusion of
  multi-value sort kinds.
- [x] 5.4 Recompose the filter page around grouped conditions and a quieter
  sort builder, then update the frontend delivery guide and verify desktop
  and narrow layouts, keyboard focus, and no in-list scroll reset.
- [ ] 5.5 Run focused Go and Vue suites, type-check, production build,
  OpenSpec strict validation, and diff checks; run a real-data localhost
  smoke request through the Vite proxy without opening a public listener.

## 6. Concise controls, four-level sorting, and numbered pagination

- [x] 6.1 Update the interaction contract, OpenAPI, Serving validation, and
  focused Go tests so an ordered request may contain at most four unique sort
  rules while invalid fifth or duplicate rules remain safely rejected.
- [x] 6.2 Replace the threshold button row with an ordinary `不限／1–9 年及以上`
  select, remove system-oriented helper copy from the public list page, and
  prove component tests do not render the removed wording or a zero-year
  browser choice.
- [x] 6.3 Replace previous/next-only pagination with URL-backed numbered
  pages, bounded page-size selection, and direct page jump. Verify generated
  requests use only `page` and `page_size`, preserve filters and sort rules,
  and do not contain a cursor parameter.
- [x] 6.4 Run focused Go and Vue suites, type-check, production build,
  OpenSpec strict validation, and desktop/390px visual checks against the
  private localhost preview.

## Workflow follow-up

- Review the completed change before deployment.
- Deploy the compatible Go image before the browser image; retain the existing
  domain/public-release gate from `add-jd-search-serving-ui`.
- Archive this change after the project's review requirements are satisfied.
