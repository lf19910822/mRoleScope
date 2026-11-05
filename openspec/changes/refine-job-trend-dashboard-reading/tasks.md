# Tasks

## 0. Contract and TDD preparation

- [x] 0.1 Confirm the router, chart-presentation, dashboard-view, and
      responsive-style seams in `design.md` before the first test.
- [x] 0.2 Inspect the existing trend state, chart option factory, route,
      component test conventions, and Ocean Breeze shell before selecting
      fixtures.

## 1. Canonical entry and observation identity

- [x] 1.1 Add a red router test for `/trends` resolving to the canonical
      dashboard route; add the minimum redirect and verify green.
- [x] 1.2 Add a red chart-presentation test for stable row colour and concise
      legend formatting when sibling rows are pending; implement and verify
      green.

## 2. Honest at-a-glance reading

- [x] 2.1 Add a red chart-presentation test for latest numeric observation,
      prior-numeric delta, zero, and all-gap behaviour; implement and verify
      green.
- [x] 2.2 Add a red mounted-view test for the one-action first-curve flow,
      compact copy, and refresh visibility; implement and verify green.

## 3. Interaction and responsive finish

- [x] 3.1 Add a red view/style regression for all-history gesture guidance,
      removal of point-click feedback, mobile safe-area spacing, and dark
      rail tokens; implement and verify green.
- [x] 3.2 Verify desktop and 390px configured/empty states through the fixed
      5175 entry, including eight curve markers, legend readability, zero,
      gap, and dark theme.

## 4. Verification and delivery

- [x] 4.1 Run focused and complete frontend unit suites, type check, build,
      strict OpenSpec validation, and `git diff --check`.
- [x] 4.2 Update tasks and record the exact visual and automated validation
      result before commit, PR, merge, and CVM deployment.
