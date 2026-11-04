# Tasks

## 1. Compare layouts with real list data

- [x] 1.1 Add a development-only `/jobs?prototype-layout=a|b|c` host that
      retains the existing list request and passes only loaded display-safe list
      facts into the prototype.
- [x] 1.2 Create three structurally distinct Ocean Breeze desktop layouts with
      independently fixed left navigation and right query rails, then add a
      development-only URL switcher.
- [x] 1.3 Check all three variants at desktop and 390px widths, verify the
      production build excludes the preview controls, and hand the three local
      preview URLs to the user for selection.

## 4. Production query canvas (TDD)

- [x] 4.1 Record the selected B query-canvas layout and extend this change
      with its production behavior, responsive boundary, and no-new-API scope.
- [x] 4.2 Add a red DOM-level test for the production query canvas and its
      complete structured role facts, then implement the smallest page
      rearrangement that passes it.
- [x] 4.3 Add a red interaction/style-contract test for production mouse-only
      1.15× card focus, then implement the minimal focused state and motion.
- [x] 4.4 Verify desktop and 390px with the real local Serving-backed preview;
      run the frontend test/build/format checks and strict OpenSpec validation.
- [x] 4.5 Remove the losing previews and their development-only switcher after
      the production implementation replaces them.

## 2. Selected-layout refinement

- [x] 2.1 Update the B preview into the selected query canvas: retain its
      fixed query console, use C's two-column result collection, and make the
      requested record facts visible without decorative empty space.
- [x] 2.2 Verify the revised desktop and 390px preview with current Serving
      list data, then hand the revised local URL to the user for acceptance.

## 3. Query-canvas focus motion (TDD)

- [x] 3.1 Add a failing component test for mouse-only focused-card state and
      touch-pointer non-activation.
- [x] 3.2 Add the smallest interaction implementation to satisfy the state
      test, then add a failing visual-contract test for 1.15 scale, smooth
      return, overlap layering, and sharp resting text.
- [x] 3.3 Add the minimal focused-motion CSS, run the focused tests and full
      frontend validation, and visually inspect the real-data preview.

## 5. Discarded embedded-workspace experiment

The viewport-bound, independently scrollable result workspace was rejected by
the product owner after visual review. It is superseded by section 6 and MUST
NOT be used as the production layout contract.

## 6. Global role-list scroll repair (TDD)

- [x] 6.1 Record the corrected page-scroll reading model and compact count
      heading.
- [x] 6.2 Add red regression contracts for global result scrolling, persistent
      desktop rails, and the compact result heading.
- [x] 6.3 Implement the smallest shell and query-canvas correction without
      changing list requests, filters, or pagination semantics.
- [x] 6.4 Verify desktop page scrolling and 390px normal flow with real local
      Serving data, then run full frontend validation.

## 7. Wide-desktop query rail alignment

- [x] 7.1 Record the far-right, top-aligned, viewport-height query-rail
      behavior and its content-width reservation.
- [x] 7.2 Add a regression contract for the wide-desktop fixed query rail.
- [x] 7.3 Implement and verify the rail against real local Serving data without
      changing list or filter behavior.

## 8. Single-column role reading and compact query rail (TDD)

- [x] 8.1 Record the single-column reading order, 428px wide-desktop query
      rail, and paired concise-filter arrangement without changing any query
      semantics.
- [x] 8.2 Add red layout contracts for the one-column collection, 428px rail
      reservation, and paired controls.
- [x] 8.3 Implement the compact console composition and single-column role
      collection; retain the normal browser page scroll.
- [x] 8.4 Verify the real local Serving-backed route at desktop and 390px,
      then run the relevant frontend and strict OpenSpec checks.

## 9. Contained technology browsing and compact result records (TDD)

- [x] 9.1 Record the contained keyword-browser behavior and non-duplicated
      status-presentation rule without changing query semantics or data.
- [x] 9.2 Add red contracts for a rail-contained keyword browser and a
      four-fact result card with top-line status pills.
- [x] 9.3 Implement the smallest component and card-density changes that pass
      the contracts.
- [x] 9.4 Verify the real local route with an open technology browser and
      representative cards, then run frontend and strict OpenSpec validation.

## 10. Three-band role records (TDD)

- [x] 10.1 Record the three-band card hierarchy, complete keyword presentation,
      responsive wrapping, and no-preview-control scope without changing API or
      modelling behavior.
- [x] 10.2 Add red component and style contracts for the title/status band,
      five-fact wrapping overview, full technology row, and no title clipping.
- [x] 10.3 Implement the card markup and scoped responsive/light/dark styling
      while retaining route navigation and all display-safe list fields.
- [x] 10.4 Verify real desktop and phone previews including long titles,
      multi-city locations, empty keywords, and dense keyword lists; run type,
      build, test, and strict OpenSpec validation.

## 11. Unspecified formal-requirement display

- [x] 11.1 Write a failing display-format regression, then render an absent or
      non-explicit education/work-experience requirement as `-` consistently
      across the list, detail, and development prototype without changing the
      serving data or modelling semantics.

## 12. Readable sorting menus

- [x] 12.1 Write a failing sorting-menu regression, then let sorting-kind and
      sorting-direction menus expand to their option-label width while keeping
      the right query console and its request behavior unchanged.

## 13. Title-first role identity

- [x] 13.1 Write failing presentation regressions, then move the primary
      job-family tag beside the role title and render deterministic project
      labels only as exact highlighted title spans; remove standalone project
      title tags without changing Serving data or project filtering.
- [x] 13.2 Give deterministic title-project spans a bright-yellow, bold
      highlighter treatment in both light and dark schemes without altering
      title text, project filtering, or serving data.

## 14. Research-workbench clarity (TDD)

- [x] 14.1 Add red user-facing regressions for compact workbench context,
      display-safe applied-query summary, and mobile filter-sheet open/close
      interactions; add a shell-level colour-scheme seam.
- [x] 14.2 Implement the smallest summary, collapsible desktop console, sticky
      actions, and mobile bottom-sheet behavior without changing query state,
      Serving calls, or pagination.
- [x] 14.3 Complete paired Ocean Breeze light/dark application tokens and
      scoped workbench styling, including readable resting metadata and one
      query-wake signature.
- [x] 14.4 Verify desktop, 390px, light and dark previews; run the relevant
      frontend tests, type check, build, `git diff --check`, and strict
      OpenSpec validation.
