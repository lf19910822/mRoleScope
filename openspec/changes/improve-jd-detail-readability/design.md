# Design

## Context

See [proposal.md](proposal.md) for motivation and the three capability specs
for the behavioral contract. The current detail endpoint returns three
normalized strings and formal technology labels. It does not expose body
evidence to the browser. `JobDetailView.vue` consequently renders each
section in one `pre-wrap` paragraph and each view owns a text-only loading
branch.

The production read-only audit on 2026-11-02 establishes the shape to support:

| Current serving inventory (176 JDs)                 | Responsibilities | Requirements | Supplemental |
| --------------------------------------------------- | ---------------: | -----------: | -----------: |
| Non-empty sections                                  |              176 |          176 |          101 |
| Safely listable with repeated ordinal/bullet marker |              172 |          172 |           86 |
| No recognized delimiter (fallback)                  |                4 |            4 |           13 |

Normalization has collapsed line breaks in this corpus: list markers are
therefore commonly inline rather than line-leading. The scoped production
read on 2026-11-02 found 876 same-body technology-evidence ranges, all within
their returned section bounds. It also found 135 duplicate
`(source_field,start,end)` tuples and 789 valid rows whose start fell inside a
prior row's interval; the serving sanitizer's deterministic greedy selection
would retain 199 non-overlapping intervals. This is an audit fact, not a
front-end condition: the implementation must still validate every range
defensively and omit unsafe coordinate records rather than block a detail.

The page serves a reader who needs to scan a real recruitment brief quickly.
The selected visual direction is a restrained **floating-island ledger**:
each genuine source-list item receives one reading-sized Sea Glass surface,
instead of becoming a dense paragraph or a generic dashboard card stack. It
extends the supplied Ocean Breeze visual baseline and uses the provided
Inspira UI/Uiverse sources as visual-reference libraries only; no copied
component or new dependency is required.

| Token     | Value     | Use                                        |
| --------- | --------- | ------------------------------------------ |
| Deep Ink  | `#121c22` | Headings and primary text                  |
| Slate     | `#52636d` | Long-form body text                        |
| Sea Glass | `#eaf7fa` | Item surface and skeleton base             |
| Tide Blue | `#007bab` | Rail, active state, focus-adjacent accents |
| Lagoon    | `#008287` | Secondary pulse and term emphasis border   |
| Mint Wash | `#dff4f0` | Technology term background                 |

The signature element is a compact, `30px` Sea Glass **outlined ordinal ring**
beside each source item. Its larger, semibold-to-bold numeral makes the source
order easy to scan without turning the list into a process diagram. It is
structural--it communicates the actual list marker--rather than a decorative
solid-number badge or an interactive stepper. There is deliberately no
connector between rings: JD entries are independently readable requirements,
not workflow stages. The ring has no independent click, selection, or
completion state, and this change introduces no background effect; background
treatments remain a later visual-design decision. Section labels are semantic
titles, not a second ordinal system. Highlighted technology terms use a
low-contrast mint underwash and lagoon baseline so the reader can scan prose
without tag-shaped visual noise.

## Goals / Non-Goals

**Goals:**

- Preserve source text and only recognize structure that a deterministic
  marker grammar proves.
- Reuse persisted deterministic mention coordinates to highlight the exact
  current-body substring without a second matching algorithm.
- Make both initial and replacement reads feel stable at desktop and 390px.
- Keep the existing endpoint path and public information boundary intact.

**Non-Goals:**

- Do not alter ODS, DWD, normalization, collection, or deterministic-analysis
  output; no reparse or migration is needed.
- Do not infer requirements from sentences, train an AI formatter, add JD body
  search, or expose raw evidence, aliases, rules, versions, or audit facts.
- Do not turn recognized entries into editable checklists or claim that their
  display order is a new modeled data field.

## Decisions

### 1. Create a conservative, offset-preserving body presentation model

A front-end pure function will turn one normalized section into either:

- a sequence of `lead`, source-proven `group-heading`, `ordered-item`, or
  `bullet-item` blocks with original source offsets and original visible
  markers; or
- exactly one `fallback` block spanning the whole section.

