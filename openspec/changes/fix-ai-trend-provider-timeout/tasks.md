# Tasks

## 1. Provider and intent contract

- [x] 1.1 Lower the fixed Qwen trend output budget to 1200 and verify the request profile in the synthetic provider test.
- [x] 1.2 Clarify prompt expression/metric separation and verify category wording does not produce metric names as expressions.

## 2. Verification

- [ ] 2.1 Run Go trend tests, OpenSpec strict validation, `git diff --check`, deploy the fix, and repeat the real input-to-application business test with token/audit checks.

## Workflow follow-up

- Merge through protected `main` before the real deployment test.
- Archive the change after live business verification.
