# Tasks

## 1. Serving category contract

- [x] 1.1 Add failing Serving and HTTP contract tests for categorized
  technology filter options, deterministic ordering, and the controlled
  unknown-kind fallback; verify focused Go tests first fail.
- [x] 1.2 Map dictionary kinds to public categories in the existing
  filter-options projection, extend the typed API contract and OpenAPI schema,
  and verify focused Go tests pass without a new endpoint.

## 2. Category-first selector

- [x] 2.1 Add failing Vue component tests for category browsing, global
  case-insensitive term search, and selection preservation; verify the tests
  first fail against the flat selector.
- [x] 2.2 Implement the accessible two-pane A selector and integrate it only
  into the technology-keyword field; verify emitted `technology` selection
  values and existing list URL tests pass.
- [x] 2.3 Update the JD browser delivery guide with the category-first and
  global-search interaction; verify no internal taxonomy or processing detail
  is described as user-facing data.

## 3. Acceptance

- [ ] 3.1 Run focused Go tests, Vue unit tests, type checking, production
  build, strict OpenSpec validation, and `git diff --check`; manually inspect
  desktop and 390px selector states at the fixed `127.0.0.1:5175` preview
  entrypoint.
