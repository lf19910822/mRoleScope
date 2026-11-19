# Tasks

## 1. Formal trend project boundary

- [x] 1.1 Exclude `unrecognized` from AI trend project options and resolver candidates while keeping job-filter options unchanged; verify tool and resolver tests.
- [x] 1.2 Verify broad project planning never emits `unrecognized` batch requests and formal project drafts remain applicable.
- [x] 1.3 Make batch trend summaries preserve successful items and report per-item safe errors; verify partial-result and audit tests.

## 2. Validation and live verification

- [x] 2.1 Run Go/frontend tests, OpenSpec strict validation, and `git diff --check`; deploy and repeat the real project query/application test with a mixed-success batch. Revision `0946b95` completed both project summary batches without `unrecognized`, returned 8 curves, and completed validation plus AI explanation; unit tests cover mixed-success batches retaining valid summaries and marking the request partial when needed.

## Workflow follow-up

- Merge through protected `main` before deployment.
- Archive the change after live verification.
