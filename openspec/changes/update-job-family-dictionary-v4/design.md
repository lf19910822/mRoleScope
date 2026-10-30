# Design

## Scope

V4 is a data-only revision of the deterministic title dictionary. The parser
remains V1: it accepts the same fifteen keys, scans only normalized titles,
masks only reviewed project ranges, and selects one primary family by match
count, last effective end position, then lexical family key. The current
embedded asset changes from V3 to new immutable V4 so the existing result
identity naturally preserves a separate V4 history row.

## Approved V4 delta

| Family | V3 alias | V4 action | Reason |
| --- | --- | --- | --- |
| `platform_infra` | `AI` (case-insensitive, bounded) | remove | AI alone does not imply platform/infrastructure work. |
| `platform_infra` | `agent` (case-insensitive, bounded) | remove | Agent alone does not imply platform/infrastructure work. |

All other V3 aliases remain byte-for-byte equivalent in V4. In particular,
`AI 基础设施`, `AI 存储`, `训练平台`, `推理平台`, `模型平台`, `大模型`, `MaaS`,
and other explicit platform signals remain `platform_infra` aliases.

Composite AI-application aliases such as `AI全栈开发`, `Agent开发`, and
`游戏AI研发` remain in `ai_application`; they do not need a fallback bare
platform match. Likewise, bare `AI` and `Agent` do not become aliases of
`ai_algorithm` or `ai_application`.

## Verification and rollout

1. Add V4 contract tests that fail while V3 remains embedded.
2. Add V4, switch the embed target, and prove that complete application and
   algorithm phrases retain their intended matches without inferred platform
   evidence.
3. Validate the full suite and the strict OpenSpec change.
4. Merge, build and deploy the normal images through GitHub Actions.
5. Invoke only `/usr/local/sbin/mrolescope-analysis pending` on CVM to derive
   V4 history, then query aggregates in a read-only transaction.

No existing row is rewritten or deleted. A deployment rollback merely makes V3
the active identity again; V4 results already written remain immutable audit
facts.
