# Validation

Completed locally on 2026-11-03, except the controlled CVM delivery in task
3.2.

- Red phase: the worker orchestration test initially failed to compile because
  the post-release Serving seam did not exist.  The database regression is
  red against the former candidate-selection query because a newer unpublished
  Go result leaks into the current technology options.
- Focused synthetic PostgreSQL validation applied all migrations to a temporary
  loopback-only database and passed both
  `TestServingReconciliationUsesOnlyActiveTechnologyPublication` and
  `TestServingJDDetailPublishesOnlyCurrentBodyTechnologyHighlights`.
- The existing details-highlight fixture was corrected to publish its synthetic
  technology configuration and advance the publication pointer.  This keeps
  its established contract aligned with the new product boundary: Serving
  only displays technology facts from the active publication.
- `go test ./cmd/... ./internal/...`, `make deploy-config-check`, strict
  OpenSpec validation, and `git diff --check` pass in the isolated PR worktree.
- No real database, JD text, credential, source, or deployment setting was
  accessed or changed during local validation.
