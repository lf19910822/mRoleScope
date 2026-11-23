# Spec Delta

## Purpose

为趋势智能体提供可重复、可审阅、可追踪的评测基准，使模型、提示词、工具编排和草稿校验的改动能够用统一结果比较。

## ADDED Requirements

### Requirement: Benchmark cases have a reviewable source

The system SHALL maintain 30 human-authored Chinese trend questions, divided into six categories with five questions each, and SHALL derive executable evaluation data only after the Markdown cases and their expected constraints are reviewable. Each case SHALL state its ranking metric when ranking is part of the request.

#### Scenario: Complete first benchmark

- **WHEN** the first benchmark is prepared
- **THEN** it contains five cases each for explicit values, category or subcategory scope, global scope and ranking, multiple conditions, unsupported or recovery behavior, and draft validation or explanation

#### Scenario: Case has explicit expectations

- **WHEN** a benchmark case is reviewed
- **THEN** it includes its intended dimensions, formal values or scope, ranking semantics, expected curve capacity behavior, and end-to-end pass conditions

### Requirement: Evaluation runs use an isolated fixed data source

The system SHALL run the main benchmark against a versioned, reproducible Serving snapshot that is reserved for evaluation and SHALL prevent benchmark execution from changing production data, daily ingestion tasks, or real user dashboards.

#### Scenario: Repeatable snapshot run

- **WHEN** the same case is run under the same evaluation version
- **THEN** the evaluator reads the same snapshot and receives comparable candidate and trend data

#### Scenario: Live smoke test is separate

- **WHEN** a real CVM Serving smoke test is run
- **THEN** its result is labeled separately and excluded from the fixed benchmark score

#### Scenario: Production state is protected

- **WHEN** a benchmark run completes or fails
- **THEN** its reads and isolated application checks do not modify real user curves, Serving data, or daily pull tasks

### Requirement: Runs follow the product AI contract

The system SHALL reuse the product prompt, model adapter, tool schema, orchestration, validation, and token accounting, while isolating every case and repetition in a fresh request with a maximum of 20 tool calls.

#### Scenario: Three independent repetitions

- **WHEN** a case is evaluated
- **THEN** it is run three times serially with no inherited conversation, draft, tool result, or application state

#### Scenario: Tool budget is exceeded

- **WHEN** a run attempts more than 20 tool calls
- **THEN** that run is marked failed and the evaluator records that the limit was reached

#### Scenario: Infrastructure failure

- **WHEN** a tool or infrastructure failure prevents a complete judgment
- **THEN** the run is classified separately from an AI semantic failure and is retried at most twice before being marked incomplete

### Requirement: Structured outputs are scored by semantic constraints

The system SHALL use deterministic checks for schema validity, formal values, query scope, ranking metric and order, candidate capacity, and draft validity, allowing semantically equivalent tool paths while enforcing user restrictions and required dependencies. Display names SHALL be presentation data only; comparison SHALL use normalized formal IDs.

#### Scenario: Explicit values are constrained

- **WHEN** a case asks to compare named values
- **THEN** only those normalized formal values are accepted, and an unrelated wildcard or extra curve fails the scope check

#### Scenario: Ranking order is meaningful

- **WHEN** a case asks for the top value, top N, or an ordered ranking
- **THEN** the evaluator checks result order; ordinary comparisons and category candidate sets are compared without requiring order

#### Scenario: Candidate capacity is reached

- **WHEN** valid candidates exceed eight curves
- **THEN** the evaluator accepts the product's fixed ordered first eight, including when values are zero, and checks the reported total and omitted count

#### Scenario: Partial or unsupported conditions occur

- **WHEN** a case contains an unknown value, unsupported condition, or a backend failure affecting only some candidates
- **THEN** the evaluator requires the system to preserve valid results, explain the limitation, and avoid silently widening the request to `*`

#### Scenario: Category scope retains a named value

- **WHEN** a case asks for all values in a category while calling out one value as especially important
- **THEN** the evaluator allows other values from that category but requires the named formal value to appear in the final draft

#### Scenario: Invalid AI parameters are corrected

- **WHEN** the model sends an invalid tool parameter
- **THEN** the run records the validation error, sends it back to the model for correction, and evaluates the corrected continuation

#### Scenario: Backend failure is not retried as an AI error

- **WHEN** a syntactically valid tool request fails in the backend
- **THEN** the run records the tool failure, continues with other valid candidates when possible, and does not request an AI correction solely for that backend failure

### Requirement: Evaluation records bounded audit evidence

The system SHALL save a structured trace for each judged run containing the case, evaluation versions, model output, tool names and parameters, tool result summaries, validation outcome, draft outcome, timing, token counts, and estimated cost, without saving chain-of-thought, credentials, or unnecessary JD text. Evaluation traces SHALL remain evaluation artifacts and SHALL NOT be shown to ordinary users.

#### Scenario: Successful trace

- **WHEN** a run produces a complete draft
- **THEN** its trace records the structured result, tool activity, draft checks, first-event latency, total duration, token usage, and cost

#### Scenario: Failed trace

- **WHEN** a run fails or is incomplete
- **THEN** its trace records the safe failure classification and available tool and validation history without exposing sensitive payloads

#### Scenario: Trace is not a user-facing response

- **WHEN** a benchmark trace is written
- **THEN** it is available to evaluators and developers through evaluation artifacts only and is absent from the ordinary product response

### Requirement: Reports separate quality, reliability, and performance

The system SHALL report each case's three-run pass count, tool-call, query-scope, and draft sub-scores, six category pass rates, overall equal-weight pass rate, infrastructure incompleteness, tool-call counts, token usage, cost, and average, P50, and P95 latency metrics. Cost and latency SHALL remain independent performance metrics and SHALL NOT alter the functional pass result.

#### Scenario: Report summarizes repetitions

- **WHEN** the benchmark report is generated
- **THEN** each case shows a pass count from 0/3 through 3/3, category summaries are separate, and incomplete infrastructure runs are not counted as AI failures

#### Scenario: Version metadata is present

- **WHEN** a report is generated
- **THEN** it identifies the model and provider, prompt version, tool schema version, Serving snapshot version, code revision, and evaluation run ID

### Requirement: Explanation checks are advisory

The system SHALL perform lightweight checks that the explanation is Chinese, within the product length limit, consistent with selected curves, and clear about material limitations, without requiring fixed wording or making explanation quality a hard end-to-end gate.

#### Scenario: Explanation wording varies

- **WHEN** two explanations use different Chinese wording but describe the selected curves and limitations accurately
- **THEN** both remain eligible for the same functional result and the explanation check records only its advisory outcome

### Requirement: Draft evaluation is separate from application integration

The system SHALL treat generation and application as separate evaluation boundaries: the main AI benchmark ends after a valid draft passes application-preflight validation, while applying curves and rendering the dashboard are covered by independent integration tests.

#### Scenario: Draft reaches the benchmark endpoint

- **WHEN** a run returns a draft that passes validation
- **THEN** the AI benchmark can pass without mutating the dashboard

#### Scenario: Application integration is tested

- **WHEN** application behavior is evaluated
- **THEN** it uses isolated test state and reports application or rendering failures separately from AI benchmark scores
