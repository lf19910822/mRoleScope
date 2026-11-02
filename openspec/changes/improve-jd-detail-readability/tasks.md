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

## 3. Stable application loading feedback

- [x] 3.1 Add a token/reference-counted browser request coordinator and one application-shell loading layer with a delayed Ocean Breeze tide treatment; verify unit tests cover overlapping requests, settling order, failures, accessible status, and reduced-motion behavior.
- [x] 3.2 Add geometry-matched initial skeletons for the JD list and detail, then replace inline text-only loading branches; verify view tests show skeletons only before a first successful response and reserve the normal content region.
- [x] 3.3 Retain each view's latest successful response during replacement fetches and ignore stale route/filter responses; verify browser-flow tests cover filter, pagination, and detail transitions without unmounting existing content or changing scroll position while loading.

## 4. Integrated validation and handoff

- [x] 4.1 Run focused Go service/HTTP tests, the Vue unit and browser-flow suites, `npm --prefix web run build`, `openspec validate improve-jd-detail-readability --strict`, and `git diff --check`; record exact commands and results in the change validation note.
- [x] 4.2 Perform a production-safe detail API acceptance read through the existing private path and confirm a current JD renders list/fallback sections and exact technology emphasis without exposing internal fields; do not record JD body content or credentials.

## Workflow follow-up

- Review the completed change before deployment.
- Archive the change after the project's review requirements are satisfied.
