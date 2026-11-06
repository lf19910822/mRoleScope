# Tasks

## 1. Release contract

- [x] 1.1 Define the local-main, immutable-digest and CVM-boundary contract.

## 2. Implementation

- [x] 2.1 Add the local release verification target, including a disposable
      PostgreSQL integration database and all existing backend/frontend/deploy
      checks.
- [x] 2.2 Add a release command that builds, labels, pushes, resolves and
      validates the four digest-addressed images before invoking the CVM
      restricted deployment helper.
- [x] 2.3 Add isolated tests for dry-run, successful four-image release and
      fail-closed label mismatch behavior.
- [x] 2.4 Write the operator runbook and persistent agent pointer.

## 3. Verification

- [x] 3.1 Run the release-command test, deployment configuration checks,
      strict OpenSpec validation and `git diff --check`.
