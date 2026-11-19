# Tasks

## 1. Explanation contract and provider

- [x] 1.1 Add the bounded explanation context/schema and Qwen explanation request with a 100-Unicode-character limit; verify provider parsing, usage, and privacy filtering.
- [x] 1.2 Add the display-only `explain_draft` operation and keep it out of Serving tool quotas/audit tool counts; verify stream contract tests.

## 2. Runner integration

- [x] 2.1 Build safe post-tool context, call the explanation provider after validation, accumulate tokens, and preserve deterministic fallback on failure; verify ordered runner and audit tests.
- [x] 2.2 Verify the existing frontend renders the explanation after operations without layout changes; run frontend unit/type tests.

## 3. Validation and live verification

- [ ] 3.1 Run full Go/frontend tests, OpenSpec strict validation, and `git diff --check`; deploy and run a real request confirming model explanation ordering and length.

## Workflow follow-up

- Merge through protected `main` before deployment.
- Archive the change after live verification.
