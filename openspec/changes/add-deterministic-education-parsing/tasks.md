# Tasks

## 1. Deterministic education contract

- [x] 1.1 Write red table-driven tests at the `ParseEducationRequirement` seam for reviewed Chinese/English degree aliases, longest-match precedence, code-point evidence, and the product mapping `研究生 -> master`; verify the focused test fails before parser implementation.
- [x] 1.2 Implement the immutable education lexicon asset, version/digest accessors, and minimal parser needed for task 1.1; verify the focused parser tests pass.
- [x] 1.3 Add red/green parser tests and implementation for required versus preferred/unrestricted modality, lower versus upper bounds, explicit alternatives, and unresolvable conflicts; verify the focused parser test package passes.
- [x] 1.4 Add regression tests excluding responsibility-only text, `博士后`, and non-candidate `研究生` phrases; verify no excluded phrase creates a formal bound.

## 2. Durable deterministic results

- [x] 2.1 Add additive migrations for versioned deterministic education results and source-evidence provenance, with constraints and indexes; verify migrations apply cleanly to the integration-test database.
- [x] 2.2 Write a failing PostgreSQL integration test at the persistence seam for one completed result, its rule/evidence provenance, immutable identity, and coexistence with an AI education projection; verify it fails before repository persistence exists.
- [x] 2.3 Implement repository persistence and formal deterministic education read APIs; verify task 2.2 passes and no AI table is modified.

## 3. Incremental fixed pipeline

- [x] 3.1 Write a failing integration test at the `ProcessPendingEducationRequirements` seam proving first-run catch-up, repeat-run idempotency, and one result for a later body-content version; verify it fails before processor implementation.
- [x] 3.2 Implement the independent deterministic education processor and durable work selection; verify task 3.1 passes without model credentials or a provider client.
- [x] 3.3 Write a failing worker-operation test proving normal fixed `worker analyze` invokes deterministic education processing independently of published technology analysis; verify it fails before wiring.
- [x] 3.4 Wire deterministic education processing into the standard analysis operation and safe aggregate observability; verify task 3.3 passes and synchronization remains best-effort for downstream analysis.

## 4. Formal read boundary and verification

- [x] 4.1 Write and pass a repository/API-facing regression test proving the formal JD education read uses deterministic data and never falls back to the AI projection.
- [x] 4.2 Update the JD-modeling and remote-operation guidance to document the deterministic source of truth, one-time catch-up, daily incrementality, and retained AI history; verify referenced commands and names match the shipped worker operation.
- [x] 4.3 Run `go test -p 1 ./internal/jdanalysis ./internal/jobsync -count=1`, the worker test suite, `openspec validate add-deterministic-education-parsing --strict`, and the migration-backed CI-equivalent checks; record the results before deployment.

## Workflow follow-up

- Deploy through the existing Docker Hub and GitHub Actions workflow only after review.
- On CVM, run the fixed standard analysis operation once for the deterministic baseline, then verify aggregate-only idempotency on a second run; do not trigger any AI trial or publication.
