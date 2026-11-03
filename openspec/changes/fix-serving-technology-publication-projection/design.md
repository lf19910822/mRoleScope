# Design

## Published configuration is the Serving boundary

`technology_analysis_publication_state` is the durable declaration of the
only technology configuration approved for product display.  Serving
reconciliation SHALL join a technology result to that pointer.  Result
completion time remains a deterministic tie-breaker only within that one
configuration; an unpublished or superseded configuration cannot influence a
current JD card, its details, or filter options.

## Post-commit reconciliation

The v0.3 release transaction must remain limited to immutable dictionary,
configuration, evidence, pointer, and attestation facts.  The Serving
projection remains a separately retryable DWD-to-Serving operation, so the
worker performs it only after `ReleaseV03Upgrade` has committed successfully.

The fixed publish-upgrade command has these outcomes:

1. release fails: no reconciliation is attempted;
2. release succeeds or is safely reused: reconcile every published inventory
   run using the active publication pointer;
3. reconciliation fails: return a safe error without rolling back the
   already-committed release; rerunning the same fixed command retries it.

This preserves the existing best-effort projection boundary while ensuring a
successful controlled upgrade reaches the public Serving API immediately in
the normal case.

## Verification

An integration regression fixture creates an active v0.2 result and a newer
unpublished v0.3-style result.  It verifies that only the active result is
served, then advances the publication pointer and verifies that `Go` appears
in `ListFilterOptions` after reconciliation.  A worker unit test verifies the
post-release call order and retryable projection error behavior without a
real database.
