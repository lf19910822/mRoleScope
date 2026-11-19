# Tasks

## 1. Contracts and storage boundaries

- [x] 1.1 Define the versioned structured intent, draft-curve, safe-error, and streaming-event contracts, including the 20-call, 100-candidate, eight-curve, three-attempt, and batch-eight limits, and verify Go contract tests reject unknown fields and invalid enum combinations.
- [x] 1.2 Add the bounded AI trend-audit schema and retention indexes for irreversible request identity, validated intent outcome, tool status/error codes, aggregate token/call/time data, final state, and draft summaries, and verify migration integration tests never persist raw user text, raw tool output, JD content, credentials, or model responses.

## 2. Deterministic Serving summaries and registered tools

- [x] 2.1 Add a Serving-owned summary reader that reuses current formal releases, snapshot completeness, real zero/missing-day semantics, event definitions, and Asia/Shanghai ranges, and verify unit and integration tests for average, period-end, peak, unique, event, change, zero, missing, and incomplete-release cases.
- [x] 2.2 Implement registered single and batch tools for formal categories, category values, project/job-family/location values, deterministic ranking, trend summaries, event summaries, and draft validation, and verify each tool enforces current-value checks, bounded output, batch size eight, safe errors, and no raw database boundary crossing.
- [x] 2.3 Add deterministic candidate ordering and identity handling for exact/alias/possible matches, stable formal-name ties, universal eight-curve truncation, exclusion values, duplicate merging, and job-family secondary-family distinction, and verify focused resolver and ranking tests.

## 3. Intent orchestration and draft validation

- [x] 3.1 Add the Qwen intent prompt/schema adapter and deterministic metric mapping for default average, period-end, peak, unique, event, absolute-change, and relative-change selection, and verify malformed, unsupported, and conflicting time-range intents produce safe correction feedback.
- [x] 3.2 Implement the controlled Go orchestrator that lets the model choose only registered tools, enforces the 20-call/time/token budgets, uses finite summaries, retries only invalid AI parameters up to three total attempts, and verifies tests distinguish AI-parameter retries from backend failures.
- [x] 3.3 Implement natural-language resolution for categories, aliases, bounded similarity, up to three candidates per expression, multi-dimension ambiguity, exclusion phrases, wider-category expansion, unsupported-subrequest reporting, and actionable no-result examples, and verify representative Chinese and English-form technical expressions.
- [x] 3.4 Implement multi-sub-question allocation, one-chart grouping, at-least-one-per-valid-subquestion reservation, all-zero drafts with warnings, partial candidate failures, global-scope fallback explanations, and final draft revalidation against the active publication, and verify draft contract tests preserve valid rows and never invent values.

## 4. Streaming API, cancellation, errors, and audit

- [x] 4.1 Add the authenticated single-request HTTP streaming route that accepts one natural-language request and emits newline-delimited JSON operation events, filtered tool names/parameters, draft events, and terminal success/partial/failure/user-stopped events, and verify HTTP tests cover flushing, event order, safe payloads, access control, and malformed input.
- [x] 4.2 Connect request cancellation to the orchestrator, emit distinct validation-correction versus backend-failure events, continue independent work after valid-parameter backend failures, retain partial drafts after user stop, and verify tests cover duplicate-submit prevention semantics, stop state, no automatic backend retry, and all-backend-failure without a retry action.
- [x] 4.3 Persist only the bounded trend-audit summary and implement 30-day/10,000-record oldest-first cleanup with warning-only cleanup failure, and verify capacity, cleanup, restart, and redaction tests.

## 5. Trend-dashboard draft experience

- [x] 5.1 Add Vue state and a streaming JSON client for the temporary AI draft, operation list, stop action, independent-request lifecycle, successful replacement of old drafts, preservation after failure/stop, and application while a newer request streams, and verify state/parser tests for all terminal states.
- [x] 5.2 Add the trend-page input, grouped multi-subquestion draft rows, concise server-derived reasons, default selections, possible/expansion/global-scope explanations, zero/all-zero notices, unsupported-feature notices, exclusion notices, no-result example buttons, and explicit replace/append/application controls, and verify component tests for user-visible decisions.
- [x] 5.3 Add collapsed operation history with a current-page expand action, current-page cleanup on refresh/navigation, formal-label behavior after application, and 390px/desktop responsive layout coverage, and verify existing trend dashboard tests, frontend build, and responsive verification scripts.

## 6. End-to-end verification and documentation

- [x] 6.1 Add an end-to-end fake-tool/Serving test that exercises category mapping, ranking, batch summaries, partial failure, AI parameter correction, backend failure continuation, cancellation, old-draft preservation, and final draft replacement, and verify the complete stream and draft state.
- [x] 6.2 Add security and contract checks proving no arbitrary SQL, JD content, credentials, internal release identifiers, raw model reasoning, or unbounded result payload reaches the public stream, and verify `go test ./...`, the web test/build commands, and `git diff --check`.
- [x] 6.3 Update the AI 作图 guidance and operational documentation with the finalized entry scope, tool limits, retry responsibility, stream event contract, audit retention, and user-visible fallback rules, and verify all documented limits match the OpenSpec spec and implementation contracts.

## Workflow follow-up

- Archive the change only after implementation, strict OpenSpec verification, and review requirements are complete.
- Start `$openspec-apply-change` explicitly before implementing any tracked task.

## 7. Backend runtime integration follow-up

- [x] 7.1 Add the fixed Qwen Plus intent adapter with the versioned intent schema, bounded response parsing, cancellation, and provider-safe errors; verify credentials and raw provider content never enter the public stream.
- [x] 7.2 Add the Serving-backed trend runner that resolves current formal values, batches bounded summaries, ranks candidates deterministically, revalidates drafts, and emits the existing NDJSON event contract.
- [x] 7.3 Inject the runner into the authenticated API, pass the existing root-owned Qwen runtime configuration through the private AI egress network, and keep the route unavailable when provider configuration is absent.
- [x] 7.4 Remove the frontend local mock path so production and development state consume only the AI draft stream; retain a transport-level NDJSON fixture in unit tests.
- [x] 7.5 Read Qwen prompt/completion usage and persist it with the live runner's bounded trend-audit summary, including tool-call status and duration, without storing raw requests, responses, credentials, or tool payloads.
