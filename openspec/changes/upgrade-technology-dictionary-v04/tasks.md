# Tasks

## 1. Freeze v0.4 dictionary policy and persistence model

- [x] 1.1 Add the immutable v0.4 source document and structured seed, including Unreal base/version rules, Unity aliases, React/ReAct case policies, all mandatory first-batch identities (`UObject`, `World Partition`/`WorldPartition`, `StateTree`, `MassEntity`, `GameThread`, `BuildGraph`, `Wwise`, `FMOD`/`FMOD Studio`, `CRIWARE`/`Criware`, `Maya`, `DCGM`, `Nsight Systems`, `Nsight Compute`, `NVSwitch`, `CANN`, `NUMA`, `PCIe`, `RoCE`/`ROCE`, `kubelet`, `kube-scheduler`, `controller-manager`, `KubeFed`, `controller-runtime`, `client-go`, `KEDA`, `Elasticsearch`, `Ceph`, `OpenStack`, `QEMU`, `libvirt`, `cloud-init`), and only evidence-backed exclusion patterns; verify seed validation rejects duplicate rules, unapproved enabled rules, and invalid emission references.
- [x] 1.2 Add additive PostgreSQL migration support for exclusion policies, rule-emission relations, configuration snapshot membership, policy fingerprint inputs, and frozen-version mutation guards; verify migration up/down and immutable-trigger integration tests pass.
- [x] 1.3 Extend dictionary/configuration loading and canonical fingerprint calculation to include every v0.4 matching input; verify a changed exclusion or emission relation produces a different configuration fingerprint and cannot mutate an existing candidate or published configuration.
- [x] 1.4 Document the v0.4 alias map, identity expansion table, direct-match policy, and observed exclusion provenance in `Docs/开发指导/JD建模/technical-keyword-alias-map-v0.4.md`; verify the document matches the structured seed and contains no real JD text or secrets.

## 2. Implement deterministic matching and identity expansion

- [x] 2.1 Implement per-rule case and boundary behavior so `React`/`react` map to frontend React, exact `ReAct` maps to the agent identity, and unregistered case variants do not silently match; verify synthetic matcher tests cover positive, negative, and mixed-case cases.
- [x] 2.2 Implement direct matching for Unreal, Unity and the first-batch approved aliases without requiring positive context, while preserving longest registered source-span selection; verify standalone `Unity`, `UE`, `UE5`, `Unity3D`, and high-confidence aliases match and longer aliases suppress shorter overlapping source spans.
- [x] 2.3 Implement explicit exclusion evaluation within the normalized-field local window and strong-punctuation segment boundary; verify observed patterns such as `react to user input` suppress only the intended occurrence, while `熟悉 react` and `熟悉 Unity` remain matches.
- [x] 2.4 Implement source-rule emission expansion so `UE4`, `UE5`, and `UE5.4` produce base plus major-version identities while retaining one real source interval; verify `UE4` never emits UE5, `UE5.4` never creates a 5.4 display identity, and emitted evidence remains replayable.
- [x] 2.5 Implement the mandatory first-batch identities and alias groups as executable rules, preserving all required separations (`RoCE` vs `RoCEv2`, `QEMU` vs `KVM`, `client-go` vs `Go`/`Kubernetes`, `NVSwitch` vs `NVIDIA`/`NVLink`, and complete Nsight names vs bare `Nsight`); verify synthetic matcher tests cover every listed identity and alias.
- [x] 2.6 Update matcher, coordinate, and evidence-validation documentation/comments to describe source spans versus emitted identities; verify `go test ./internal/jdanalysis/...` passes with no source text emitted in diagnostics.

## 3. Persist and process v0.4 analysis results

- [x] 3.1 Update the processor and repository persistence path to store one mention per emitted identity and validate each evidence row through the frozen source-rule emission relation; verify synthetic integration fixtures persist both Unreal identities against the same `UE5` coordinates.
- [x] 3.2 Preserve idempotent full-history queueing for every body-content version and ensure candidate processing uses only the frozen v0.4 configuration; verify repeated preparation creates no duplicate work and source-metadata-only changes do not enqueue body analysis.
- [x] 3.3 Preserve safe-failure behavior for individual body analysis failures without rewriting source history or importing v0.3 results; verify failure audits contain safe counts/identifiers and no body text, snippets, credentials, or connection strings.
- [x] 3.4 Add integration coverage for full historical v0.4 processing, evidence-coordinate replay, pending/missing result counts, and configuration isolation; verify the candidate cannot pass the release gate while any required result is incomplete or invalid.

## 4. Generalize and secure the successor release path

- [x] 4.1 Refactor the common successor release checks from the v0.3-specific path into a reusable internal flow while retaining a fixed v0.4 operational adapter; verify arbitrary dictionary labels, SQL, database URLs, images, and source bodies are rejected.
- [x] 4.2 Implement the v0.4 release gate requiring the expected published v0.3 predecessor, frozen policy fingerprint, approved rule/emission set, complete historical results, and valid evidence; verify failed gates leave v0.3 active and successful release atomically publishes v0.4 and marks v0.3 superseded.
- [x] 4.3 Add idempotent release and immutable attestation coverage; verify repeated successful release returns a reused outcome without duplicate release facts or pointer regression.
- [x] 4.4 Update the controlled operations/runbook documentation for v0.4 seed, approval, preparation, analysis, publication, retryable Serving reconciliation, and rollback; verify every documented operation is fixed-scope and contains no credential values.

## 5. Restrict Serving to the active v0.4 publication

- [x] 5.1 Update technology result, filter-option, job-detail, and trend query paths to join the active publication pointer and exclude superseded/unpublished configurations; verify fixtures containing both v0.3 and v0.4 results return only v0.4 after publication.
- [x] 5.2 Ensure missing v0.4 results do not silently fall back to v0.3 after the pointer switch, while preserving the product-defined no-result behavior; verify public contract tests cover missing, unpublished, and superseded result cases.
- [x] 5.3 Preserve post-commit Serving reconciliation as retryable and pointer-driven; verify a projection failure does not roll back the committed v0.4 release and a repeated fixed operation retries reconciliation safely.

## 6. Validate the complete change

- [x] 6.1 Add end-to-end synthetic coverage spanning v0.4 seed, approval, configuration preparation, full analysis, evidence replay, release, Serving projection, and historical audit separation; verify all scenarios in both v0.4 capability specs are represented.
- [x] 6.2 Run focused Go tests, migration/integration tests with `TEST_DATABASE_URL` when available, and frontend/public-contract tests affected by technology options; record any skipped real-database checks explicitly.
- [x] 6.3 Run `openspec validate --change upgrade-technology-dictionary-v04 --strict`, repository formatting checks, and `git diff --check`; resolve only issues caused by this change and report unrelated pre-existing failures separately.

### Validation note

- Focused and full Go tests, disposable PostgreSQL migration/up-down and v0.4 end-to-end integration tests, public-contract tests, formatting checks, `git diff --check`, OpenSpec strict validation, and deployment static checks pass on 2026-11-17.
- The release-quality script owns a temporary PostgreSQL 16.4 container and supplies `TEST_DATABASE_URL` only to the integration-test process; it does not read or modify project development data.

## Workflow follow-up

- Publish v0.4 to the CVM only after the reviewed implementation is merged to protected `main`, the local release gate is clean, and a separate deployment authorization is provided.
- After production publication, verify the active pointer, Serving projection, and public technology filters using safe counts and identifiers only; do not expose or copy JD正文。
- Archive this OpenSpec change only after implementation verification and release documentation are complete.
