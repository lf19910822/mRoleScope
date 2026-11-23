# Tasks

## 1. Partial batch delivery

- [x] 1.1 Add red/green tests for mixed and all-failed batch summary payloads.
- [x] 1.2 Preserve successful summaries, mark the orchestration result partial, and retain per-item safe errors.

## 2. Evaluation accounting

- [x] 2.1 Add a report regression test for infrastructure resource usage and functional score separation.
- [x] 2.2 Aggregate usage and duration from executed traces while excluding missing zero timings from latency percentiles.
- [x] 2.3 Add a delayed-provider regression test and measure the first model-visible event after the initial local operation.

## 3. Verification

- [x] 3.1 Run the full Go test suite, vet, diff check, and strict OpenSpec validation.
- [ ] 3.2 Deploy the fix and rerun a bounded real-model smoke before deciding whether a full paid Benchmark rerun is necessary.
