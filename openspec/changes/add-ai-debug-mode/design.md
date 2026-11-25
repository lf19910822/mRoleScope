# Design

## Context

The public trend endpoint accepts an authenticated browser request and emits
display-safe NDJSON. `TrendRunner` owns classification, the model/tool loop,
Draft construction, and aggregate audit persistence. `QwenIntentProvider` is
the only boundary that constructs provider requests and decodes provider
responses. The current `AuditStore` deliberately stores hashes and bounded
counters, so it is not a suitable raw trace sink.

## Goals / Non-Goals

**Goals:**

- Make every model turn observable, including the exact bounded request body,
  response body, tool calls, tool payloads, correction errors, final Intent,
  Draft, and terminal outcome.
- Make a single controlled run easy to capture as NDJSON for offline analysis.
- Keep the debug path disabled unless explicitly enabled and reachable only
  from loopback/operator execution.
- Ensure debug runs do not create normal aggregate audit rows or Intent hashes.
- Let operators inspect the same run in Langfuse Cloud without making Langfuse
  availability part of the AI request's success path.

**Non-Goals:**

- No public-page toggle, frontend UI, or ordinary user access to debug traces.
- No change to public event redaction or aggregate audit retention.
- No persistence of debug payloads in PostgreSQL.
- No Langfuse export for ordinary public trend requests.

## Decisions

1. **Use an explicit debug trace sink in the orchestration context.** The
   runner and provider can emit structured records without coupling the model
   adapter to HTTP or a database. A no-op sink preserves the normal path.
2. **Expose a separate loopback-only internal NDJSON handler.** Reusing the
   public endpoint would make raw prompts and tool results part of the web
   contract. The internal handler accepts one bounded query, invokes the same
   runner through a debug method, and streams records to the operator.
3. **Capture provider bodies but never headers.** Request and response JSON are
   the evidence needed to reproduce planning decisions; authorization headers,
   environment values, and connection strings are excluded at the boundary.
4. **Skip aggregate audit in debug runs.** The operator stream is the source
   artifact for the controlled run. This prevents raw exchanges from being
   represented as a misleading public audit record and specifically avoids the
   normal Intent hash path.
5. **Bound each trace.** Query length, event count, request/response bytes, and
   total duration use the existing AI limits or stricter debug limits so a
   malformed provider cannot turn the diagnostic endpoint into unbounded
   storage or memory use.
6. **Use OpenTelemetry OTLP/HTTP instead of a Langfuse SDK.** Langfuse
   documents `/api/public/otel` for HTTP/JSON and HTTP/protobuf, which keeps the
   Go dependency surface small and lets the exporter be disabled when runtime
   keys are absent. The Japan endpoint and project ID are runtime settings.
7. **Dual-write local and remote records.** The existing recorder remains the
   authoritative local stream. A Langfuse adapter observes the same events,
   maps model calls to `generation`, tool calls to `tool`, and the run to one
   root trace. Export errors become trace status metadata and never replace the
   local result.
8. **Flush only at controlled-run completion.** The adapter uses a bounded
   20-second flush deadline and returns an export status event containing the
   trace ID/URL when known. Public requests do not create an exporter or flush.

Alternatives considered: adding raw fields to `ai_trend_request_audits` was
rejected because it would mix sensitive debug data with retained production
audit; adding a debug query parameter to the public endpoint was rejected
because it would create a browser-accessible raw-data path.

## Risks / Trade-offs

- [Raw prompts can contain the operator's query and model context] → Require
  explicit enablement, loopback access, bounded runs, and operator-controlled
  output capture; never log the trace through the normal application logger.
- [A provider response may contain unexpected fields] → Store the bounded raw
  body as an opaque debug payload while excluding request headers and runtime
  secrets; reject oversized records.
- [The debug path can drift from production behavior] → Invoke the same
  `TrendRunner` and registry, changing only the trace sink and audit policy.
- [Complete payloads may be large or contain sensitive business data] → Export
  only Debug runs, cap observation payloads, omit credentials, use the
  operator-approved Japan project, and rely on the Hobby plan's 30-day access
  window without claiming custom deletion.
- [OTLP exporter dependencies increase binary size and failure modes] → Keep
  the exporter behind an optional runtime configuration and test the local
  no-export path independently.
