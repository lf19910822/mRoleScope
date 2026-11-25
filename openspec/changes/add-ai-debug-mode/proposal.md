# Proposal

## Why

The public AI trend stream intentionally hides the final Intent and tool
payloads, while the aggregate audit stores only hashes and counters. That is
insufficient for diagnosing model planning errors such as a category query
being reduced to one ranked candidate.

This change adds a separately gated, operator-only AI debug mode that exposes
the complete bounded model/tool exchange for one controlled run without
persisting the normal Intent hash audit. The same controlled trace is also
exported to the approved Langfuse Cloud Japan project so an operator can
inspect the run in a trace waterfall.

## What Changes

- Add a debug trace contract covering model requests/responses, tool calls and
  results, validation/correction turns, final Intent, Draft, and terminal
  status.
- Add an operator-only internal NDJSON debug stream that runs the same trend
  orchestration without the public web UI contract.
- Export only controlled Debug runs to Langfuse Cloud Japan over OTLP/HTTP,
  including model generations, tool observations, corrections, Intent, Draft,
  terminal status, token usage, and bounded metadata.
- Return a Langfuse trace ID and clickable trace URL in the debug stream.
- Flush the Langfuse exporter for at most 20 seconds at Debug completion;
  exporter failures SHALL not fail the local Debug result.
- Gate the stream behind an explicit runtime debug setting and loopback-only
  access; keep it disabled by default.
- Keep normal public events, aggregate audit retention, and Intent hashing
  unchanged for ordinary requests.
- Ensure debug traces never include authorization headers, provider API keys,
  database connection strings, or other runtime credentials.
- Use the Langfuse Hobby plan's default 30-day data access window; custom
  deletion scheduling is out of scope.

## Capabilities

### New Capabilities

- `ai-debug-trace`: Controlled capture and streaming of complete AI trend
  orchestration exchanges for operator diagnosis.

### Modified Capabilities

- None.

## Impact

The affected areas are `internal/aitrend` model/provider and runner seams,
OpenTelemetry/OTLP exporter wiring, the internal HTTP server, runtime
configuration, and migration-free operator tooling/tests. The existing public
trend endpoint and aggregate audit schema remain compatible and are not
expanded to store raw model exchanges. The Langfuse project ID and keys are
runtime-only values supplied by the operator.
