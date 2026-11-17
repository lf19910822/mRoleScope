# Tasks

## 1. Red tests

- [x] 1.1 Extend the local release orchestrator test to require the exact pinned PostgreSQL Docker pull preflight before any image build, require mirror-context output, and reject direct official Registry probes as a release gate.
- [x] 1.2 Extend the CVM deployment helper test with a simulated Compose pull failure and verify safe diagnostics, no retry, and no migration or service startup.

## 2. Implementation

- [x] 2.1 Add the bounded CVM Docker daemon preflight to `scripts/release-remote-development.sh`, using the pinned PostgreSQL reference from the reviewed Compose definition and preserving dry-run behavior.
- [x] 2.2 Add failure diagnostics around the helper's fixed Compose pull, including mirror context and temporary-log cleanup without reading or printing secrets.
- [x] 2.3 Update the autonomous release runbook with the authoritative network path and failure interpretation.

## 3. Verification

- [x] 3.1 Run focused release/helper shell tests, deployment configuration checks, strict OpenSpec validation, and `git diff --check`.
- [x] 3.2 Run the repository release verification target if the local environment supports it; report any environment-only skip without calling it a successful verification.
