# Proposal

## Why

The current JD search page places its complete filter form above the result
list. A visitor scanning lower results cannot see or adjust the active query
without leaving the reading position, and the application-navigation rail does
not yet establish the intended fixed reading frame.

The three desktop arrangements have now been compared against real Serving
list data. The product owner selected the query canvas: B's right-side query
console combined with C's result collection. The collection was subsequently
refined into a single reading column so each role can be scanned without
splitting attention between neighbouring cards.

The accepted production review found a second-order problem: the page still
behaves a little like a landing page and a long form rather than a focused
research workbench. Its large route heading delays the first record, the
desktop console can bury its submit action beneath low-frequency controls and
sorting, and a phone reader meets the full form before any role. The existing
dark-scheme card override also has no matching application theme.

## What Changes

- Replace the production `/jobs` layout with the selected query canvas. It
  keeps the existing real Serving list request, URL query state, filter
  semantics, sort priority, and page/page-size pagination.
- Use the browser's normal page scroll for the single-column result collection.
  The application's existing left function navigation and the real, operable
  right query console remain visible in independent desktop rails; neither is
  constrained to the result collection's height.
- Show each role as a compact structured record: location, title, job family,
  experience, education, and first-discovered date. Activity state and urgency
  remain visible as the single pair of top-line status pills, rather than being
  repeated in the fact grid.
- Reshape each role record into three compact, responsive information bands:
  title with a small role glyph, primary job-family tag, and status pills; a
  wrapping four-fact overview of location, experience, education, and first
  discovery; and a complete, wrapping technology-keyword row. Emphasize
  project labels only over their exact deterministic title matches with a
  bright-yellow, bold highlighter treatment, rather than repeat them as title
  tags, and retain other job families as an optional
  supplementary row labelled “副岗位族”. Titles and keyword lists are never
  truncated or collapsed behind a `+N` affordance.
- Give desktop mouse users a temporary 1.15× role-card magnification for
  reading compact facts. It layers above nearby content and returns smoothly;
  static cards remain untransformed for crisp text. Touch does not activate it.
- Present the matching-role count as a compact list heading rather than a
  large standalone panel that competes with the roles themselves.
- On wide desktop screens, pin a 428px query console to the viewport's far
  right and top edges. Reserve its width from the result workspace and use the
  full viewport height to expose as many query controls as possible.
- Pair concise related controls in the console—education with experience,
  project with location, activity with urgency, and the two discovery dates—so
  its ordinary, no-sort state fits in a desktop viewport. Identity and
  technology controls remain full width. Existing sorting controls and filter
  semantics are unchanged. The technology-keyword browser adapts to the rail
  width, so its complete category and term panels never overflow or become
  clipped by the viewport.
- At a 390px narrow viewport, collapse the result collection to one column and
  show the collection before filtering. The complete query console opens only
  from the visible “筛选” trigger as a contained bottom sheet.
- Replace the route's decorative landing-page masthead with compact workbench
  context. Show active query conditions as display-safe chips alongside the
  collection rather than making a reader infer them from compact controls.
- Keep identity and qualification filters immediately visible in the desktop
  rail. Place lower-frequency contextual filters and multi-rule sorting in
  collapsed sections; keep submit/reset anchored at the rail bottom while the
  form scrolls.
- Complete Ocean Breeze's dark appearance for the application shell and Naive
  controls, rather than darkening only result cards. Raise resting-card
  metadata to a readable compact size and use tabular figures for dates/counts.
- Use one signature device: a narrow Tide Blue “query wake” beside the result
  heading. Remove repeated decorative wave marks from the page and console.
- Keep right-console option menus and both technology-browser panes contained:
  wheel input may scroll their own option lists, but reaching an edge must not
  move the page or the query console. The technology category pane is wide
  enough for its dictionary category labels to stay on one line.
- Remove the losing previews and the development-only switcher rather than
  shipping a production route toggle.

## Impact

- Vue list-page layout and its focused interaction tests.
- No Serving API, router contract, data model, filter semantics, persistent
  state, deployment listener, mock data, or additional port changes.
