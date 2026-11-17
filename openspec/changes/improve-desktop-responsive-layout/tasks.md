# Tasks

## 1. Responsive behavior contract and red tests

- [x] 1.1 Add source-level layout contract tests for the four breakpoint modes, compact desktop configuration trigger, and zero reserved trend rail space; verify the tests fail against the current two-breakpoint implementation.
- [x] 1.2 Add browser geometry checks at 1440, 1280, 1100, 900, and 390 CSS pixel widths for chart workspace bounds, rail/trigger visibility, no horizontal overflow, and mobile configuration access; verify the checks fail for the current 1100/900 layouts.

## 2. Desktop shell and trend layout implementation

- [x] 2.1 Implement wide and standard desktop tokens and breakpoints without changing the wide-screen persistent rail behavior; verify the 1440 and 1280 geometry checks preserve the established split layout.
- [x] 2.2 Implement compact desktop on-demand configuration mode using the existing curve editor, close control, scroll container, and footer actions; verify 1100 and 900 geometry checks show a full-width chart workspace and an operable drawer trigger.
- [x] 2.3 Adjust compact/standard chart, event table, card padding, typography, and detail/drilldown bounds while preserving the existing ocean-breeze visual language; verify no horizontal overflow and readable control geometry across the matrix.
- [x] 2.4 Adjust shared desktop navigation width only where needed to protect the trend workspace, keeping route labels, active states, and mobile navigation behavior unchanged; verify existing `AppShell` tests and navigation behavior pass.

## 3. Responsive regression coverage and documentation

- [x] 3.1 Add TDD coverage for opening and closing compact desktop configuration, breakpoint transition while open, curve list scrolling, footer reachability, and keyboard focus; verify the focused front-end tests pass.
- [x] 3.2 Run the 390px mobile acceptance matrix and confirm navigation, configuration sheet, trend table, drilldown, and detail overlay remain usable; record any environment limitations without weakening the contract.
- [x] 3.3 Update the change validation record with browser evidence, viewport dimensions, commands, and known native-platform gaps; verify `openspec validate improve-desktop-responsive-layout --strict` passes.

## 4. Final quality gates

- [x] 4.1 Run the complete frontend unit suite and type check; verify all tests pass.
- [x] 4.2 Run the production frontend build and `git diff --check`; verify the bundle builds without layout-related errors.

## Workflow follow-up

- Complete code review and merge only after responsive browser acceptance passes.
- Deploy only after a separate explicit deployment authorization.
