# Tasks

## 1. TDD contracts

- [x] 1.1 Add application-shell tests for the desktop navigation launcher, collapse action, route preservation, and mobile navigation regression.
- [x] 1.2 Add trend-view tests for a universally available configuration launcher, close action, focus restoration, and zero reserved right rail space when closed.

## 2. Implement independent panels

- [x] 2.1 Refactor `App.vue` so the desktop navigation is a fixed on-demand panel with a persistent launcher and no reserved grid column.
- [x] 2.2 Refactor `JobTrendView.vue` so curve configuration is a fixed on-demand panel at every viewport, with desktop and mobile-specific launcher placement.
- [x] 2.3 Update trend overlay bounds and responsive CSS without changing chart, event, drilldown, or detail semantics.

## 3. Validate

- [x] 3.1 Run focused application-shell and trend-view unit tests, then the complete frontend unit suite and type check.
- [x] 3.2 Run production build, `git diff --check`, and strict OpenSpec validation.
- [x] 3.3 Run browser acceptance at 1440px, 1100px, 900px, and 390px; record geometry and interaction evidence.
