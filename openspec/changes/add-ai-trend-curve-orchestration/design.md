# Design

## Context

See `proposal.md` for the motivation and user-visible scope. The repository already has a Go Qwen-compatible client with strict JSON contracts, bounded retries, safety errors, budget accounting, and AI audit helpers. It also has a `jdserving.TrendReader` seam that returns one validated historical curve per request, and a Vue trend dashboard that stores up to eight editable curve rows and renders them with ECharts.

The new capability must sit above these seams. It must not read ODS/DWD tables, reuse the JD semantic-analysis tables as a draft protocol, calculate daily facts in the browser, or change the current dashboard until the user explicitly applies a draft. The current code has no AI trend route and no streaming transport.

## Goals / Non-Goals

**Goals:**

- Add a trend-dashboard-only natural-language draft flow with deterministic metric interpretation and safe formal-value resolution.
- Let AI choose and sequence registered tools while the server owns permissions, schemas, budgets, deterministic calculations, and draft validation.
- Stream operation events and final draft state through one cancellable request.
- Preserve current Serving history semantics, formal releases, missing dates, real zeros, and existing curve application behavior.
- Keep drafts, operation records, and user-facing explanations separate from long-lived bounded audit summaries.

**Non-Goals:**

- No continuous conversational memory, cross-page draft persistence, arbitrary SQL, JD-content retrieval, image generation, new chart types, percentage/event-value axes, education/experience natural-language dimensions, or compound intersection curves in this change.
- No WebSocket task broker or background job queue.
- No replacement of the existing `TrendReader` public response with AI-produced points.

## Decisions

### 1. Use a controlled Go orchestrator, not a general agent framework

The orchestrator will reuse the existing Qwen client and structured-response safety boundary. A registry will expose fixed tools with explicit input and output schemas. The model may choose a registered tool and the next call, but the server validates every call and owns the 20-call budget, time limit, candidate limits, batch limits, and total token budget.

LangChain, LangGraph, and MCP are not runtime dependencies for this capability. The repository has no such runtime today, and the single-request flow does not need cross-process discovery or resumable graphs. A future adapter can expose the same registry if tool sharing becomes necessary.

### 2. Separate intent normalization from deterministic analysis

The request pipeline has four server-visible stages:

```text
user text
  -> structured intent (Qwen)
  -> schema/business validation and formal-value resolution
  -> deterministic summary/ranking tools
  -> validated curve draft
```

The intent schema carries supported dimensions, sub-questions, normalized time range, ranking metric, order, limit, exclusions, output metric, and explanation metadata. It cannot carry SQL, arbitrary formulas, table names, database fields, or final numeric facts.

Metric mapping is deterministic. The default ranking metric is period-average active jobs. Period-end, peak, unique-in-period, event counts, absolute change, and relative change are explicit enumerations. Complex metrics select ordinary `daily_active_jobs` curves and never become a chart axis in this change.

### 3. Provide high-level registered tools with bounded summaries

The initial registry will cover the decisions recorded in the exploration log:

- list current formal technology categories;
- list values within a formal technology category;
- list current values for project, job family, and location;
- rank candidates with a fixed deterministic metric;
- read one or a batch of trend summaries;
- read one or a batch of event summaries;
- validate one or a batch of curve-draft conditions.

The tools call Serving-owned deterministic readers. Trend summaries contain only formal names, ranking metrics, peak information, event summaries, and valid/missing-day counts. They do not return full daily points to AI. Batch tools process at most eight candidates, and each result carries an independent available/unavailable/failure outcome.

### 4. Resolve expressions against the current formal publication

The resolver uses normalization, current formal aliases, and bounded similarity against current display-safe values. One expression can expand to at most three candidates. Exact and formal-alias matches outrank possible matches. Possible matches are labeled in the draft. A wider-category fallback is labeled “扩展推荐，不是精确匹配” and defaults to selected.

Category questions use period-average ranking. Multi-sub-question requests reserve one candidate for each valid sub-question before filling remaining slots. A universal eight-curve cap applies to every result, including all-zero candidates. Stable formal-name ordering resolves ties and makes truncation deterministic. Duplicate curves merge only when their complete identity matches, including the job-family secondary-family flag.

The resolver also handles exclusion phrases. When a request combines a project with a technology question and intersection curves are unavailable, the draft includes the project curve plus global technology curves. The AI explanation carries the “global scope” qualification. After application, the formal chart label remains the formal value.

### 5. Keep the draft separate from applied dashboard rows

The Vue trend view will own a temporary AI draft state in addition to the existing applied-row state. A draft row contains a validated condition, group/sub-question identity, selection reason, match label, scope explanation, selected state, and per-row availability status.

The draft is created only in the current trend page. Refreshing or leaving the page clears it and its expandable operation record. If a request succeeds, it replaces the previous unapplied draft. If it fails, is stopped, or has no usable result, the previous draft remains. A new request does not inherit prior text or draft context. An old draft remains applicable while a new request streams.

Existing application semantics remain explicit. With no applied curves, the action is “应用草稿”. With applied curves, the user chooses replace or append. Append checks the existing eight-curve limit and reports a conflict instead of silently dropping rows. Applied curves continue to use the existing dashboard and Serving APIs.

### 6. Use one POST-like HTTP stream with structured JSON events

The browser submits one request and reads a single server response stream with `fetch` and `AbortController`. The wire format is newline-delimited JSON events so a POST body can carry the natural-language request while the response remains incrementally readable. The response is served with a streaming content type and proxy buffering disabled for this route.

