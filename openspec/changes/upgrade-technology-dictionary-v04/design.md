# Design

## Context

See [proposal.md](proposal.md) and the two capability specifications. The existing technology pipeline already stores normalized JD body versions, immutable dictionary versions, approved executable rules, analysis configurations, per-body results, evidence coordinates, and one active publication pointer. The current matcher selects the longest non-overlapping source alias and each physical rule belongs to one technology identity. The current v0.3 release path is version-specific.

The v0.4 change must preserve those historical facts while adding three semantics that the current model cannot represent safely: explicit exclusion policies, one source span emitting multiple technology identities, and a successor release that never exposes a partially analyzed dictionary.

## Goals / Non-Goals

**Goals:**

- Make v0.4 matching deterministic from a frozen dictionary/configuration snapshot.
- Preserve recall for explicit technical terms without requiring nearby positive context.
- Represent Unreal base/version expansion as auditable identity relations tied to a real source span.
- Keep React and ReAct separate under explicit case policies.
- Reuse the active-publication boundary for every Serving technology query.
- Complete and validate historical reanalysis before switching the public pointer.

**Non-Goals:**

- Do not build a general natural-language classifier or fuzzy spelling corrector.
- Do not use AI, source tags, titles, or job summaries to infer technology mentions.
- Do not modify v0.3 rows, evidence, release attestations, or historical body text.
- Do not add a public endpoint for selecting dictionary versions.
- Do not make source collection depend on analysis success.

## Decisions

### 1. Use a frozen matching-policy snapshot

The v0.4 analysis configuration will snapshot not only enabled alias rules but also exclusion policies and identity-emission relations. The stable rule-set fingerprint will be calculated from a canonical ordered representation containing:

- source rule key, canonical identity, alias, case mode, boundary mode and overlap priority;
- exclusion pattern key, pattern, case mode, boundary mode and local-window policy;
- emitted identity key and relation type for every source rule;
- the v0.4 matcher-policy version.

The database representation should use additive normalized relations rather than an opaque unversioned runtime map. The candidate configuration stores the IDs of all frozen members, and the release gate verifies the same fingerprint used during analysis.

**Alternative rejected:** keep exclusions and emissions only in Go constants. That would make a historical result depend on the running binary rather than on the released configuration and would make remote evidence replay incomplete.

### 2. Model inherited identities as explicit rule emissions

A source alias rule remains the owner of the text span, while a durable emission relation declares every identity produced by that source rule. The source rule emits itself by default; a `UE4`, `UE5`, or `UE5.4` rule additionally emits `Unreal Engine`. A `UE5` rule therefore produces two mention inputs that both reference the same real source interval and source rule provenance.

Persistence validation must accept an evidence row when the referenced dictionary rule is frozen in the configuration and its emission relation includes the mention identity. It must not require the rule's primary identity and the mention identity to be identical in the inherited case.

The matcher selects overlap candidates by source interval first. Once `UE5.4` wins over `UE5` and `UE`, all emissions of the selected source rule are retained. This prevents the current longest-match algorithm from silently dropping the base Unreal identity.

**Alternative rejected:** create duplicate physical alias rules for the base and version identities. That would make overlap selection and approval state ambiguous, and would require special handling for duplicate evidence rules without expressing the parent relation directly.

### 3. Make positive context optional and exclusions explicit

Every approved direct technical alias is eligible to match without a nearby positive technology word. Context is used only to recognize an approved negative pattern. A negative pattern is a fixed literal/boundary rule with a provenance reference to the audit decision that authorized it; it is not a general semantic classifier.

The exclusion evaluator sees only the same normalized source field, the nearest stable strong-punctuation segment, and a maximum local window of 96 Unicode code points around the alias. The current body normalizer folds whitespace, so newline boundaries are not used. Chinese and ASCII sentence/semicolon punctuation are strong boundaries; commas, enumeration punctuation and slashes are not hard boundaries.

The v0.4 seed initially includes only exclusion patterns that have been observed and approved. If no exclusion pattern matches, the direct alias match remains valid.

**Alternative rejected:** require a positive context token such as “game”, “engine”, or “frontend”. It improves precision in synthetic examples but produces unexplained false negatives for concise JD statements such as `熟悉 Unity`.

### 4. Keep case policy per alias

`React` and lowercase `react` are separate explicit aliases of the frontend identity, while `ReAct` is an exact-case alias of the agent identity. No global case folding is applied to those rules. Other spellings such as `REACT` require their own reviewed rule before they can match.

`Unreal`/`unreal` and `Unity`/`unity` retain their explicitly reviewed aliases. `UE`, `UE4`, `UE5`, and `UE5.4` use ASCII identifier boundaries so they do not match inside longer identifiers.

**Alternative rejected:** convert the complete dictionary to ASCII case-insensitive matching. That was the direct cause of the React/ReAct ambiguity and would weaken the auditability of every future mixed-case technology name.

### 5. Publish v0.4 through a closed successor operation

The implementation will factor the common successor-release checks into an internal reusable path, but the operational entry point remains fixed to v0.4 and accepts no arbitrary dictionary label, SQL, database URL, image, or source text. The operation locks the active publication state, verifies that the expected predecessor is the published v0.3 configuration, verifies the frozen v0.4 allowlist and complete results, enqueues or verifies the historical catch-up set, and commits the pointer/configuration status transition atomically.

