# Tasks

## 1. Regression coverage

- [ ] 1.1 Add a failing PostgreSQL integration test that rejects a newer
  unpublished technology configuration and exposes Go only after the active
  publication pointer advances and Serving reconciles.
- [x] 1.2 Add failing worker orchestration tests for release success, reused
  release, release failure, and retryable Serving reconciliation failure.

## 2. Implementation

- [x] 2.1 Restrict technology-resolution selection to the active published
  configuration.
- [x] 2.2 Reconcile Serving after the fixed v0.3 release operation commits,
  preserving safe retry semantics.

## 3. Verification and delivery

- [ ] 3.1 Run focused and full Go tests, deployment configuration checks,
  strict OpenSpec validation, and `git diff --check`.
- [ ] 3.2 Deploy only the approved immutable image path, invoke the existing
  fixed v0.3 upgrade operation to retry projection, and verify aggregate
  filter-options evidence that Go is visible.
