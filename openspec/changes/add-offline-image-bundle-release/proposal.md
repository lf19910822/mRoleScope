# Proposal

## Why

The reviewed release can build and push immutable images locally, but the CVM currently cannot reliably reach Docker Hub's Registry and authentication endpoints. This blocks deployment even when the application build, tests, and local image publication succeed. We need a controlled SSH-based image-bundle path that avoids Docker Hub without turning deployment into ad-hoc manual Docker commands.

## What Changes

- Add an offline release-bundle capability for the four reviewed `linux/amd64` application images plus the pinned PostgreSQL runtime image required by Compose.
- Build a self-describing bundle containing all five image archives, a manifest, the source revision, and integrity checksums.
- Upload the bundle over the existing key-based SSH channel to a root-controlled CVM staging area.
- Make the CVM deployment boundary verify the bundle before loading or starting anything, including image completeness, architecture, source revision labels, archive checksums, and bundle revision consistency.
- Reuse the existing migration, health-check, release-record, rollback, and image-retention behavior after the images are loaded locally.
- Add a release-mode selector so Docker Hub remains the normal path and the offline bundle is an explicit emergency path.
- **BREAKING**: direct manual `docker load`, `docker compose up`, mutable tags, and unverified image archives remain unsupported deployment operations.

## Capabilities

### New Capabilities

- `offline-image-bundle-release`: Provides an auditable, integrity-checked, SSH-delivered image bundle release path that preserves the existing controlled CVM deployment lifecycle.

### Modified Capabilities

None. The current Docker Hub release behavior remains unchanged; this change adds an alternative release transport.

## Impact

- Affected local release orchestration in `scripts/` and deployment boundary assets in `deploy/remote-development/`.
- Adds local Docker archive generation and SSH transfer without exposing runtime secrets or database connections.
- Adds CVM staging and cleanup behavior but does not change application APIs, database schemas, public network exposure, or runtime credentials.
- Requires new shell-level tests for manifest validation, archive integrity, revision consistency, failure atomicity, and release-mode selection.
