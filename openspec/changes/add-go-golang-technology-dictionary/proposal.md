# Proposal

## Why

The published technology dictionary does not currently expose the Go programming
language: its `Go` and `Golang` aliases remain conditional.  Go is a required
programming-language dimension and case variation in source JD text must not
silently omit it.

The current v0.2 dictionary is already published, so changing it in place
would invalidate its fingerprint and historical evidence.  The correction must
therefore use an immutable successor and a complete reanalysis.

## What Changes

- Add immutable technology dictionary v0.3, retaining v0.1 and v0.2 unchanged.
- Register `Go` and `Golang` as aliases of the canonical `Go` programming
  language with ASCII case-insensitive matching and ASCII identifier boundaries.
- Make only those two reviewed v0.3 rules newly approval-eligible; all other
  v0.3 rules retain the completed v0.2 policy.
- Add fixed, root-owned v0.3 candidate seed, approval, preparation, analysis,
  and release-upgrade operations.
- Atomically advance the active technology-analysis pointer only after v0.3
  has full historical coverage and coordinate validation; preserve v0.2 facts
  as superseded history.

## Non-Goals

- No fuzzy spelling correction, unregistered abbreviation, source-text
  collection, schema change, AI invocation, or frontend-only synthetic option.
- No modification, deletion, or reclassification of v0.2 evidence.

## Impact

- `internal/jdanalysis` gains the v0.3 dictionary/release policy and tests.
- The existing fixed remote-development worker operations gain version-specific
  v0.3 paths with no source, COS, host port, or arbitrary SQL input.
- A later explicitly authorized deployment can populate the normal Serving API
  with verified Go mentions; this change itself does not touch CVM data.
