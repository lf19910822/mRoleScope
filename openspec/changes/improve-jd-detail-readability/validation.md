# Validation notes

## Production-safe range audit — 2026-11-02

The query ran through `root@118.89.68.77` inside the private PostgreSQL
container. It was aggregate-only and read-only: it did not select JD text,
aliases, identifiers, connection strings, or credentials, and it made no
mutation.

| Measure                                                                        | Count |
| ------------------------------------------------------------------------------ | ----: |
| Same-current-body technology evidence ranges                                   |   876 |
| In-bounds ranges                                                               |   876 |
| Duplicate `(source_field,start,end)` tuples                                    |   135 |
| Valid rows starting inside a preceding interval                                |   789 |
| Ranges retained by the serving sanitizer's deterministic non-overlap selection |   199 |

The serving behavior is intentionally best effort: a duplicate or overlap is
omitted from visual emphasis only. The JD body, its structured technology
tags, and unrelated data remain available.

## Local verification status

- Passed `go test ./internal/jobsync ./internal/jdserving ./internal/httpserver`
  and `go test ./...`.
- Passed `python3 -m unittest collector.test_collector` (6 tests).
- Passed `npm run test:unit -- --run` from `web/` (16 files, 48 tests) and
  `npm run build` from `web/`.
- Passed scoped Prettier and ESLint checks for every changed/new front-end
  file. Pre-existing unrelated formatting findings were not rewritten.
- Passed `openspec validate improve-jd-detail-readability --strict` and
  `git diff --check`.

The newly written migrated-PostgreSQL scenario is included but was skipped
locally because `TEST_DATABASE_URL` is not configured; the scoped CVM audit
above validates the production data shape without mutating it.

The local checks did not include an interactive browser viewport review. That
visual acceptance remains a manual product-review activity; it is not
represented as a completed automated validation task.

## Immutable-image deployment and private API acceptance — 2026-11-02

