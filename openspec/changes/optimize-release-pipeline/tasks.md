# Tasks

## 1. Fingerprinted deployment boundary

- [x] 1.1 Add deterministic deployment-asset fingerprinting to the local release orchestrator and verify path/content changes alter only the safe 64-character digest.
- [x] 1.2 Extend bootstrap with an optional validated fingerprint argument and root-owned marker persistence, verifying invalid values fail before installation.
- [x] 1.3 Skip bootstrap downloads on an exact remote fingerprint match and verify a mismatch/missing marker uses the existing staged download and checksum flow.

## 2. Observable release timing

- [x] 2.1 Add safe elapsed timing for preflight, quality gate, image publication, asset refresh, and remote deployment phases and verify output contains no secrets.
- [x] 2.2 Avoid forced registry refreshes for digest-pinned local base images and verify the release build command retains platform and digest-pinned Dockerfiles.

## 3. Regression coverage and documentation

- [x] 3.1 Extend release and bootstrap shell tests for cache hit, cache miss, invalid fingerprint, and preserved deployment ordering.
- [x] 3.2 Update the autonomous release runbook and bootstrap initialization guidance with the marker behavior and timing output.
- [ ] 3.3 Run focused tests, strict OpenSpec validation, `make remote-release-verify`, and `git diff --check`; record the first cache-miss deployment and a later cache-hit release.

## Workflow follow-up

- Merge the reviewed change into protected `main` before deploying the optimized release path.
- Archive this change after release verification and timing evidence are recorded.
