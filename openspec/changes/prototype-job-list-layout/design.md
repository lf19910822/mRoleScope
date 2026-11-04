# Design

## Question

Which three-column search workspace lets a JD reader compare results and alter
query controls without losing either context?

This direction is now the production `/jobs` layout. It receives the existing
loaded list response, so titles, counts, locations, and tags remain Serving
facts rather than fixture content. Its right query rail keeps every existing
filter, sorting, reset, and apply control bound to the same URL/API state.

## Shared frame

All variants retain the Ocean Breeze baseline and the task's true information
hierarchy:

```
fixed function rail | document-scrolling role collection | fixed query rail
```

| Token     | Value     | Purpose                            |
| --------- | --------- | ---------------------------------- |
| Deep Ink  | `#121c22` | Results, titles, navigation labels |
| Sky Field | `#e8f2f8` | Quiet page ground                  |
| Sea Glass | `#eaf7fa` | Query controls and active surfaces |
| Tide Blue | `#007bab` | Current state and readable anchors |
| Lagoon    | `#008287` | Secondary query accents            |
| Divider   | `#e3ebf0` | Structural rules                   |

The signature device is a thin "query wake": a tide-blue vertical rule and
small wave mark that align the active result region with the right query rail.
It describes a real relationship—the controls change this collection—rather
than acting as decoration.

On desktop the page uses one browser scroll context. The app shell owns a
viewport-fixed left rail; on wide screens the list route pins its 428px query
console to the viewport's far-right and top edges. The content column explicitly
reserves the console's width plus its breathing gap, so the fixed rail cannot
cover a role card. The console uses the full viewport height and scrolls
internally only when its form exceeds that height. Its normal filter arrangement
pairs concise related controls in two columns: education / experience, project
/ location, activity / urgency, and discovery-from / discovery-to. Title, job
family, and technology remain full width. The role collection itself never owns a vertical
scrollbar, so a reader scans the cards using the familiar page scroll while
both rails remain available. At intermediate widths and below 760px, the
console remains in the content layout to preserve usable reading width; at
760px and below both rails return to ordinary document flow and only the
document scrolls.

## Variants

### A — Tidal workbench

Wide, breathing result cards sit in a center reading column. A full query
atelier on the right groups conditions by what the reader knows about a role:
identity, qualifications, and publication state. It favors careful JD reading
and is the likely default for the first release.

### B — Results ledger

The center becomes a dense, ruled role ledger with compact facts aligned in
columns. The right rail is a stacked query console with visible active-query
summary. It favors scanning many JDs and comparing formal facts.

### C — Collection canvas

The center uses a broader collection canvas: result cards expose tags and
context, while the narrow right rail becomes a vertically indexed filter
library. It favors exploratory browsing and makes the available facets feel
like a reference index rather than a form.

The variants deliberately disagree about list density and filter organization,
not merely palette or copy. The existing font stack remains; the distinctive
decision is structural, not a gratuitous new typeface.

## Selected refinement — Query canvas

**Decision:** Accepted by the product owner on 2026-11-03.

The selected direction combines B's query console with C's structured result
record. The collection uses one reading column rather than two competing card
columns, so a reader can scan a role's facts and tags as one uninterrupted line
of sight. A card has three compact information bands rather than a table tile:
the first combines a small role-node glyph, the untruncated title, and the
primary job-family tag at left with its two state pills at right; the second is
a wrapping, inline four-fact overview (location, experience, education, first
discovery); the third is a complete wrapping technology-keyword row. Project
labels are rendered only as exact, bold bright-yellow highlighted spans within
the title that deterministically supplied them, so they do not duplicate the
title as tags.
Other matched job families remain visible in an optional supplementary row
labelled “副岗位族”.
When a role has no technology keywords, that entire band is absent: the facts
end directly, or meet the supplementary row through a single adjacent divider
rather than an empty, double-ruled gap.
Activity and urgency are represented once only by their top-line pills,
including the ordinary non-urgent label; duplicating them in the facts would
make the compact record unnecessarily tall.

