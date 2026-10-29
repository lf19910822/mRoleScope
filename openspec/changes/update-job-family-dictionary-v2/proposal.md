# Proposal

## Why

The latest published snapshot has 17 completed `unclassified` job-family
titles. The audit report records an approved V2 dictionary decision for 16 of
them. The remaining `云游戏技术专家` is an industry/domain title without a
reviewed functional-family signal and must remain `unclassified`.

## What Changes

- Publish a new immutable embedded dictionary asset with version
  `deterministic-job-family-dictionary-v2`; retain V1 as historical source.
- Preserve all 13 stable family keys and parser logic; add only the aliases
  approved in the 2026-10-29 audit report.
- Apply case-insensitive, ASCII-identifier-bounded matching to the reviewed
  ASCII aliases `AI`, `agent`, `MaaS`, `Posttrain`, `k8s`, and `Kubernetes`.
- Add title-only parser regressions for all 17 audited titles, the approved
  cross-family effects of `Agent开发` and `K8S云原生`, and bounded `AI` behavior
  for `AIOps`.

## Non-Goals

- No new family key, AI call, JD-body input, schema migration, source-sync
  change, manual per-title override, chart change, or deployment.

## Impact

The changed dictionary version and digest make known title identities pending
for the existing provider-free catch-up path. Historical V1 results remain
immutable and auditable; a later authorized run processes the V2 baseline.
