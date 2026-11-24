# Tasks

## 1. Error contract

- [x] 1.1 Add `invalid_value` and document the distinction from `unavailable_value`, `invalid_enum`, and `backend_failure`.
- [x] 1.2 Add red tests for unknown category, unknown formal value, invalid enum, and backend failure mappings.

## 2. Model feedback loop

- [x] 2.1 Add a red Qwen request contract test proving a model-correctable tool error is returned with a repair instruction and safe structured error.
- [x] 2.2 Implement generic correction feedback in the tool-loop prompt while preserving history, budgets, and redaction boundaries.
- [x] 2.3 Add orchestrator tests proving model-correctable errors can recover and backend errors remain partial without correction retry.

## 3. Verification

- [x] 3.1 Run focused Go tests, full Go tests, `go vet`, `git diff --check`, and strict OpenSpec validation.
- [x] 3.2 Run the existing fixture Trend/Agent regression and verify no public response leaks internal error text.
