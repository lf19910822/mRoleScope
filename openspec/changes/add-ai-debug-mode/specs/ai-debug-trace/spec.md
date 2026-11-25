# Spec Delta

## Purpose

This capability provides a controlled diagnostic trace for understanding how
the AI, tools, validation loop, and deterministic Draft builder reached a
trend result, without making raw model exchanges part of the public service.

## ADDED Requirements

### Requirement: Debug mode is explicitly gated

The system SHALL keep AI debug mode disabled by default and SHALL require an
explicit runtime enablement before accepting a debug run.

#### Scenario: Disabled debug mode
- **WHEN** an operator requests a debug run while debug mode is disabled
- **THEN** the system rejects the request without invoking the model or tools

#### Scenario: Enabled debug mode
- **WHEN** debug mode is explicitly enabled and a bounded operator request is
  received through the internal diagnostic path
- **THEN** the system runs the same trend orchestration with debug recording

### Requirement: Debug trace exposes the complete bounded interaction

An accepted debug run SHALL emit ordered records for model exchanges, tool
calls and results, validation or correction feedback, the final Intent, Draft,
and terminal status, subject to configured size and duration limits.

#### Scenario: Model narrows a category
- **WHEN** the model discovers a category, requests summaries, and returns a
  final Intent with a ranking limit
- **THEN** the trace includes the category/tool evidence, final Intent, and
  Draft so an operator can identify where candidates were narrowed

#### Scenario: Tool correction
- **WHEN** a model tool call fails with a correctable contract error and a later
  turn retries it
- **THEN** the trace includes the failed call, safe error, correction turn, and
  corrected call in order

### Requirement: Debug output is isolated from the public service

The diagnostic path SHALL be separate from the public browser endpoint, SHALL
be reachable only from loopback or an equivalent explicitly controlled
operator channel, and SHALL not be exposed in the normal web UI.

#### Scenario: Non-loopback request
- **WHEN** a debug request arrives from a non-loopback address
- **THEN** the system rejects it without starting a run

#### Scenario: Public endpoint unchanged
- **WHEN** a normal trend request is sent to the public endpoint
- **THEN** it emits the existing display-safe stream and does not include raw
  provider exchanges or final Intent JSON

### Requirement: Debug traces do not create production Intent-hash audit rows

An accepted debug run SHALL emit its trace to the operator stream and SHALL
skip the normal aggregate audit persistence, including Intent hashing.

#### Scenario: Debug run completion
- **WHEN** a debug run completes successfully or fails
- **THEN** the trace contains the terminal outcome and no production audit row
  is written for that run

### Requirement: Credentials are excluded from debug traces

The debug recorder SHALL never emit authorization headers, provider API keys,
database connection strings, session tokens, invite codes, or equivalent
runtime credentials.

#### Scenario: Provider exchange capture
- **WHEN** a provider request contains an authorization header and a response
  contains model JSON
- **THEN** the trace includes the bounded request/response bodies only and
  excludes the authorization header and runtime credential values

### Requirement: Controlled Debug runs export to Langfuse

When Langfuse runtime configuration is complete, an accepted Debug run SHALL
create one Langfuse root trace and export its bounded model, tool, validation,
Intent, Draft, terminal, token, duration, and metadata records over OTLP/HTTP.

#### Scenario: Langfuse trace is created
- **WHEN** an enabled Debug run completes with valid Langfuse configuration
- **THEN** the trace contains the run hierarchy and the debug response exposes
  a trace ID and URL for the Japan Cloud project

#### Scenario: Langfuse is not configured
- **WHEN** an enabled Debug run has no complete Langfuse configuration
- **THEN** the local Debug stream still runs and no Langfuse export is attempted

### Requirement: Langfuse failures do not block local debugging

The Langfuse exporter SHALL be best effort and SHALL not change the local
Debug result when export, network, or flush fails; the local stream SHALL
report the export status and bounded error instead.

#### Scenario: Export request fails
- **WHEN** Langfuse rejects a batch or the network is unavailable
- **THEN** the local NDJSON still contains the complete available trace and a
  failed export status, and the request's AI result is preserved

#### Scenario: Flush timeout
- **WHEN** the exporter has not flushed within 20 seconds after Debug completion
- **THEN** the local request ends with a bounded timeout status and does not wait
  indefinitely

### Requirement: Langfuse configuration is runtime-only

The Langfuse endpoint, project ID, public key, and secret key SHALL be read
from runtime configuration; the secret key SHALL never be exposed in events,
logs, URLs, source files, or browser payloads.

#### Scenario: Runtime configuration
- **WHEN** the operator supplies the Japan Cloud endpoint, project ID, public
  key, and secret key in the protected runtime environment
- **THEN** the exporter authenticates without writing those values to the
  repository or public stream
