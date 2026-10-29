# Tasks

## 1. Contract and red tests

- [x] 1.1 Add parser regressions for the 17 audited `unclassified` titles and
  the two approved cross-family cases; verify they fail under V1.

## 2. V2 dictionary

- [x] 2.1 Add a separate immutable V2 asset with only the approved aliases;
  retain V1 and switch the current embedded asset to V2.
- [x] 2.2 Verify the focused V2 parser tests and existing parser regression
  suite pass, including bounded ASCII matching and the retained unclassified
  domain-only title.

## 3. Documentation and validation

- [x] 3.1 Record V2 aliases and retained `unclassified` boundary in the
  job-family workflow plan.
- [x] 3.2 Run the relevant Go test suite, strict OpenSpec validation, and
  `git diff --check`; record results without deploying or processing history.
