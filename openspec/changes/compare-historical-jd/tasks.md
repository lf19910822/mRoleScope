# Tasks

## 0. Contract and TDD preparation

- [x] 0.1 Confirm public comparison types, historical detail seam, pure diff
      seam, and detail component seam.
- [x] 0.2 Add OpenAPI and TypeScript contract fixtures without internal fields.

## 1. Pure comparison engine

- [x] 1.1 Add red tests for dimension set comparison, formatting, empty values,
      and urgent/title scalar changes.
- [x] 1.2 Add red tests for section additions/removals, list reordering,
      character fragments, whitespace normalization, and Unicode text.
- [x] 1.3 Implement deterministic comparison functions and verify green.

## 2. Serving historical detail

- [x] 2.1 Add red repository tests for same-transaction historical/current
      reads, changed/current/closed mode boundaries, empty comparison, and
      comparison-unavailable degradation.
- [x] 2.2 Implement the read-only comparison query and display-safe response.
- [x] 2.3 Add route/OpenAPI tests for optional comparison and safe errors.

## 3. Detail reader UI

- [x] 3.1 Add red component tests for independent tags, counts, collapsed
      state, dimension values, and body fragment semantics.
- [x] 3.2 Implement diff rendering, light/dark tokens, and responsive layout.
- [x] 3.3 Add retry behavior that preserves the historical reader and context.

## 4. Verification

- [x] 4.1 Run focused Go/Vitest tests, type-check, build, and `git diff --check`.
- [x] 4.2 Run strict OpenSpec validation and record results.
- [ ] 4.3 Verify desktop and 390px historical changed/empty/unavailable states
      against real data (blocked until `TEST_DATABASE_URL` is available; the
      component and responsive-style tests are complete).
