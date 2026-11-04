# Tasks

## 1. Safe serving highlight ranges

- [x] 1.1 Write failing repository tests for detail technology ranges covering a same-body selected result, an older fallback result, each source-field mapping, out-of-bounds evidence, and overlapping evidence; implement the deterministic range loader and sanitizer, and verify `go test ./internal/jdserving` passes.
- [x] 1.2 Add the additive, display-safe highlight-range type to the public detail DTO and `openapi/openapi.yaml`; verify contract and HTTP tests prove the payload exposes only section/start/end, retains existing detail fields, and excludes all provenance, dictionary, parser, alias, fallback, and audit fields with `go test ./internal/jdserving ./internal/httpserver`.
- [x] 1.3 Run a scoped, read-only CVM query after implementation to count current same-body range rows, in-bounds rows, duplicates, and overlaps; record only aggregate counts in the change validation notes and verify no production data is mutated.

## 2. Lossless JD body reading components

- [x] 2.1 Add a pure, offset-preserving section presentation utility with red/green unit tests for consecutive Arabic markers, Chinese markers, repeated spaced bullets, inline normalized lists, lead-in text, decimals/versions, hyphenated technology names, incomplete markers, and full-text fallback; verify `npm --prefix web run test -- --run` covers the parser cases.
- [x] 2.2 Add a text-fragment highlighter with tests for code-point ranges after astral Unicode, block-range clipping, malformed/overlapping client ranges, and literal angle-bracket source text; verify rendered output uses text nodes and marks only valid supplied intervals.
- [x] 2.3 Build semantic ordered, unordered, lead-in, and fallback body-section components using the Ocean Breeze requirements-ledger treatment; verify focused component tests retain all text and use appropriate list semantics.
- [x] 2.4 Integrate the typed detail client and body components into `JobDetailView`, replacing the dense paragraph presentation while retaining absent-supplemental behavior; verify detail-view tests cover a recognized list, a fallback body, and highlighted text.
- [ ] 2.5 Check desktop and 390px viewports against the supplied Ocean Breeze baseline; verify long CJK/technical text wraps without horizontal overflow, row markers remain readable, and normal/fallback text stays selectable.
- [x] 2.6 Move the selected floating-island pointer response from the development-only prototype into the production list component, with bounded `1.05` scale/`±4deg` tilt, a 360ms reset, touch exclusion, and focused component tests.
- [x] 2.7 Keep inactive source-list text outside the 3D compositing treatment; enable the selected 3D effect only while its mouse row is active, with a focused regression test.
- [x] 2.8 Accept a closing-parenthesis ordinal when its source requirement begins with a number, while retaining decimal, version, and enumeration-comma protections; verify the focused parser regression test and full front-end suite pass.
- [x] 2.9 Render one unambiguous top-level Arabic or Chinese ordinal as a single ordered source-item surface, while keeping the repeated-marker threshold for bullet lists; verify a single enumeration-comma ordinal regression test and the full front-end suite pass.
- [x] 2.10 Accept an enumeration-comma ordinal when its source requirement begins with a number, so the first item of a numbered list cannot become lead text; retain dot-style decimal/version protection and verify the parser regression and full front-end suite pass.
- [x] 2.11 Replace the verbose technology-keyword disclosure copy with the compact `+ 展开` / `− 收起` native button at every viewport width; preserve ARIA state/controls and verify the detail and prototype expand/collapse tests plus the full front-end suite pass.
- [x] 2.12 Recognize source-proven `基础` / `加分项` headings around an ordinal reset, render each group as a separate semantic list, and retain a full-text fallback for unheaded resets; verify parser and component regressions plus the full front-end suite pass.
- [x] 2.13 Retain source-proven `基础` / `加分项` groups when a group starts at `1` but later skips an ordinal, without inventing the missing marker; retain fallback for an equivalent ungrouped gap and verify the real-pattern parser regression plus the full front-end suite pass.
- [x] 2.14 Recognize a one-horizontal-space numeric-dot ordinal without accepting spaced decimal/version fragments; preserve the source marker/numeral and verify a focused parser regression plus the full front-end suite pass.
- [x] 2.15 Render two-or-more compact hyphen entries within a proven ordinal source item as a nested semantic bullet list; retain the parent preamble, lone compact hyphens, and hyphenated terms as source prose, and verify parser/component regressions plus the full front-end suite pass.
- [x] 2.16 Write failing detail/component regressions proving that an over-eight keyword collection initially renders only eight labels and that source-item rows remain static under pointer movement; replace the ineffective CSS-only disclosure with a bounded preview and remove the floating-island handlers and styles, then run the full front-end suite.
- [x] 2.17 Write a failing regression for numeric-leading items inside a
      source-proven consecutive numeric-dot sequence, then promote only its
      interior ambiguous candidates without content-form whitelists; retain
      fallback behavior for edge/isolated decimal and version candidates and
      verify parser/component regressions plus the full front-end suite pass.

## 3. Stable application loading feedback

- [x] 3.1 Add a token/reference-counted browser request coordinator and one application-shell loading layer with a delayed Ocean Breeze tide treatment; verify unit tests cover overlapping requests, settling order, failures, and accessible status.
- [x] 3.2 Add geometry-matched initial skeletons for the JD list and detail, then replace inline text-only loading branches; verify view tests show skeletons only before a first successful response and reserve the normal content region.
- [x] 3.3 Retain each view's latest successful response during replacement fetches and ignore stale route/filter responses; verify browser-flow tests cover filter, pagination, and detail transitions without unmounting existing content or changing scroll position while loading.

## 4. Integrated validation and handoff

- [x] 4.1 Run focused Go service/HTTP tests, the Vue unit and browser-flow suites, `npm --prefix web run build`, `openspec validate improve-jd-detail-readability --strict`, and `git diff --check`; record exact commands and results in the change validation note.
- [x] 4.2 Perform a production-safe detail API acceptance read through the existing private path and confirm a current JD exposes non-empty body sections and display-safe technology emphasis without exposing internal fields; do not record JD body content or credentials.

## Workflow follow-up

- Review the completed change before deployment.
- Archive the change after the project's review requirements are satisfied.
