# Proposal

## Why

The current JD detail view renders each normalized body section as one dense
paragraph. Readers cannot quickly distinguish independent responsibilities or
requirements, even though the current production corpus commonly expresses
them with repeated inline number markers or spaced bullet markers. The
browser also needs a stable, pleasant loading treatment that does not shift
the page while an existing result is being refreshed.

## What Changes

- Render a JD body section as an ordered set of readable requirement rows only
  when conservative, deterministic markers prove that it is a list. Preserve
  the complete normalized source text as one fallback block when its item
  boundaries cannot be established safely. Preserve explicit `基础` / `加分项`
  source groups even if a later source marker accidentally skips a number;
  retain the shown markers rather than repairing or inventing an ordinal.
- Treat a numeric-dot candidate followed by a digit as structurally ambiguous,
  rather than classifying its following content. Promote it only when it is an
  interior part of a source-proven consecutive ordinal sequence bounded on
  both sides by unambiguous ordinal markers; keep isolated and edge candidates
  as fallback text so decimals and versions remain safe.
- Recognize a compact source typo between a numeric ordinal and its dot (such
  as `2 .`) without mistaking decimal/version fragments for entries. When an
  ordinal source item contains multiple proven compact `-` subentries, render
  them as nested requirements while retaining the parent responsibility.
- Project display-safe, current-body technology mention ranges through the
  existing JD-detail endpoint so the browser can highlight exactly the matched
  text in responsibilities, requirements, and supplemental text. Do not add
  an endpoint or expose dictionary, parser, source-version, audit, or fallback
  metadata.
- Present list items and highlighted terms using the existing Ocean Breeze
  visual language: restrained tide-blue marker/edge treatments, readable
  spacing, text-safe rendered fragments rather than HTML injection, and
  complete static source-list rows for uninterrupted reading.
- Establish one global, accessible route/data loader with a fixed overlay
  layer and preserved content geometry. Initial loads use a matching skeleton;
  refreshes keep the existing page in place under a non-blocking loading veil.

## Capabilities

### New Capabilities

- `jd-detail-body-presentation`: Deterministic, lossless body-section list
  presentation with a raw-text fallback for ambiguous content.
- `jd-detail-technology-highlighting`: Display-safe exact technology-match
  ranges for the current body and their accessible rendering in a JD detail.
- `jd-browser-loading-feedback`: Consistent, non-jumping loading feedback for
  browser routes and data refreshes.

### Modified Capabilities

<!-- No archived capability exists yet; the related earlier change artifacts
     are still in flight rather than installed under openspec/specs/. -->

- None.

## Impact

- Frontend: a reusable body-section renderer, a safe fragment highlighter, a
  global loader/skeleton layer, and the JD list/detail views and tests.
- Serving: additive fields on the existing `GET /api/v1/jobs/{id}` contract,
  its OpenAPI description, repository query, and contract tests. Existing
  list/detail fields and endpoint paths remain compatible.
- Data: no ODS/DWD mutation and no migration. Serving reads existing
  deterministic technology evidence only when it belongs to the same current
  body version; unmatched, stale, malformed, or overlapping ranges are simply
  omitted from highlighting.
