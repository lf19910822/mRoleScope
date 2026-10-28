# Tasks

## 1. Deterministic title-matching contract

- [x] 1.1 Write red table-driven tests at the `ParseProjectTitle` seam for the fifteen reviewed entries, game/direction classification, case-insensitive English matching, ASCII identifier boundaries, Unicode code-point evidence, simultaneous matches, and no-match results; verify the focused test fails before matcher implementation.
- [x] 1.2 Implement the immutable project dictionary asset, identity/digest accessors, and the minimal title matcher needed for task 1.1; verify the focused parser tests pass.
- [x] 1.3 Add red/green regression tests for title-only scope, unregistered suffixes, overlapping aliases, and explicit boundary policies; verify no test case infers a game from body text, source project metadata, or a technology term.

## 2. Durable title-level results

- [x] 2.1 Add additive migrations for title identities, metadata-version observations, versioned project-tag result headers, assignments, and match evidence with constraints/indexes/immutability guards; verify migrations apply cleanly to the integration-test database.
- [x] 2.2 Write a failing PostgreSQL integration test at the result publication/read seam for a completed matched title, completed no-match title, assignment/evidence provenance, immutable identity, and reuse across two metadata versions that share a title; verify it fails before persistence exists.
- [x] 2.3 Implement title-identity persistence, transactional project-tag publication, and deterministic formal reads; verify task 2.2 passes without modifying AI, body-analysis, or technology-result tables.

## 3. Incremental fixed processing

- [x] 3.1 Write a failing integration test at the `ProcessPendingProjectTags` seam proving first-run title catch-up, repeat-run idempotency, reuse after metadata-only change, a distinct result after title change, and reprocessing on a new dictionary identity; verify it fails before processor implementation.
- [x] 3.2 Implement the independent project-tag processor and durable unseen-title selection; verify task 3.1 passes with no model credential, provider client, or body-content input.
- [x] 3.3 Write a failing worker-operation test proving normal fixed `worker analyze` invokes project-tag processing independently of education, published technology analysis, and AI configuration; verify it fails before wiring.
- [x] 3.4 Wire project-tag processing and aggregate-only observability into the standard analysis operation; verify task 3.3 passes and a project-tag failure remains downstream of successful source synchronization.

## 4. Formal boundary, documentation, and verification

- [x] 4.1 Write and pass a repository/API-facing regression test proving formal project reads use only completed deterministic title results and never fall back to source `projectId`/`projectName`, JD-body mentions, technology results, or AI projections.
- [x] 4.2 Update `JD解析目标.md` and add project-tag operation guidance documenting title-only matching, named-game versus source-direction grouping, one-time catch-up, daily incrementality, unrecognized titles, dictionary maintenance, and retained source-field/body-mention boundaries; verify documented operation names match the shipped worker operation.
- [x] 4.3 Run focused parser, PostgreSQL integration, processor, worker, and repository test suites; run migration-backed CI-equivalent checks, `openspec validate add-deterministic-project-tagging --strict`, and `git diff --check`; record results before deployment.

## Workflow follow-up

- Deploy through the existing Docker Hub and GitHub Actions workflow only after review.
- On CVM, invoke the fixed standard analysis operation once for the deterministic baseline and validate aggregate-only idempotency on a second run; do not invoke an AI trial or publication operation.
