# Design

## Context

The release orchestrator currently verifies the local checkout, runs the local quality gate, builds and pushes four application images, refreshes CVM deployment assets, and invokes the root-controlled deployment helper. The helper then runs one Compose pull for PostgreSQL and the four immutable application digests. A direct `curl` test against `registry-1.docker.io` is not part of the release code, but external diagnosis incorrectly treated that path as authoritative even though the CVM Docker daemon had a configured registry mirror.

The fix must make the release path self-describing and must not add a second network path that can be confused with the real one.

## Goals

- Make the CVM Docker daemon's actual pull path the only release network truth.
- Fail before Docker Hub image builds and pushes when the pinned runtime image cannot be pulled by the CVM.
- Preserve the existing one-shot Compose pull and deployment helper boundary.
- Provide enough safe context to distinguish mirror failure, direct-registry failure, local image cache, and application health failure.
- Keep dry-run completely non-remote.

## Non-goals

- Do not remove or replace the configured Tencent Cloud registry mirror.
- Do not add Docker Hub credentials to the CVM or inspect runtime secrets.
- Do not add automatic retries, direct Registry probes, offline bundle transport, or a paid registry.
- Do not alter application images, Compose services, migration behavior, or public exposure.

## Decisions

### Preflight placement

`--deploy` runs the CVM preflight after local `main`/worktree checks and before the local `remote-release-verify` quality gate, any Docker test-image build, or any application image build and push. This avoids spending local Docker work or a push on a CVM that cannot obtain the pinned PostgreSQL runtime image. The preflight itself only checks the pinned runtime image and does not start or replace an application service.

The preflight uses the exact PostgreSQL image reference parsed from `deploy/remote-development/compose.yaml`, including its digest. It performs one bounded `docker pull` through the CVM daemon. It prints only the safe registry mirror list and pull status.

### Failure diagnostics

The root helper keeps the normal live Compose pull output through `tee` into a temporary root-owned log. If the command fails, it prints the saved output once, then prints the daemon's configured registry mirrors and the fixed Compose pull operation. It does not retry, curl a registry endpoint, or interpret the logical `docker.io/...` image name as proof of the transport endpoint.

The temporary log is removed on both success and failure. The existing failure record continues to contain only the phase, safe image references, revision, and timestamp.

### Test strategy

Shell tests use mocked `ssh` and Docker CLIs. The release orchestrator test verifies that the remote command contains the exact pinned pull, mirror reporting, bounded timeout, and no direct official Registry probe, and that the preflight occurs before the first Docker build. The helper test verifies the diagnostic branch on a simulated Compose pull failure and confirms no second pull, migration, or service start occurs.

## Operational flow

```text
clean main
  -> CVM Docker daemon preflight for pinned postgres digest
  -> local quality gate
  -> build and push four immutable application images
  -> refresh reviewed CVM assets
  -> helper Compose pull with failure diagnostics
  -> existing migration, health, rollback, record, cleanup
```