The role-node glyph is the card's single signature device. It is a quiet,
Ocean-Breeze teal network mark that identifies a role as a structured record;
it replaces the decorative top accent and does not pretend that the facts are a
sequential workflow. The card uses a shallow 1px border, 16px corner radius,
and only content-driven height. On narrow screens status pills receive their
own line before a full-width title, then the overview and keywords wrap without
data loss. Dark-scheme card colors retain the same information contrast.

The query console keeps its grouped, scan-friendly arrangement, but its groups
are filters rather than sequential steps. It therefore has no decorative
`01`–`04` labels. This selected refinement is implemented directly in the
production list page; there is no duplicate preview control in the finished
product.

The technology-keyword browser is a contained detail view of the query
console, not a viewport-sized overlay. At rail width its category column may
be narrower, but remains wide enough to keep dictionary category labels on one
line. The complete two-panel browser stays within the trigger's available width
and is never clipped at the right viewport edge. Its category and term panes,
as well as the Naive UI menus in the query console, contain their wheel scroll
chains: scrolling an option list may not pull the console or the document.
Sorting-kind and sorting-direction menus deliberately opt out of the trigger's
fixed menu width: they retain that trigger as their anchor and minimum width,
but grow to their natural option-label width. This keeps complete Chinese sort
labels readable without widening the fixed query console or disturbing its
compact paired-control layout.

## Accepted clarity refinement — research workbench

This refinement keeps the Ocean Breeze visual baseline but spends its visual
emphasis on one true relationship: controls change the collection. The result
heading therefore receives a narrow Tide Blue *query wake* instead of another
decorative wave icon. The dense, small-count result heading is the route's
thesis; the previous oversized route masthead is reduced to quiet context so a
reader reaches a live record sooner.

The desktop console distinguishes a reader's common first-pass constraints
(title, family, technology, education, experience) from contextual narrowing
and deliberate sorting. Context and sorting are collapsed until needed. This
keeps the no-sort workbench short while preserving every existing control and
query parameter. Its action bar remains visible at the bottom of the console,
so a reader never has to hunt for “应用筛选” after changing a lower control.

The collection reflects the applied query with compact display-safe chips. A
chip names a reader-recognisable condition and can remove just that condition;
the summary never exposes parser, source, or implementation state.

At phone width the result collection comes first. “筛选” opens the existing
form as a bottom sheet above a translucent page overlay; it does not create a
second filter model, change filtering semantics, or turn the role list into an
internal scroll region. Applying or resetting a query closes the sheet. The
desktop right rail remains unchanged above the narrow breakpoint.

Ocean Breeze has a complete paired dark token set. The shell declares the
active colour scheme and passes matching Naive UI theme/overrides together;
scoped route surfaces then use those tokens rather than a card-only dark
exception. Card labels, values and tags remain compact, but no longer depend
on hover magnification for legibility. Dates and counts use tabular figures
for comparison.

### Focus motion

The query canvas has one deliberately strong interaction: a hovered desktop
card becomes the active record, scaling to `1.15` above adjacent surfaces with
a `360ms` spring-like return. This temporary magnification supports reading
small structured facts without opening the detail page. It is mouse-only and
has no resting transform, `will-change`, or 3D promotion, so static Chinese
text remains crisp.

## Production boundaries

- The query console uses the same draft state and actions as the former
  top-of-page filter form. It does not create another query model.
- The fixed left function rail belongs to the app shell and is not duplicated
  in the route component.
- At desktop widths, only the query form may scroll internally; the role
  collection remains in the document's scroll flow. The left navigation is
  fixed and, on wide screens, the right query rail is fixed to the viewport's
  far-right and top edges. Both are independent of the result collection
  height.
- The matching-role count is a compact structural list heading, not a large
  decorative panel.
- The role collection is one column at every width. This preserves a single
  stable reading order while the browser page scrolls.
- At narrow widths the fixed rails are removed and the query console collapses
  into normal document flow. The result collection has one column by 390px, so
  no role card is horizontally compressed.
- No development-only URL switcher, prototype copy, or implementation detail
  appears in the final product.
