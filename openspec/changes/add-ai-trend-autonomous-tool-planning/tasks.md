# Tasks

## 1. Tool session contract and category snapshot

- [x] 1.1 Define the model-turn result contract for a final Intent or a bounded tool call, expose only the approved trend tools, and verify strict decoding rejects unknown tool names, fields, and malformed arguments.
- [x] 1.2 Build a request-scoped technology category snapshot from Serving filter options, including formal category values, display names, and category members; verify the snapshot is reused by category tools without a second database read.
- [x] 1.3 Make `list_available_categories` and `list_category_values` accept formal category values or display names, normalize to one formal value, and verify unknown categories return a safe unavailable-value error without wildcard expansion.

## 2. Qwen model-tool orchestration

- [x] 2.1 Extend the Qwen adapter to send bounded tool definitions and parse either tool calls or the final `ai-trend-draft-v1` JSON while retaining provider usage counts; verify mocked responses for a tool call, a tool result continuation, and a final Intent.
- [x] 2.2 Implement the model-tool loop with the existing 20-call budget, request timeout, three Intent correction attempts, and cancellation behavior; verify exhaustion and cancellation produce partial or stopped results without extra calls.
- [x] 2.3 Feed only bounded, display-safe tool results back to the model and emit filtered operation events for each tool call; verify SQL, credentials, JD text, raw daily points, and unfiltered provider payloads never reach events or audit records.
- [x] 2.4 Replace the category-specific Intent prompt branches with generic scope, wildcard, time-range, metric, exclusion, and result-limit rules; verify prompt contract tests cover explicit values, category values, unrestricted dimensions, unknown categories, and date phrases.

## 3. Generic candidate resolution and result limits

- [x] 3.1 Replace hard-coded technology-category fallback logic with catalog-driven category resolution; verify a new category fixture is handled without adding a category-specific code branch and that category candidates exclude adjacent categories.
- [x] 3.2 Enforce each sub-query's `ranking.limit` during deterministic candidate allocation while preserving the shared eight-curve cap and per-sub-query reservation rule; verify top-one, top-N, insufficient candidates, duplicate curves, and multiple sub-queries.
- [x] 3.3 Ensure wildcard expansion occurs only for an unrestricted dimension and that unknown or conflicting category expressions remain unresolved; verify no category-limited query emits a whole-technology trend request.
- [x] 3.4 Preserve per-item batch trend failures and successful candidates through ranking, validation, explanation, and final draft creation; verify a mixed-success batch still yields a partial draft and the correct terminal status.

## 4. End-to-end verification and documentation

- [x] 4.1 Add an end-to-end fake-provider test for “使用最多的编程语言是什么” that observes category discovery, category-value lookup, bounded trend summaries, `limit=1`, and a single language curve in the final draft.
- [x] 4.2 Add end-to-end cases for explicit “比较 Go 和 Python”, unrestricted “哪些技术关键词”, unknown category, database category, invalid tool arguments, and backend batch failure; verify tool order, safe errors, curve scope, and partial delivery.
- [x] 4.3 Update `Docs/开发指导/AI作图_01/AI作图开发决策.md` with the implemented model-tool loop, generic category discovery, and `ranking.limit` behavior; verify the document records the final 20-call and error-responsibility boundaries.
- [x] 4.4 Run `go test ./...`, the relevant frontend unit/type checks, `openspec validate add-ai-trend-autonomous-tool-planning --strict`, and `git diff --check`; record any unavailable real-provider or database checks explicitly.
