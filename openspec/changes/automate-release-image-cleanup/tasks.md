# Tasks

## 1. Contract and test preparation

- [x] 1.1 Add an isolated red deployment-helper test covering current and
      predecessor retention, stale unreferenced removal, container-reference
      protection, and foreign-image exclusion.
- [x] 1.2 Add the release-image-retention OpenSpec contract and document the
      no-global-prune/no-volume-deletion boundary.

## 2. Implementation

- [x] 2.1 Capture and validate the predecessor's four-image `current.env`
      record before a new successful release overwrites it; fail cleanup closed
      when a present record is invalid.
- [x] 2.2 Add best-effort, namespace- and digest-limited post-success cleanup
      that retains two release generations and skips images referenced by any
      container.
- [x] 2.3 Wire the isolated test into `make deploy-config-check` without
      weakening the existing deployment ownership, allowlist, rollback, or Web
      Edge checks.

## 3. Verification and rollout

- [x] 3.1 Run shell syntax checks, deployment configuration tests, strict
      OpenSpec validation, and `git diff --check`.
- [ ] 3.2 The reviewed helper was checksum-verified and installed on the CVM
      on 2026-11-06; verify the next successful deployment records and runs
      cleanup without touching volumes or live containers.
