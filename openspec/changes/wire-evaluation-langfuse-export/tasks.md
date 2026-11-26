# Tasks

## 1. Remote evaluation wiring

- [x] 1.1 Add the explicit Langfuse evaluation switch and four endpoint/project/key environment mappings to both isolated evaluation services, then verify the Compose diff contains no secret literals or published ports.
- [x] 1.2 Extend the remote AI evaluation checker and its test fixture to require all five Langfuse mappings while preserving secret-value redaction, then verify both checker commands pass.

## 2. Operator documentation

- [x] 2.1 Document the Langfuse evaluation preflight, case-level Session lookup, and the required stop point before `run-trend`, then verify the command and configuration names match the deployed helper.

## 3. Verification and release readiness

- [x] 3.1 Run focused Go tests, remote evaluation/Compose checks, strict OpenSpec validation, and `git diff --check`; verify no evaluation process is started.
- [x] 3.2 Deploy the reviewed change through the approved release path and verify the CVM revision and redacted runtime key presence, then stop before invoking `run-trend`.

## Workflow follow-up

- After explicit operator confirmation, run the fixed `run-trend` helper and inspect the resulting case Sessions in Langfuse.
