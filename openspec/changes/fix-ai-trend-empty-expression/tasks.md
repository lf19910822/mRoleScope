# Tasks

## 1. Broad expression fallback

- [x] 1.1 Treat empty normalized expressions as broad dimension queries and preserve exclusions, expansion labels, and candidate bounds; verify resolver tests.
- [x] 1.2 Skip `rank_values` for empty or broad expressions while retaining summary-based ranking; verify planner tests contain no empty rank parameter.
- [x] 1.3 Verify the runner produces ranked project curves from an empty expression and keeps the existing specific-value behavior; add regression tests.

## 2. Validation and live verification

- [ ] 2.1 Run Go tests, frontend tests, OpenSpec strict validation, and `git diff --check`; deploy and run a real “哪些项目的岗位最多？” request through draft application.

## Workflow follow-up

- Merge through protected `main` before deployment.
- Archive the change after live verification.
