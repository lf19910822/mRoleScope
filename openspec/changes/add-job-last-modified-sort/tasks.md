## 1. History derivation and serving

- [x] 1.1 Add tests for latest qualifying daily modification and reopening,
      including no history, close, reanalysis, gaps, same-day runs and repairs.
- [x] 1.2 Implement a bounded, history-consistent list derivation without
      per-result snapshot queries; verify PostgreSQL fixture parity with history.
- [x] 1.3 Extend list API sort validation, nullable response and OpenAPI; test
      directions, nulls, multiple sort rules, stable pagination and scopes.

## 2. List presentation

- [x] 2.1 Extend route state, controls and API serialization for both sort
      directions and add focused frontend tests.
- [x] 2.2 Display the date or `-` in every result card's existing metadata
      layout; cover date, missing value and existing interactions with tests.

## 3. Verification

- [x] 3.1 Run focused Go, PostgreSQL integration and frontend tests, format,
      build, strict OpenSpec validation and `git diff --check`; report skips.
- [x] 3.2 Measure a representative query plan/timing and verify desktop and
      390px browser presentation, identifying fixture versus production data.
