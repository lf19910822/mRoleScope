# Proposal

## Why

The Go/Golang v0.3 technology correction is published and its historical
analysis has completed, but the Serving projection was not refreshed as part
of the controlled upgrade operation.  As a result, the public filter-options
endpoint still reads the earlier v0.2 technology resolutions and cannot show
`Go`.

The projection candidate query also chooses the newest completed technology
result without constraining it to the active publication pointer.  A future
completed but unpublished candidate could therefore appear in Serving before
its release gate has passed.

## What Changes

- Restrict Serving technology-resolution candidates to the configuration
  identified by `technology_analysis_publication_state`.
- Extend the fixed v0.3 publish-upgrade worker operation to reconcile the
  Serving inventory after the durable release transaction commits.
- Treat Serving reconciliation as a retryable post-release projection: a
  failed refresh returns a safe operation failure, while a repeated fixed
  publish-upgrade reuses the release and retries the projection.
- Add regression coverage for both the published-configuration selection and
  release-to-projection orchestration.

## Non-Goals

- No change to Go matching rules, dictionary content, source collection,
  public API shape, or frontend behavior.
- No direct SQL repair, deletion, or rewriting of v0.2/v0.3 historical facts.
- No coupling of the Serving projection transaction to the immutable release
  transaction.

## Impact

- `internal/jdserving` selects only the active published technology result.
- `cmd/worker` makes the existing fixed v0.3 release path make its published
  facts visible to the existing Serving API.
