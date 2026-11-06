# Design

## Contract

`JobDetailResponse.comparison` is optional. It is present only for a
historical detail of a changed job for which a current JD is available.
Successful empty comparisons contain no changes and render no change tags.
Unavailable comparisons return a display-safe status and retry hint while the
historical response remains HTTP 200 and complete.

The comparison contains dimension changes with display labels and formatted
historical/latest values, plus body sections whose ordered fragments are
`unchanged`, `deleted`, or `inserted`. No IDs, run/version identifiers,
database names, parser versions, or raw errors cross the public boundary.

## Data and consistency

Serving reads the node-day membership and current membership in one
repeatable-read, read-only transaction. Multi-value dimensions compare as
sets, ignoring order. The compared dimensions are title, locations, main and
other job families, education, minimum experience, technology keywords,
projects, and urgent recruitment. First discovered date and lifecycle state
are excluded.

Body comparison normalizes only line endings and consecutive spaces for
matching, preserves source text/case/punctuation in output, compares list
items/sentences before character-level changes, and treats reordering as
delete plus insert. Historical deleted text is emitted before adjacent latest
inserted text. Full section additions/removals remain visible. Myers diff is
bounded by an edit-distance guard; an unusually large replacement is still
returned completely as one deleted block followed by one inserted block,
preventing pathological memory use without hiding text.

## UI

The existing single-column detail reader gets independent collapsed
`维度变动` and `正文变动` summaries. Counts mean affected fields/sections,
not fragment count. Body fragments render in their original positions; old
list numbering is preserved and new items use their latest position. Diff
colors use local light/dark tokens and explicit `aria-label`/text semantics.

## Failure and performance

Comparison failure never removes historical content. The UI shows
`变动对比暂不可用` and retries only the current detail request. The existing
100ms skeleton, 1.5s detail p95, and bounded query budgets remain the release
targets. Full fragments are not silently truncated.