It considers one marker family at a time. One or more Arabic or Chinese
ordinals (`1.`, `1、`, or closing-parenthesis equivalents) at valid top-level
boundaries become an ordered list; multiple ordinal markers must remain
consecutive. A bullet list requires two top-level spaced bullet markers.
An ordinal suffix using a dot followed by another digit is structurally
ambiguous rather than immediately accepted or rejected. The parser may
promote a contiguous run of those candidates only when it is an interior
portion of a syntactically consecutive ordinal sequence with unambiguous
ordinal markers on both sides. This uses numbering context, not a whitelist
of following requirement forms: `1. …；2.1-3年…；3.1年以上…；4. …` is a proven
list, while an isolated/edge candidate such as `1.28 年` remains fallback
text. The ordinary valid-boundary rule also keeps `v3.5` ineligible. A
closing-parenthesis or enumeration-comma ordinal remains a marker when its
content starts with a number: `2）2年以上相关经验` and
`1、3年以上游戏客户端开发经验` are unambiguous requirements rather than decimals.
A bullet must be at the start or follow whitespace/`；`/`;`, and must itself be
followed by whitespace; this preserves hyphenated terms such as `kube-proxy`.

The numeric-dot grammar also allows one source whitespace character between an
Arabic ordinal and its dot (for example `2 .` or `2 ．`) when the dot is not
followed by a digit. This accommodates a common copy-formatting defect while
retaining the decimal/version guard (`1 .28` remains prose) and the existing
top-level boundary requirement. The marker's visible source spelling remains
available to the renderer; the circular marker extracts only its numeral.

An otherwise consecutive Arabic sequence may restart at `1` only when the
source itself places a recognized grouping label (`基础` or `加分项`, with an
optional colon) immediately before the reset. The presentation inserts that
source span as a `group-heading`, ends the prior item before it, and starts a
separate semantic ordered list. Within one such explicit source group, markers
that begin at `1` may increase while skipping a number (for example `1. → 3.`)
because the source has already proved both group and item boundaries. The
renderer retains the source markers exactly and never invents the missing
number. Outside an explicit group, a gap or reset remains a lossless fallback:
it may be malformed input rather than a safe group break.

An already proven top-level ordinal item may contain a nested compact hyphen
list. A compact child marker is an ASCII `-` at a valid list boundary whose
next character is non-whitespace; the parent must contain at least two such
markers. This recognizes normalized source lists such as `-根据…` without
confusing hyphenated terms (`kube-proxy` has no valid preceding boundary). The
parent's text before the first child remains its visible heading, and every
child consumes a source-preserving interval through the following child or the
parent's end. The renderer uses a semantic nested `ul` beneath its parent
ordered item. A lone compact hyphen stays normal prose, avoiding a speculative
sub-list from a command flag or punctuation.

The parser does not treat commas, semicolons, sentence stops, or line breaks
as item separators. A preamble before the first accepted marker becomes a
visible `lead` block. Each item retains its source marker rather than
inventing a new ordinal. All offsets are Unicode code-point offsets, matching
the persisted technology evidence convention.

**Why this instead of sentence splitting or an AI formatter:** punctuation is
common inside technical prose, and the product must not present an invented
sentence boundary as a requirement. The fallback is less decorative but
honest and covers the 4/4/13 current unstructured sections.

### 2. Add only safe current-body ranges to the existing detail contract

`BodySections` gains an optional `technology_highlights` collection whose
records contain only a public section key (`responsibilities`, `requirements`,
or `supplemental`) and `[start, end)` Unicode code-point offsets. The
repository loads it after identifying the current membership and only joins
technology evidence when all of the following are true:

1. the selected technology resolution has priority `0`;
2. its result body ID equals the membership's body ID;
3. the source-field mapping targets an actually returned body section; and
4. the offsets are in bounds and non-overlapping after a deterministic sort.

The serving layer translates the stored source-field names to public section
keys, validates code-point bounds against the already loaded strings, sorts by
section/start/end, and omits duplicate or overlapping records. It never returns the stored
matched alias, dictionary rule, version, source/body ID, selection priority,
or audit data. The response is additive; clients that do not know the field
continue to work.

The renderer receives both the block's section-relative code-point range and
the section ranges. It clips valid ranges to the block, creates text and
`mark` fragments from `Array.from(text)`, and renders them through Vue text
bindings--never `v-html`. Defensive range merging remains in the client even
though the audit currently found no overlaps.

**Why coordinates rather than browser `includes`/regular expressions:** the
canonical label can differ from the dictionary alias that matched, case rules
are dictionary-owned, and a served fallback result might belong to an earlier
body. Coordinates preserve the authoritative match without allowing a false
highlight.

### 3. Build composed reading components rather than expand the view

`JDSectionList` owns marker recognition and semantic `ol`/`ul`/fallback
markup, including source-proven group headings, compact source sublists, and
their separate parent lists.

Ordinal recognition first normalizes the supported source marker families:
Arabic dot (`1.`, `2．`, `2 .`, `2 ．`), Arabic enumeration comma (`1、`),
Arabic closing parenthesis (`1)`, `1）`), and their Chinese numeral variants.
After two unambiguous consecutive markers establish one family, candidates from
other families are treated as ordinary source text. A style-mismatched marker
may bridge a skipped serial only when it supplies every missing serial between
two retained markers; a genuine unproven gap still uses the lossless fallback.
This prevents technology text such as `NVLink 5）` from invalidating an
otherwise readable dot-style list without weakening the decimal/version
guards.

