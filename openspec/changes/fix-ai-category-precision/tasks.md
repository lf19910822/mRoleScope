# Tasks

## 1. Category precision and explanation safety

- [x] 1.1 Add red/green tests for Chinese intent prompt guidance and programming-language candidate restriction.
- [x] 1.2 Add the deterministic category fallback and the 80-character explanation margin; verify provider and runner tests.

## 2. Validation and live verification

- [x] 2.1 Run full Go/frontend tests, OpenSpec strict validation, and `git diff --check`; deploy and verify the real programming-language query. Revision `8b5f0d2` returned only C++/C#/Go, no non-language technologies, and a complete 71-character Chinese explanation.

## Workflow follow-up

- Merge through protected `main` before deployment.
- Archive the change after live verification.