Serving reconciliation remains post-commit and retryable, following the existing v0.3 projection boundary. A failed projection cannot roll back a committed dictionary publication; rerunning the same fixed operation retries reconciliation against the active pointer.

**Alternative rejected:** copy `ReleaseV03Upgrade` into a second large `ReleaseV04Upgrade` implementation. That would preserve version-specific duplication and make future dictionary successors more likely to diverge in their release gates.

### 6. Mandatory v0.4 dictionary scope

The following five groups are not optional candidate suggestions. They are the mandatory first-batch scope of v0.4 and must reach the approved executable rule set before v0.4 can pass its release gate. Each item receives an independent stable identity unless the table explicitly marks it as an alias.

| Group | Formal identities and aliases | Required separation |
| --- | --- | --- |
| Unreal subsystems | `UObject`; `World Partition`/`WorldPartition`; `StateTree`; `MassEntity`; `GameThread`; `BuildGraph` | Keep each subsystem independent; do not replace them with only `Unreal Engine` |
| Game tools | `Wwise`; `FMOD`/`FMOD Studio`; `CRIWARE`/`Criware`; `Maya` | FMOD spellings share one identity; CRIWARE spellings share one identity; do not merge tools with the engine |
| GPU/performance | `DCGM`; `Nsight Systems`; `Nsight Compute`; `NVSwitch`; `CANN`; `NUMA`; `PCIe`; `RoCE`/`ROCE` | Do not infer `GPU`, `NVIDIA`, `CUDA`, `NVLink`, or `RoCEv2`; bare `Nsight` is not an alias of either complete product |
| Kubernetes components | `kubelet`; `kube-scheduler`; `controller-manager`; `KubeFed`; `controller-runtime`; `client-go`; `KEDA` | Keep components separate from `Kubernetes`, `Kubebuilder`, and `Go` |
| Infrastructure | `Elasticsearch`; `Ceph`; `OpenStack`; `QEMU`; `libvirt`; `cloud-init` | Do not infer `Elastic`, `ELK`, `Kibana`, `KVM`, or Kubernetes |

The seed will also include the explicitly approved Unreal base/version expansion, Unity aliases, and React/ReAct correction. The five groups above must be represented in the v0.4 source document, structured seed, approval allowlist, executable snapshot, synthetic matcher tests, and release-gate coverage; they may not remain candidate-only records.

`Megatron`, `Triton`, `Transformer(s)`, `eBPF`, `RLAIF`, `Superluminal/Superlumina`, `PerfDog`, `Postman`, `Chromium`, CAD product names, operating systems, host platforms and vendor names remain candidate/conditional unless a separate reviewed rule is added.

### 7. Preserve the current data boundary

The analyzer continues to read only the three normalized body fields: `description`, `jobRequire`, and `addition`. It stores rule IDs, identity IDs, normalized-field hashes and Unicode code-point intervals, never source snippets or raw JD text in local artifacts, logs, or public responses. The source synchronizer remains independent; analysis work is queued after source publication and cannot roll back a successful collection.

## Risks / Trade-offs

- [Direct matching may retain a rare ordinary-language occurrence] -> Require explicit, evidence-backed exclusion patterns and add each observed false positive as a new immutable dictionary rule in a later version.
- [One source span emits multiple identities and increases evidence rows] -> Keep the body text stored once, share the same coordinate/hash, and index emissions by configuration and rule.
- [A v0.4 release can be blocked by one failed historical body] -> Preserve v0.3 as active, expose only safe failure counts, and retry the candidate without mutating source history.
- [Generic successor release logic could accidentally accept arbitrary targets] -> Keep a fixed v0.4 operational adapter and validate the expected predecessor, target version, allowlist and policy fingerprint inside the transaction.
- [Old Serving queries could accidentally select newest completed results] -> Require every Serving query and projection candidate to join the active publication pointer, with regression coverage for unpublished and superseded results.
- [A normalizer change could move evidence coordinates] -> Do not change `jd-body-v1` in this change; if normalization changes later, create another dictionary/parser successor and reanalyze all bodies.

## Migration Plan

1. Add additive schema support for v0.4 exclusion policies, rule-emission relations, configuration snapshot membership and the expanded policy fingerprint. Keep published v0.1–v0.3 rows immutable.
2. Add the v0.4 seed and fixed approval allowlist, including synthetic tests for direct matches, explicit exclusions, overlap selection, identity expansion, and React/ReAct separation.
3. Prepare a v0.4 candidate configuration and enqueue every historical body version idempotently. Run the analyzer remotely against PostgreSQL normalized bodies only.
4. Run release-gate checks for zero pending/missing work, complete results, frozen rule/emission membership, valid evidence coordinates and the expected v0.3 predecessor. Do not move the pointer if any check fails.
5. Commit the v0.4 publication pointer and v0.3 superseded transition atomically, then reconcile Serving projections as a retryable post-commit operation.
6. Verify that current filters, job details and trend statistics expose only v0.4, while remote audit queries can still distinguish v0.3 historical facts.

Rollback is pointer-preserving: before v0.4 publication, v0.3 remains active. After a confirmed material defect, use the existing controlled withdrawal/supersession mechanism rather than deleting or rewriting v0.4 facts; routine retries must not restore v0.3 silently.