Events are intentionally small and typed:

```json
{"type":"operation","status":"running","tool":"rank_values","params":{"dimension":"technology"}}
{"type":"operation","status":"completed","tool":"rank_values","params":{"dimension":"technology"}}
{"type":"draft","status":"partial","curves":[...]}
{"type":"done","status":"success"}
```

Operation events may show registered tool names and filtered parameters because the user explicitly requested this transparency. They do not show result counts, raw tool output, SQL, secrets, JD text, internal stack traces, or model chain-of-thought. Final draft events carry only validated draft data and safe explanations.

The stream emits operation states, validation-retry states, draft partial/complete states, and a terminal `success`, `partial`, `failure`, or `user_stopped` state. The client disables duplicate submit, exposes “停止分析”, collapses the operation record after completion, and retains a current-page “查看操作记录” control.

### 7. Separate AI-correctable errors from backend failures

Schema, enum, dimension-combination, time-range, ordering, and curve-limit failures produce a safe structured correction message for Qwen. The total AI attempt count is three, including the first generation. The stream shows validation failure, correction, and retry.

Once parameters pass validation, database, trend-service, formal-release, timeout, and internal-tool failures are backend failures. They are marked in the operation stream and independent work continues. They do not trigger another AI attempt. If every backend path fails, the UI shows a failure message without a retry button. A user can submit a new request manually.

Stopping cancels the request context. Already applied curves remain unchanged. Candidates found before cancellation remain in a temporary draft marked “分析已停止，结果可能不完整”.

### 8. Add a separate bounded trend-audit summary

The existing JD semantic-analysis audit tables and raw-output artifact store are not the user trend draft protocol. Add a dedicated bounded audit record for trend requests. It stores only an irreversible request identity, normalized intent validation outcome, registered tool status/error codes, aggregate call/token/time values, final state, and draft summary references.

Cleanup keeps the newest 30 days and at most 10,000 records, deleting oldest entries first. Audit persistence or cleanup failure creates an operational warning and does not fail the trend request or unrelated work. The complete operation list remains page-local and is never reconstructed from the audit record.

### 9. Preserve current formal data and chart contracts

Serving summary readers reuse the current formal release selection and historical snapshot rules. Missing dates remain unavailable, complete no-match dates remain zero, and the browser never fills gaps. AI applies no alternate dictionary cache. Draft application revalidates values against the current active publication.

The current trend chart remains a single mixed-dimension岗位数量 chart. Global-scope and expansion explanations remain in the AI answer or draft, while applied chart legends use formal curve names. Existing applied curves are not overwritten by an AI request until the explicit replace/append action.

### 10. Use the existing Qwen runtime configuration for the API runner

The trend route uses a fixed Qwen Plus structured-output adapter. It reuses the
existing Beijing Workspace endpoint rule and the already approved
`MROLESCOPE_QWEN_WORKSPACE_ID` / `MROLESCOPE_QWEN_API_KEY` runtime variables;
the values are read only by the API process and are never logged, persisted, or
sent to the browser. The API is attached to the private backend, ordinary edge
connection required by Web Edge, and the dedicated `ai-edge` egress network.
Without both runtime values the trend route remains unavailable and existing
read-only trend routes continue to work.

The model returns only the versioned intent. The runner resolves that intent
against the current Serving filter publication, reads bounded trend summaries,
and builds the draft locally. The browser has no local curve fixtures or
development mock branch; unit tests fake the HTTP NDJSON response at the
transport boundary.

The Qwen adapter also returns provider-reported prompt and completion token
counts through an internal usage seam. The runner accumulates usage across
validation retries and writes it to the bounded trend-audit record together
with aggregate tool-call status and duration. A provider that does not expose
usage receives a bounded input estimate for operational visibility; this
fallback is never presented as provider billing. Audit insertion and cleanup
are warning-only and cannot fail the user-facing draft request.

## Risks / Trade-offs

- **Broad matching can add false positives** → mark possible matches and expansion recommendations, show formal names, keep all rows editable, and require explicit application.
- **Twenty tool calls can increase latency** → use batch tools, finite summaries, a server time limit, and an operation stream with stop support.
- **Tool names and parameters couple the UI to the registry** → expose only filtered registered names and parameters, keep final semantics in typed event fields, and treat registry names as a reviewed public diagnostic contract for this feature.
- **A one-chart global fallback can be misunderstood** → put the global-scope explanation in the AI answer and draft, while preserving formal chart labels after application.
- **Formal publications can change between generation and application** → revalidate every draft row at application time and mark retired values unavailable.
- **A streaming proxy can buffer events** → configure the route and edge to flush streaming responses, and test the deployed path with a fake slow tool sequence.
- **A partial draft may look complete** → show explicit partial/stopped state and keep the operation record available on demand.

## Migration Plan

1. Add the trend-audit migration and bounded cleanup job or request-time cleanup path.
2. Add the registered deterministic summary tools and orchestrator behind a new authenticated trend-draft route.
3. Add stream event contract tests and fake-tool tests before enabling the dashboard entry.
4. Add the Vue draft state, editor, stream reader, stop action, and explicit replace/append application controls.
5. Verify desktop and 390px layouts, current formal-release changes, missing/zero data, dictionary retirement, partial failures, and stream cancellation.
6. Rollback by disabling the trend-draft route and dashboard entry. Existing trend routes and applied curves remain usable. The audit table can remain inert for later re-enable.
