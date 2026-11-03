# Tasks

## 1. Dictionary contract

- [x] 1.1 Add failing synthetic tests for v0.3 `Go`/`Golang` alias identity,
  ASCII case-insensitive matching, identifier boundaries, and approval
  eligibility while proving v0.2 remains unchanged.
- [x] 1.2 Add immutable v0.3 source documentation, seed, and fixed allowlist;
  make the dictionary tests pass.

## 2. Controlled reanalysis and upgrade

- [x] 2.1 Add failing synthetic lifecycle tests for v0.3 approval and a
  v0.2-to-v0.3 atomic upgrade that preserves historical v0.2 facts.
- [x] 2.2 Implement fixed v0.3 candidate approval and release-gate paths;
  make the lifecycle tests pass.
- [x] 2.3 Wire fixed worker, Compose, systemd, bootstrap, and GitHub Actions
  operations for v0.3 without opening a network/data boundary.

## 3. Verification

- [x] 3.1 Run focused Go tests, full Go tests, deployment configuration checks,
  strict OpenSpec validation, and `git diff --check`.
- [x] 3.2 Record the deferred deployment sequence; do not alter CVM data until
  separately authorized.
