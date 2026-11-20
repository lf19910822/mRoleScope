# Tasks

## 1. Regeneration and language behavior

- [x] 1.1 Clear the pending draft at the start of a new request and verify the old draft is absent while the request is running.
- [x] 1.2 Require Simplified Chinese explanation output, reject non-Chinese text to the fallback, and verify the provider prompt/validation.

## 2. Validation and live verification

- [x] 2.1 Run full Go/frontend tests, OpenSpec strict validation, and `git diff --check`; deploy and verify regeneration plus Chinese explanation in the browser. Revision `7160e8b` showed a Chinese explanation; browser regeneration changed pending draft rows from 8 to 0 immediately, then loaded 8 new rows after completion.

## Workflow follow-up

- Merge through protected `main` before deployment.
- Archive the change after live verification.
