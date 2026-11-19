# Design

## Context

The current release orchestrator builds four application images locally, pushes them to Docker Hub, and passes immutable Docker Hub digests to the root-controlled CVM deploy helper. Compose also needs the pinned PostgreSQL image, so a Docker Hub outage can still block deployment even when all application images were pushed successfully. The existing helper owns migration, health checks, release records, rollback, and image cleanup and must remain the only activation boundary.

## Goals / Non-Goals

**Goals:**

- Add an explicit offline release mode that uses the existing SSH control channel and does not contact Docker Hub from the CVM.
- Preserve one revision identity across all application images and validate the pinned PostgreSQL image by its loaded image ID.
- Keep bundle upload, extraction, image loading, Compose environment selection, migration, health checks, release records, rollback, and cleanup inside controlled scripts.
- Make archive generation and validation testable locally without a real CVM or runtime secrets.

**Non-Goals:**

- Do not replace the default Docker Hub release path.
- Do not build application source on the CVM.
- Do not change public networking, runtime secrets, database schema, or application behavior.
- Do not add a registry mirror, proxy, or paid image service.

## Decisions

### Bundle format

Use a gzip-compressed tar bundle with fixed archive names and a strict line-oriented manifest. The manifest records format version, Git revision, image reference, archive filename, image ID, architecture, and archive SHA-256 for five images. A fixed format keeps validation available with the base utilities already present on the CVM and avoids introducing a JSON parser or package dependency.

### Image identity

Build application images with the existing revision label, retag them under an offline-only local namespace, and save those tags. The PostgreSQL image is pulled locally at its existing pinned digest and saved under an offline tag. After `docker load`, the helper compares each loaded image ID and architecture with the manifest; application images must also carry the bundle revision label. Compose receives the offline tags only for this deployment.

### Transfer and activation boundary

The local orchestrator creates the bundle in a private temporary directory, verifies it, uploads it to a fixed root-owned staging directory with `scp`, and invokes a new restricted bundle entry point. The CVM helper validates archive paths before extraction, verifies every checksum and image identity, loads all five images before changing Compose state, and then delegates to the existing lifecycle phases. A bundle failure exits before migration or service replacement.

### Compose compatibility

Parameterize only the pinned PostgreSQL image with a default that is byte-for-byte equivalent to the current Compose value. The normal Docker Hub path continues using the default. Offline mode exports a fixed local PostgreSQL tag and fixed local application tags for the helper invocation; no mutable user-controlled image name is accepted.

### Cleanup and rollback

The bundle is removed from staging after a successful or safely recorded failed operation. Loaded images are retained or removed by the existing two-generation cleanup rules, and the current release record is never advanced until migration and health checks pass. The failed bundle diagnostic record contains only revision, phase, status, and safe counts.

## Risks / Trade-offs

- [Large transfer] → Uploading five saved images may take longer than a registry pull; use one compressed bundle, a bounded staging directory, and checksum verification before activation.
- [Docker archive compatibility] → Pin the bundle format and test with the repository's supported Docker version; reject unknown formats rather than attempting best-effort extraction.
- [Stale local PostgreSQL image] → Require the exact pinned PostgreSQL image ID in the manifest and fail if the local image cannot be inspected at that digest.
- [Partial upload] → Upload to a revision-specific temporary file, verify its whole-file checksum remotely, and rename it only after the transfer completes.
- [CVM disk pressure] → Check available space before staging and clean temporary files on every exit path; never run global Docker prune.

## Migration Plan

1. Add failing local tests for bundle creation, manifest validation, tamper rejection, and release-mode selection.
2. Add the offline bundle builder and local orchestrator path.
3. Add the root-controlled CVM bundle helper, bootstrap installation, and Compose PostgreSQL override.
4. Run shell regression tests, local release verification, OpenSpec strict validation, and `git diff --check`.
5. Merge to protected `main`; use the offline mode only after the reviewed code is on `main` and a deployment authorization is present.
6. Roll back by invoking the existing release rollback behavior; no source data or runtime secret migration is required.

## Open Questions

None. The five-image bundle boundary and explicit offline mode are fixed by the Docker Compose dependency graph and the current deployment failure.
