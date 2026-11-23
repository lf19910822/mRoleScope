# Tasks

## 1. Benchmark source and fixed evaluation data

- [x] 1.1 Create the human-reviewable Markdown benchmark with exactly 30 Chinese questions in six categories of five, including formal-value expectations, scope, ranking metric and semantics, fixed ordering rules for zero-valued candidates, error behavior, and draft pass conditions; verify the document count and required fields with a deterministic validation test.
- [x] 1.2 Define the versioned machine-readable case format and generator from the reviewed Markdown source; verify generated cases preserve canonical IDs, wildcard constraints, ranking semantics, capacity rules, and evaluation version metadata.
- [x] 1.3 Build a sanitized fixed Serving snapshot and snapshot-backed readers for evaluation-only use; verify fixture data is deterministic, contains no credentials or unnecessary JD text, and is not referenced by daily ingestion or production query configuration.

## 2. Isolated benchmark execution

- [x] 2.1 Add evaluation run and case types carrying model/provider, prompt, tool schema, snapshot, code revision, case, and evaluator version identifiers; verify an immutable evaluation version rejects missing or inconsistent metadata.
- [x] 2.2 Add an evaluation runner that reuses the production trend AI construction seam with snapshot-backed Serving interfaces and a fresh state per case repetition; verify prompt, provider, tool registry, validation, and token accounting use the production path.
- [x] 2.3 Implement controlled serial execution with three valid repetitions per case, the 20-call tool limit, and at most two infrastructure retries per sample; verify invalid AI parameters trigger correction feedback, valid-request backend failures do not trigger AI retries, and semantic failures, infrastructure failures, incomplete samples, and limit breaches receive distinct statuses.
- [x] 2.4 Add execution tests proving a benchmark run cannot write real dashboard, Serving, ingestion, or production audit state; verify application checks use isolated test state and are cleaned up.

## 3. Deterministic semantic evaluation

- [x] 3.1 Implement canonical-value normalization and semantic tool-trace checks for dimensions, explicit values, categories, wildcard scope, ranking metrics, dependency order, allowed equivalent tool paths, and display-ID separation; verify explicit-value and category-scope cases reject out-of-range curves and silent wildcard expansion.
- [x] 3.2 Implement ranking and candidate-capacity checks, including ordered results for top or top-N questions, unordered set comparison for ordinary comparisons, fixed first-eight selection even for zero-valued candidates, total candidate count, and omitted count; verify representative cases from all affected categories.
- [x] 3.3 Implement draft, partial-result, unsupported-condition, backend-failure, and advisory explanation rule checks; verify valid partial results are retained, AI parameter errors are classified for correction, backend failures do not erase valid candidates or request model retries, and explanation checks never change the hard end-to-end pass.
- [x] 3.4 Add scorer tests for the complete 30-case machine dataset and assert each case exposes a deterministic end-to-end result plus tool, scope, ranking, draft, and explanation sub-results.

## 4. Trace, cost, and report artifacts

- [x] 4.1 Implement bounded structured trace output for successful, semantic-failure, infrastructure-failure, and incomplete runs, including safe tool parameters/results, validation history, draft outcome, event timings, token counts, and estimated cost; verify chain-of-thought, credentials, and unnecessary JD text are redacted.
- [x] 4.2 Add report aggregation for per-case 0/3–3/3 counts, tool-call/query-scope/draft sub-scores, six category rates, equal-weight overall pass rate, incomplete infrastructure samples, tool calls, tokens, cost, and average/P50/P95 latency; verify incomplete runs are excluded from AI failure rates and performance metrics do not alter functional results.
- [x] 4.3 Write the report and trace format documentation under `Docs/开发指导/AI评测/`, keep detailed traces in an independent evaluation artifact directory that is not exposed to ordinary users, and verify the documented paths do not overlap Serving or daily audit storage.

## 5. End-to-end evaluation command and integration checks

- [x] 5.1 Provide a local or isolated command to generate the first benchmark report from the reviewed cases and fixed snapshot; verify it runs serially, emits version metadata, and produces a report without requiring production credentials or changing production state.
- [x] 5.2 Add end-to-end tests using a controllable model/provider fixture to exercise tool discovery, trend summary, draft validation, partial results, retry classification, token usage, and stream timing through the real orchestrator seam.
- [x] 5.3 Add separate application integration checks for applying a validated draft and rendering the resulting curves in isolated state; verify application failures are reported separately and are absent from the AI benchmark score.
- [x] 5.4 Add a separate CVM real-chain smoke-test command or documented procedure; verify its results are labeled separately, excluded from the fixed benchmark report, and do not modify production data or dashboards.
- [x] 5.5 Run the first 30-question baseline, review the generated report and representative traces, and record the baseline artifact without adding a release gate; verify the baseline is labeled with its immutable evaluation version.

## 6. Change validation

- [x] 6.1 Run `openspec validate --strict --change add-ai-evaluation-benchmark` and resolve all artifact or scenario validation errors; verify proposal, spec, design, and tasks remain consistent.
- [x] 6.2 Run the targeted Go evaluation tests and the repository Go test target; verify the benchmark package, scorer, trace redaction, report statistics, and existing trend AI behavior all pass.

## 7. Global ranking regression

- [x] 7.1 Add a public `TrendRunner.Run` regression test with more than 100 technology values and implement server-side Top-N ranking so a high-value candidate beyond the former bound is selected; verify partial ranking failures preserve successful curves and do not exceed the 20-call tool budget.
- [x] 7.2 Mark unsupported cross-dimension project/technology queries as partial global results with an explicit warning, and verify the benchmark case expects that fallback rather than an exact project-filtered answer.
- [x] 7.3 Add regression coverage for unsupported percentage/chart requests and conflicting time ranges; preserve supported curves with warnings for the former and reject the latter without selecting an arbitrary period.
- [x] 7.4 Extend the reviewed case contract and fixture provider for independent multi-subquery cases, and verify M03 produces both project and technology curve groups with the technology category constraint.
- [x] 7.5 Add explicit benchmark fault injection for U05 so an invalid tool parameter is deterministic and isolated, then verify the model receives the validation event and continues after correction.
- [x] 7.6 Add a required formal-value constraint for category-scope cases, replace D05 with the database/MySQL subset case, and keep the offline fixture's mandatory trend-summary handshake aligned with the production orchestrator.
