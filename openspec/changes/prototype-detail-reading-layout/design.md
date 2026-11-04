# Design

## Question

What visual structure lets a technical recruiting reader scan a real JD's
formal facts and lengthy source-proven body items without mistaking those
items for a workflow or a collection of dashboard cards?

## Audience and page job

The audience is an interviewer or recruiting partner deciding whether a
specific currently effective role deserves attention. The page's job is to
make requirements easy to compare while keeping the original public JD facts
intact.

## Selected direction

The product owner selected **C — 边注档案** for the normal detail route. Its
margin index is the one signature structural device: it names the actual JD
regions so a reader can regain context in a long brief, while the source items
remain independent editorial rows rather than steps or dashboard cards. A/B,
the variant URL state, the keyboard preview handler, and the floating switcher
are comparison artefacts and do not belong in the shipped page.

## Selected visual language

The shipped layout is hosted by the existing detail route and uses actual
Serving data. It spends emphasis in one place only: a three-pixel Tide Blue
section edge that identifies a reading region.

| Token | Value | Use |
| --- | --- | --- |
| Deep Ink | `#121c22` | title and primary prose |
| Slate | `#52636d` | long source text |
| Sky Field | `#e8f2f8` | page ground |
| Sea Glass | `#eef8fa` | restrained secondary surfaces |
| Tide Blue | `#007bab` | section edge and active anchor |
| Lagoon | `#008287` | secondary fact emphasis |

Headings use the existing system CJK display stack at a restrained scale; body
text retains the existing Inter/system-ui reading face; source markers use the
monospaced utility face. No remote font or component dependency is introduced.

### C — 边注档案

At wide widths, a compact margin index links to the responsibilities,
requirements, and supplemental regions. The body itself uses editorial rows
with a wider marker gutter; at narrow widths the index becomes an ordinary
inline section list. It supplies navigation context without introducing another
card or workflow-like visual system.

The selected C study uses a deeper editorial Slate (`#36525f`) for its body
prose and a stronger mint technology treatment: deep teal text on Sea Mint
(`#b7efdf`), with a Lagoon underline. This deliberately separates verified
technology mentions from ordinary source text while preserving bright yellow
as the project-title-only semantic established by the current product UI.

## Production composition

The selected page keeps the exact preview arrangement:

```text
back to list
┌ formal role brief ────────────────────────────────┐
│ title / location                         states    │
├ role family · education · experience · first seen ┤
└───────────────────────────────────────────────────┘
metadata tags

正文目录              岗位职责
岗位职责              1   source-faithful requirement row
任职要求              2   source-faithful requirement row
补充说明
                      任职要求 …
```

At 700px and below, the directory becomes the same inline horizontal section
list used by the C preview. The complete metadata tag collection remains
visible; this selected reading layout does not substitute a disclosure button
or a count for public tags. Formal facts use the existing presentation helpers
without reinterpreting education, experience, activity, or discovery fields.

## Implementation boundary

The production component deliberately calls the existing deterministic
`presentJDSection` function and `HighlightedText` renderer. It receives only
the already-loaded public `JobDetailResponse`; it neither reparses source text
nor performs another request. The standard detail route renders it regardless
of build mode and ignores obsolete preview query parameters.

The margin index retains link-like visual affordance but intercepts its
activation to scroll the existing target region into view. It does not write a
fragment URL or browser-history entry: the page-level “返回岗位列表” action must
return to the list in one step regardless of how many index regions were read.
