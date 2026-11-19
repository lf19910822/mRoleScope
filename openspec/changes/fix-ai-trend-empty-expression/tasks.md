# Tasks

## 1. Broad expression fallback

- [x] 1.1 Treat empty, wildcard, and whole-dimension expressions as broad dimension queries and preserve exclusions, expansion labels, and candidate bounds; verify resolver tests.
- [x] 1.2 Skip `rank_values` for empty, wildcard, or broad expressions while retaining summary-based ranking; verify planner tests contain no invalid rank parameter.
- [x] 1.3 Verify the runner produces ranked project curves from an empty expression and keeps the existing specific-value behavior; add regression tests.
- [x] 1.4 State the exact `expressions: ["*"]` representation for broad questions in the intent prompt and verify the provider request contains the guidance.

## 2. Validation and live verification

- [x] 2.1 Run Go tests, frontend tests, OpenSpec strict validation, and `git diff --check`; deploy and run a real “哪些项目的岗位最多？” request through draft application. Revision `3a869d4` returned 8 project curves without empty or wildcard rank parameters; their measured averages were strictly descending (18.579, 14, 9.842, 9, 6, 5.211, 4.105, 2), the browser showed 8/8 rows, and all 8 Serving application requests returned HTTP 200 with 19 points. One unrelated batch summary backend failure was retained as partial status while preserving the usable draft.

## Workflow follow-up

- Merge through protected `main` before deployment.
- Archive the change after live verification.
