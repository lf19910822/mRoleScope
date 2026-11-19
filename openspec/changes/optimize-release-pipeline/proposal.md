# Proposal

## Why

Every release currently refreshes the CVM deployment boundary by downloading
roughly two dozen small files, even when no deployment asset changed. Network
timeouts on those sequential downloads add minutes to otherwise ordinary
application releases. The release must become faster without weakening
immutable image, migration, health, rollback, or asset-integrity checks.

## What Changes

- Compute a reviewed deployment-asset fingerprint from the clean `main` tree.
- Persist that fingerprint on the root-owned CVM deployment boundary.
- Skip bootstrap downloads and installation when the installed fingerprint is
  identical; still run the normal four-image digest deployment.
- Run bootstrap when the fingerprint is missing or changed, and verify the
  fingerprint before installing any asset.
- Emit elapsed time for the major release phases so future regressions are
  measurable.
- Keep the existing Docker Hub digest path, migration, health, rollback, and
  cleanup behavior unchanged.

## Capabilities

### New Capabilities

- `release-pipeline-optimization`: Reuses an unchanged, integrity-identified
  CVM deployment boundary and reports release phase timing.

### Modified Capabilities

## Impact

Affected files are the local release orchestrator, reviewed CVM bootstrap,
deployment runbook, and shell regression tests. No application API, database
schema, runtime secret, public port, image tag, or deployment authority changes.