`HighlightedText` owns range clipping and safe fragment rendering.
`JobDetailView` supplies only title, source section key, text, and detail
ranges. This division makes the conservative grammar independently testable
and lets a future detail-like page reuse the presentation without importing
route state.

Rows use the ledger rail, original-marker capsule, generous 12--16px vertical
rhythm, and static Sea Glass surfaces. The raw fallback remains normal
readable prose, clearly avoiding a misleading row frame. At 390px the
rail/marker occupies a fixed narrow column and the text grid remains min-width
zero, so long URLs, version names, and Chinese prose wrap rather than overflow.

The selected floating-island presentation instead uses one softly outlined
surface per proven source item. On a narrow viewport, status pills occupy a
row of their own before the title; they must never reduce the title's reading
measure. The prototype's variant switcher and explanatory implementation note
are development-only comparison aids and are absent from the selected view.
When a JD has more than eight technology labels, every viewport initially
renders the first eight labels inside the same two-row clipped preview and a
compact secondary button. The control reads `+ 展开` and `− 收起` rather than
restating the field name or exposing a keyword count. It borrows the quiet
bordered posture of the selected Inspira interactive-hover reference, but uses
the native button element and Ocean Breeze Sea Glass/Tide Blue tokens; it is a
disclosure control, not a primary CTA. Capping the initial collection makes
the control useful even when short labels happen to fit inside two wide rows.

Every selected source-item surface is deliberately static. It uses no
pointer-tracked scale, tilt, light wash, 3D compositing, or transform
transition; the complete text stays sharp and available to read without a
hover interaction.

The application deliberately does not apply `prefers-reduced-motion`
overrides. The supplied loader and skeleton motion remain observable regardless
of host motion preference during development and review.

### 4. Centralize request activity while keeping route geometry local

A small application loading coordinator tracks outstanding browser requests
by token, rather than a single boolean per view. The API client registers and
settles a token in `finally`; `App.vue` reads the derived count and hosts the
one fixed loading layer. A delayed appearance threshold avoids a distracting
flash for very fast requests, and the layer uses the three existing tide bars
as a restrained pulse.

The list and detail views keep their own initial-layout skeleton components:
they render only before their first successful result, reserve the real card
area, and share Ocean Breeze surfaces. Once a successful result exists,
replacement requests leave it mounted and place the global veil above it.
Per-view monotonic request IDs prevent a late older response from replacing a
newer route/filter result. A failed replacement keeps the last successful
content stable and presents the existing safe error message in context.

The JD-detail initial skeleton reserves the selected detail composition: two
title lines, a four-cell fact area, technology rows, and three source-item
surfaces. It replaces the text-only loading branch and fades into the loaded
content without a layout jump.

**Why not use inline `v-if="isLoading"` paragraphs:** those branches remove
the result region, change document height, and permit concurrent requests to
clear each other's indicator. The coordinator and skeleton split feedback
from content lifetime.

## Risks / Trade-offs

- **A true list has an unsupported or malformed marker** → It remains a
  faithful fallback paragraph. Add a grammar only after audit samples justify
  it; never loosen punctuation rules by default.
- **A future normalizer changes text after evidence was written** → Same-body
  identity plus server bounds validation suppresses highlights rather than
  displaying incorrect ones.
- **Unicode code-point offsets are mistaken for JavaScript string offsets** →
  The contract names the unit explicitly and the renderer uses `Array.from`;
  tests include an astral Unicode character before a match.
- **A transparent loading veil can feel delayed on slow networks** → It is
  coupled with an aria-live status and route-specific skeleton, while existing
  content stays legible beneath a low-opacity veil.
- **New components make snapshots visually fragile** → Test semantic blocks,
  markers, and `mark` text; reserve visual checks for desktop and 390px
  acceptance instead of asserting incidental class names.

## Migration Plan

1. Add server contract/range tests and the additive detail field. No schema
   migration or historical recomputation occurs because the required evidence
   already exists.
2. Add presentation/highlight/loading components and route tests. An older
   API simply produces no ranges and renders unhighlighted text safely.
3. Run focused Go and Vue suites, production read-only range checks, the web
   build, strict OpenSpec validation, and desktop/390px manual visual checks.
4. Release through the existing immutable-image chain. No new port, endpoint,
   public listener, credential, or database exposure is introduced.

Rollback can deploy the prior frontend (which ignores the additive field) or
the prior backend (which omits it). The renderer's fallback behavior means
there is no data migration to reverse.
