# Design

## Batch summary semantics

`batch_trend_summary` already returns one bounded response object per requested value, including a safe error for an unavailable item. The orchestrator will inspect the payload in two ways: an all-failed payload produces the existing safe failure; a mixed payload remains available and sets the orchestration result to partial so the deterministic draft builder can preserve successful curves and mark the draft partial.

## Report aggregation

Functional case scoring remains unchanged. Resource totals and total-duration latency are aggregated independently over all traces with a measured duration, while first-event distributions ignore zero/missing measurements. This keeps actual provider spend visible without allowing infrastructure failures to inflate pass rates.

## First-event timing

The evaluation Runner will ignore only the immediately emitted `analyze_intent:running` operation when setting first-event latency. The first completed model operation, validation event, tool operation, or assistant event becomes the measured first model-visible event.

## TDD seams

- Unit tests for mixed and all-failed batch payload classification.
- End-to-end partial-draft regression using one successful and one failed summary.
- Report aggregation test separating functional denominators from resource totals.
- Delayed-provider Runner test for first-event latency.
