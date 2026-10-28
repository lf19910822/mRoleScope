# Tasks

## 1. Deterministic experience contract

- [x] 1.1 Write red table-driven tests at the experience-parser seam for required Arabic and Chinese numeral forms, ranges, explicit lower-bound markers, code-point evidence, and maximum-required-bound resolution; verify the focused parser tests fail before implementation.
- [x] 1.2 Implement the embedded versioned experience lexicon, number grammar, parser identity/digest accessors, and the minimal parser needed for task 1.1; verify the focused parser tests pass without a provider client.
- [x] 1.3 Write red/green parser tests and implementation for local leading/trailing priority markers, strong-boundary isolation, explicit unrestricted experience, and unrelated tenure/age/calendar exclusions; verify preferred evidence cannot create or raise `base_year`.
- [x] 1.4 Write red/green parser tests and implementation for degree-conditioned branches, ambiguous local attachment, source-field policy, and `configured`/`not_configured`/`uncertain` status rules; verify no conditional branch creates a formal `base_year`.

## 2. Immutable formal result persistence

- [x] 2.1 Add additive up/down migrations for deterministic experience result and evidence enums, constraints, indexes, and immutability triggers; verify migrations apply to the integration-test database and reject invalid status, year, identity, and mutation attempts.
- [x] 2.2 Write a failing PostgreSQL integration test for persisting a completed deterministic result with replayable evidence, lower bound, modality, and immutable identity alongside an AI experience projection; verify it fails before repository persistence exists.
- [x] 2.3 Implement deterministic experience publish/read repository APIs, including evidence replay and validation against normalized body fields; verify task 2.2 passes without reading or mutating AI tables.
- [x] 2.4 Add and pass a formal-read regression test proving the current deterministic result is used exclusively and a missing result is unavailable rather than falling back to `ai_experience_summaries`.

## 3. Incremental fixed pipeline

- [x] 3.1 Write a failing integration test for first-run historical catch-up, repeat-run idempotency, and exactly one new result after a later body-content version; verify it fails before the processor exists.
- [x] 3.2 Implement `ProcessPendingExperienceRequirements` with current parser/lexicon identity work selection and safe aggregate status summaries; verify task 3.1 passes with no model credentials or outbound provider call.
- [x] 3.3 Write a failing worker-operation test proving fixed `worker analyze` invokes deterministic experience processing even when no technology configuration is published; verify it fails before operation wiring.
- [x] 3.4 Wire deterministic experience processing into the fixed analysis operation independently of project, education, technology, and AI paths; verify task 3.3 passes and only aggregate observability is emitted.
- [x] 3.5 Update JD-modeling objectives and remote-operation guidance with formal `base_year` semantics, excluded priority/degree branches, source provenance, historical catch-up, daily incrementality, and the absence of Qwen calls; verify documented command names match the worker operation.

## 4. Verification and rollout readiness

- [x] 4.1 Run focused parser, repository, processor, and worker tests; then run `go test -p 1 ./internal/jdanalysis ./cmd/worker -count=1` plus migration-backed CI-equivalent checks, and record the results.
- [x] 4.2 Run `openspec validate add-deterministic-experience-parsing --strict` and resolve all validation findings before implementation review.

## Workflow follow-up

- Deploy through the existing Docker Hub and GitHub Actions workflow only after review.
- On CVM, run the fixed analysis operation once for deterministic historical catch-up and once again for aggregate-only idempotency verification; do not trigger a Qwen trial, expansion, or publication.