GitHub Actions run
[`41454559426`](https://github.com/lf19910822/mRoleScope/actions/runs/41454559426)
completed successfully for main commit `c98ea22f`. It ran the build, test,
network-boundary, and web-edge-boundary checks; pushed the four runtime images
by digest; then invoked the CVM's root-owned allowlist deployment command.

Post-deployment, the CVM API and PostgreSQL Compose services both reported
healthy. A short-lived, read-only, capability-dropped Python container joined
only the existing private backend network and read one current API list entry
and its detail. It printed only this structural summary:

| Check                          |                         Result |
| ------------------------------ | -----------------------------: |
| List entries read              |                              1 |
| Non-empty JD body sections     |                              3 |
| Technology highlight intervals |                              2 |
| Public highlight fields        | `section`, `start`, `end` only |

The acceptance command did not print or persist JD text, public IDs,
credentials, connection strings, aliases, provenance, or internal processing
metadata. The temporary container was removed automatically.

## Production floating-island interaction — 2026-11-03

The selected pointer interaction was moved from the development-only visual
prototype into `JDSectionList`. Focused component tests verify that mouse
movement applies the bounded `1.05` scale and `±4deg` edge tilt, reset restores
the resting variables, and touch input does not activate the interaction.
The application has no `prefers-reduced-motion` overrides: the selected card
interaction, skeleton shimmer, and request-loader pulse remain active under
either host motion preference. A local Chromium matrix check loaded the actual
Vite component at the fixed `127.0.0.1:5175` entry in both normal and
reduced-motion modes. In both modes a mouse hover produced an active row,
`1.05` scale, bounded tilt, a non-`none` `matrix3d(...)` transform, and the
intended `0.36s` transform transition. The pending manual desktop and 390px
visual review remains tracked separately in task 2.5.

## Resting text compositing check — 2026-11-03

The floating-island component now confines 3D compositing to a pointer-active
row. A local Chromium check loaded the actual Vite component at the fixed
`127.0.0.1:5175` entry in normal and reduced-motion browser modes. In both
modes, an inactive row reported `transform: none`, `will-change: auto`, and
no marker or copy transform; the same row reported a non-`none` 3D matrix and
`will-change: transform` after mouse activation. The focused component
regression test protects the inactive rendering contract.

## Parenthesized numeric-requirement regression — 2026-11-03

The body presentation parser now treats a closing-parenthesis Arabic ordinal
as a valid marker when the source requirement immediately begins with a
number. The focused regression test failed before the grammar change and then
passed after it, verifying that the list is rendered as three ordered source
items. Existing tests retain protection against decimal/version fragments and
hyphenated technology names.

- Passed `npm run test:unit -- --run` from `web/` (17 files, 53 tests).
- Passed `npm run type-check`, `npm run build-only`, scoped Prettier and ESLint
  checks, and `openspec validate improve-jd-detail-readability --strict`.

## Enumeration-comma numeric-requirement regression — 2026-11-03

The presentation grammar now accepts an enumeration-comma ordinal whose
requirement begins with a number. This keeps the first requirement in a list
such as `1、3年以上…；2、…` aligned with later numbered source-item surfaces,
rather than presenting it as lead text. Decimal and version protection remains
limited to dot-style markers.

- Passed `npm run test:unit -- --run` from `web/` (17 files, 55 tests).
- Passed `npm run type-check`, `npm run build-only`, scoped Prettier and ESLint
  checks, and `openspec validate improve-jd-detail-readability --strict`.

## Compact technology-keyword disclosure — 2026-11-03

The detail and development-only prototype now use the same Ocean Breeze
secondary disclosure button for dense technology sets. It reads `+ 展开` when
collapsed and `− 收起` when expanded, keeps the existing ARIA state and
controlled-collection relationship, and no longer exposes a keyword count or
implementation-oriented wording. The two-row preview and button now behave
consistently at every viewport width.

- Passed focused detail and prototype disclosure tests.
- Passed `npm run test:unit -- --run` from `web/` (17 files, 55 tests),
  `npm run type-check`, `npm run build-only`, scoped Prettier and ESLint
  checks, and `openspec validate improve-jd-detail-readability --strict`.

## Source-proven ordinal subgroups — 2026-11-03

The body presentation grammar now supports a restart from `1` only when an
audited source grouping label of `基础` or `加分项` immediately precedes it. The
renderer preserves that label as a source heading and emits a separate ordered
list for its items; text before the heading remains with the preceding item
only up to the heading boundary. A reset with no recognized heading remains a
lossless fallback rather than an inferred group.

- Passed focused parser and `JDSectionList` regressions for labeled groups and
  unlabeled-reset fallback.
- Passed `npm run test:unit -- --run` from `web/` (17 files, 58 tests),
  `npm run type-check`, `npm run build-only`, scoped Prettier and ESLint
  checks, `openspec validate improve-jd-detail-readability --strict`, and
  `git diff --check`.

## Single ordinal source-item regression — 2026-11-03

The presentation grammar now renders a single unambiguous top-level Arabic or
Chinese ordinal as one ordered source item. This lets a source section with
one enumerated requirement use the same readable card treatment as a longer
numbered list, while a lone bullet remains conservative fallback prose.

- Passed `npm run test:unit -- --run` from `web/` (17 files, 54 tests).
- Passed `npm run type-check`, `npm run build-only`, scoped Prettier and ESLint
  checks, and `openspec validate improve-jd-detail-readability --strict`.

## Explicit source group with skipped ordinal — 2026-11-03

A live-detail debugging loop established that the affected normalized
requirements text contains source-proven `基础` and `加分项` groups. The first
group's markers are consecutive, while the second begins at `1` and then
continues at `3`. The prior conservative parser treated that source numbering
gap as an ambiguous section and returned one fallback block.

The presentation grammar now permits a strictly increasing skipped marker only
inside a recognized source group that begins at `1`. It preserves the source's
visible `1` and `3` markers and never manufactures the missing number. The
same skipped sequence without a recognized source heading remains fallback
text. A focused unit regression failed before this grammar change and passed
after it. The original local-detail replay now returns a list with two source
headings and four ordered items; it printed only kinds, source markers, and
offset/length metadata, never JD content or credentials.

- Passed focused `jd-body-presentation` unit tests (11 tests).
- Passed `npm run test:unit -- --run` from `web/` (17 files, 60 tests).
- Passed `npm run type-check`, `npm run build-only`, scoped Prettier checks,
  `openspec validate improve-jd-detail-readability --strict`, and
  `git diff --check`.

## Compact source-marker and nested-hyphen regressions — 2026-11-03

The body presentation grammar now accepts one horizontal source whitespace
between an Arabic numeral and its dot or full-width dot. The dot must still
not be followed by a digit, so both ordinary and spaced decimal/version forms
remain source prose. The item keeps its original interval and visible numeral;
the reader-facing ring extracts only that numeral.

Within an already proven ordinal source item, two or more compact ASCII hyphen
markers at valid list boundaries now form a nested semantic bullet list. The
parent retains the source preamble before its first child, and child intervals
preserve their source order for safe rendering and existing highlight ranges.
A lone compact hyphen and hyphenated terms remain ordinary prose.

A scoped, read-only aggregate audit of the current 178-entry Serving inventory
found one JD with the spaced numeric-dot pattern. It found two responsibility
sections containing at least two compact-hyphen candidates, with 12 candidate
markers in total. The audit returned aggregate counts only; it neither read nor
recorded JD text, titles, public IDs, or credentials.

The affected local-detail replay now yields five ordered requirement entries
across its two source groups and two ordered responsibilities with three nested
hyphen entries each. The replay printed only structure and source-marker
metadata, never JD content.

- Passed focused parser and `JDSectionList` tests (21 tests).
- Passed scoped Prettier and ESLint checks.
- Passed `npm run test:unit -- --run` from `web/` (17 files, 64 tests),
  `npm run type-check`, `npm run build-only`,
  `openspec validate improve-jd-detail-readability --strict`, and
  `git diff --check`.
- Confirmed the fixed local Vite entry at `127.0.0.1:5175` serves both the
  numeric-dot and nested-child rendering branches.

## Guaranteed technology disclosure and static source items — 2026-11-03

Two focused regressions were written and run red before the implementation:
an over-eight keyword collection rendered all nine labels despite presenting
`+ 展开`, and a mouse pointer move activated the source-item floating effect.

The detail view now renders at most the first eight technology labels while
collapsed, in addition to retaining the existing two-row guard. Selecting
`+ 展开` renders the full collection, and `− 收起` restores the eight-label
preview. This makes the disclosure useful even when short labels fit within
two wide desktop rows.

`JDSectionList` no longer installs pointer handlers or uses scale, tilt,
pointer-tracked glow, 3D-compositing, or transform-transition styles. The
change is confined to detail-body source items; the independent job-list card
hover treatment is unchanged.

- Passed focused `JobPresentation` and `JDSectionList` tests (10 tests).
- Passed `npm run test:unit -- --run` from `web/` (17 files, 72 tests).
- Passed `npm run build` from `web/` (type check and Vite build).
- Passed `openspec validate improve-jd-detail-readability --strict` and
  `git diff --check`.

## Numeric-leading interior ordinal regression — 2026-11-03

The reported Unity-client JD exposed a source-list shape that the prior
numeric-dot grammar reduced to a fallback paragraph: its continuous `1` to
`6` ordinal sequence contains interior items whose body starts with a digit.
The parser now records every valid-boundary numeric-dot candidate, marks only
the digit-following form as ambiguous, and accepts an ambiguous run only when
unambiguous source markers bound it on both sides within the existing
consecutive-sequence validation. This is structural evidence only; it does
not classify or maintain a whitelist of the following requirement text.

The first focused parser regression was run red against the reported
requirements shape and returned `fallback`; after the smallest parser change
it returns six original-marker ordered items. Regressions cover the reported
supplemental shape, non-year numeric-leading prose (`3D`), and an edge decimal
candidate that must remain fallback. The component regression confirms the
reported requirements render six semantic source-item cards.

- Passed focused parser and `JDSectionList` tests (24 tests).
- Passed scoped Prettier and ESLint checks.
- Passed `npm run test:unit -- --run` from `web/` (18 files, 80 tests) and
  `npm run build` (type check and Vite build).
- Passed `openspec validate improve-jd-detail-readability --strict` and
  `git diff --check`.
