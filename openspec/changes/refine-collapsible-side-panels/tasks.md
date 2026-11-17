# Tasks

## 1. TDD contracts

- [x] 1.1 Add application-shell tests for responsive default navigation rail states, icon-only collapsed navigation, top control placement, and no persistence writes.
- [x] 1.2 Add trend-view tests for responsive default curve rail states, color marker expansion/focus, removal of the wide trigger, and no persistence writes.
- [x] 1.3 Add reduced-motion and accessibility contracts for the rail controls.

## 2. Implement layout rails

- [x] 2.1 Refactor `App.vue` into an in-flow desktop shell with a persistent collapsed rail and responsive expanded defaults.
- [x] 2.2 Replace text glyph navigation with distinct accessible icon components and top rail controls.
- [x] 2.3 Refactor `JobTrendView.vue` into an in-flow right rail with curve color markers and responsive expanded defaults.
- [x] 2.4 Add coordinated rail/content transitions, visibility states, focus restoration, and reduced-motion behavior.
- [x] 2.5 Preserve mobile drawer/sheet behavior and existing trend, drilldown, and detail semantics.

## 3. Validate

- [x] 3.1 Run focused and full frontend tests, type check, build, and `git diff --check`.
- [x] 3.2 Run strict OpenSpec validation.
- [x] 3.3 Perform browser acceptance at `1440px`, `1280px`, `1100px`, `900px`, and `390px`, including rail interactions and no horizontal overflow.

## 4. Follow-up rail refinements

- [x] 4.1 Remove duplicate desktop rail controls; reopen the collapsed navigation from its brand icon and retain only the expanded curve-rail close control.
- [x] 4.2 Keep curve markers fixed at `32px` rows, remove the medium-desktop fixed rail width, and apply rounded rail surfaces.
- [x] 4.3 Keep desktop rail surfaces mounted during transitions and animate visibility, opacity, and transform while preserving mobile panel behavior.
- [x] 4.4 Add regression coverage for collapsed-brand reopening, single close control, fixed marker geometry, medium-desktop width behavior, and mounted transition surfaces.
- [x] 4.5 Re-run focused/full frontend validation, strict OpenSpec validation, and responsive browser acceptance.
- [x] 4.6 Keep medium-desktop collapsed-rail padding compact so fixed marker slots remain centered inside the narrow rail.

## 5. Collapsed marker hover notes

- [x] 5.1 Add fixed marker slots, separators, and mounted pull-out note markup while removing native marker tooltips.
- [x] 5.2 Reuse the existing curve label formatter for concise project-and-condition note text and add delayed hover styling without changing focus semantics.
- [x] 5.3 Validate the hover-note behavior with component tests, type check, build, and desktop/mobile browser acceptance.
- [x] 5.4 Keep configuration-close focus restoration accessible without opening a marker note when the pointer is not hovering.
