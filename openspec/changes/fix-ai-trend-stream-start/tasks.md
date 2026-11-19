# Tasks

## 1. Stream contract and orchestration

- [x] 1.1 Add the controlled `analyze_intent` operation name, allow it through HTTP event validation, and verify model operation events contain no parameters or provider content.
- [x] 1.2 Emit model `running` before every analyzer call and `completed`/`failed` after it, including validation-retry ordering; verify with orchestrator tests that the first event precedes the analyzer's delayed response.
- [x] 1.3 Keep model operations out of Serving tool quotas and audit tool counts while retaining AI attempt/token accounting; verify with runner audit tests.

## 2. Consumer and live verification

- [x] 2.1 Verify the existing frontend operation window renders model operations and keeps tool, draft, stop, and terminal behavior unchanged with frontend tests.
- [x] 2.2 Run Go tests, frontend tests, OpenSpec strict validation, and `git diff --check`; deploy the change and verify a real request receives `analyze_intent/running` before the first tool event. Live request on revision `f911f15` received the first event in 0.145s, the first Serving tool at 30.654s, completed in 35.228s, returned 39 events, and produced 8 curves successfully.

## Workflow follow-up

- Merge through protected `main` before deployment.
- Archive the change after live verification.
