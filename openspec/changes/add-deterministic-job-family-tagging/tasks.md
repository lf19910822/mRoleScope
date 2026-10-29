# Tasks

## 1. Deterministic title-family contract

- [x] 1.1 After confirming the `ParseJobFamilyTitle` seam, write a red
  table-driven parser test covering the 13 reviewed family keys, representative
  approved aliases, literal Chinese and bounded ASCII behavior, project-mask
  ranges, cross-family overlap, same-family longest-match de-duplication,
  count/last-position/key primary resolution, and `unclassified`; verify the
  focused test fails before implementation.
- [x] 1.2 Add the immutable `job_family_dictionary_v1.json`, dictionary
  validation and identity/digest accessors, then implement only the matcher
  needed for task 1.1; verify the focused parser test passes without any body,
  source-project, or AI input.
- [x] 1.3 Add red/green parser regressions rejecting duplicate cross-family
  aliases, generic unlisted terms, identifier-substring false positives, and
  project-mask overreach; verify exact literal matching remains deterministic.

## 2. Immutable observations, results, and formal reads

- [x] 2.1 Add additive migrations for job-family title identities,
  source-metadata observations, result/match/evidence records, and safe retry
  audit events with constraints, indexes, and immutable-history guards; verify
  they apply in the migration-backed integration database.
- [x] 2.2 After confirming the formal read/publication seam, write a failing
  PostgreSQL integration test for in-scope classified and unclassified titles,
  `invalid_title`, `out_of_scope`, `scope_unknown`, immutable provenance and
  evidence, metadata-only title reuse, and no fallback to old titles, bodies,
  project metadata, technology, or AI; verify it fails before persistence
  implementation.
- [x] 2.3 Implement transactional publication and `ReadFormalJobFamily` using
  only the current immutable identities and existing title history; verify task
  2.2 passes without modifying any existing deterministic or AI result table.

## 3. Maximum-effort incremental processing

- [x] 3.1 After confirming the `ProcessPendingJobFamilies` seam, write a
  failing integration test for first historical catch-up, repeat idempotency,
  metadata-only reuse, title and scope changes, family/project-mask identity
  catch-up, isolated source-observation and title safe-failure audit, and
  independent retry; verify it fails before processor implementation.
- [x] 3.2 Implement per-source observation and per-title independent pending
  processing with aggregate safe summaries and safe retry-event persistence;
  verify task 3.1 passes and a failed item does not receive an old or
  unclassified result.
- [x] 3.3 After confirming the `worker analyze` seam, write a failing worker
  operation test proving job-family processing runs without provider
  configuration and attempts alongside the other deterministic stages even if
  one job-family item fails; verify it fails before wiring.
- [x] 3.4 Wire the provider-free processor into the fixed analysis operation
  with aggregate-only observability; verify task 3.3 passes without any Qwen
  configuration or outbound call.

## 4. Documentation and integrated verification

- [x] 4.1 Update JD modelling goals and operating guidance with the approved
  scope, title-only inputs, 13-family dictionary, masking, matching/primary
  rules, statuses, history, retry behavior, historical catch-up, daily
  incrementality, and no-AI guarantee; verify documented command names match
  the shipped worker operation.
- [x] 4.2 Run focused parser, repository, processor, and worker tests; run
  migration-backed CI-equivalent checks, `openspec validate
  add-deterministic-job-family-tagging --strict`, and `git diff --check`; record
  results before review.

## Workflow follow-up

- Review the change before deployment.
- Deploy only through the existing Docker Hub and GitHub Actions workflow.
- Invoke the fixed analysis operation once for the historical provider-free
  baseline and once more to verify ordinary incremental idempotence; do not run
  a Qwen operation.
