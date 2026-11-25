# Tasks

## 1. Trace contract and provider instrumentation

- [x] 1.1 Add bounded debug trace types, context helpers, and a recorder seam; verify unit tests cover event ordering, size limits, and secret/header exclusion.
- [x] 1.2 Instrument Qwen classification, intent/tool-loop, explanation, and assistant calls to emit request/response exchange records; verify provider tests capture bodies without authorization headers.
- [x] 1.3 Emit tool call/result, validation correction, final Intent, Draft, and terminal records from the existing runner/orchestrator; verify an integration-style runner test reconstructs the complete sequence.
- [x] 1.4 Add an optional OTLP/HTTP Langfuse adapter that maps the debug events to a root trace and child observations; verify mapping and disabled-export tests.
- [x] 1.5 Add trace ID/URL and export status records with a bounded 20-second flush; verify success, missing configuration, upload failure, and timeout tests.

## 2. Controlled internal execution path

- [x] 2.1 Add explicit runtime configuration for debug enablement and bounded limits, disabled by default; verify configuration tests reject unsafe defaults and invalid limits.
- [x] 2.2 Add Langfuse endpoint/project/key runtime configuration without exposing secrets; verify configuration tests and redacted diagnostics.
- [x] 2.3 Add a loopback-only internal NDJSON debug handler and wire it separately from the public browser route; verify HTTP tests reject disabled, non-loopback, malformed, and oversized requests.
- [x] 2.4 Ensure debug runs bypass normal aggregate audit/Intent hash persistence while public runs remain unchanged; verify with audit seam tests.
- [x] 2.5 Return trace links in the internal debug stream and document the Japan Hobby project setup and 30-day access window; verify the documented operator flow.

## 3. Verification

- [x] 3.1 Run focused Go tests, `go vet ./...`, `git diff --check`, and the full project validation commands required by the repository; record any unavailable external-database checks explicitly.
