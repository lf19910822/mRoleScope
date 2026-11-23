# AI evaluation quality accounting

## ADDED Requirements

### Requirement: Preserve usable batch summaries
The trend evaluator MUST keep successful items from a batch trend summary when other items return safe per-item errors. It MUST mark the batch as failed only when no usable summary remains, while retaining the individual safe errors for filtering and audit.

#### Scenario: Mixed summary batch
- **WHEN** a batch contains at least one usable summary and at least one item error
- **THEN** the planner receives the usable summaries, the resulting draft may be partial, and the failed item is not included as a curve

#### Scenario: Entire summary batch fails
- **WHEN** every item in a batch returns an error or the batch returns no usable summary
- **THEN** the tool result is marked with a safe backend error and no failed candidate is presented as usable

### Requirement: Account resource usage independently from functional scoring
The evaluation report MUST aggregate input tokens, output tokens, tool calls, estimated cost, and measured total duration from every trace that executed, including infrastructure failures. Functional pass rates and semantic sub-scores MUST continue to exclude infrastructure and incomplete traces.

#### Scenario: Infrastructure failure after provider usage
- **WHEN** a trace contains provider usage or elapsed time but ends as infrastructure failure
- **THEN** its usage and duration contribute to report resource totals while it does not contribute to functional pass-rate denominators

### Requirement: Measure the first model-visible event
The evaluation Runner MUST measure first-event latency from request start to the first observable event after the synthetic initial `analyze_intent:running` event. If no later event occurs, the trace MUST retain an explicit zero/missing measurement rather than treating the initial local emission as model latency.

#### Scenario: Delayed model response
- **WHEN** the provider delays before completing the first model operation
- **THEN** first-event latency is greater than or equal to the provider delay

