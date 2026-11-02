# Tasks

## 1. Serving category contract

- [x] 1.1 Add failing Serving and HTTP contract tests for categorized
  technology filter options and the controlled unknown-kind fallback.
- [x] 1.2 Map dictionary kinds to public categories in the existing
  filter-options projection, extend the typed API contract and OpenAPI schema,
  and verify focused Go tests pass without a new endpoint.

## 2. Category-first selector

- [x] 2.1 Add failing Vue component tests for category browsing, global
  case-insensitive term search, and selection preservation.
- [x] 2.2 Implement the accessible two-pane A selector and integrate it only
  into the technology-keyword field; preserve emitted `technology` values and
  existing list URL behavior.
- [x] 2.3 Add a focused development note describing category-first browsing
  and global search without describing internal taxonomy to viewers.

## 3. Acceptance

- [ ] 3.1 Run Go and Vue checks, type checking, production build, strict
  OpenSpec validation, `git diff --check`, and private deployed API/UI checks
  at the fixed `127.0.0.1:5175` entrypoint.
