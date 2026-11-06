# Spec Delta

## Purpose

Bound CVM Docker image growth after an immutable mRoleScope deployment while
preserving the active release, one verified rollback release, and all runtime
data.

## ADDED Requirements

### Requirement: Successful releases retain two complete generations

After a release is healthy and recorded, the deployment helper SHALL retain the
four immutable images of the current release and the four immutable images of
the immediately preceding successful release. It SHALL capture the predecessor
from the validated prior `current.env` before writing the new current record.

#### Scenario: A third release succeeds

- **WHEN** the current release and one predecessor are recorded and a third
  release passes its health checks
- **THEN** images belonging to the new and preceding releases remain local
- **AND THEN** an eligible image belonging only to an older release may be
  removed

### Requirement: Cleanup has a narrow Docker scope

The helper SHALL consider only digest-addressed API, migration, Worker, and Web
Edge images in its configured Docker Hub namespace. It SHALL skip any image
referenced by a running or stopped container and SHALL NOT force removal or
globally prune Docker resources.

#### Scenario: A stale image is still referenced

- **WHEN** an otherwise removable old Worker image is referenced by a stopped
  container
- **THEN** the helper leaves that image intact
- **AND THEN** it may remove unrelated eligible old mRoleScope images

### Requirement: Cleanup cannot invalidate a verified deployment

The helper SHALL record release success before cleanup. A failure to list or
remove an individual candidate SHALL be logged as cleanup-incomplete and SHALL
NOT stop healthy services, rewrite the release as failed, or remove data
volumes.

#### Scenario: Docker refuses one candidate removal

- **WHEN** one old image cannot be removed
- **THEN** the current service continues running with its successful release
  record intact
