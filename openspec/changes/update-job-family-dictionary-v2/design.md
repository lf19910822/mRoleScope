# Design

## Scope

V2 is a data-only revision of deterministic job-family matching. The parser
implementation, 13 `family_key` values, title-only input boundary, exact
project masking, match selection, and primary-family tie-break rules do not
change. The active embedded asset moves from V1 to a new V2 file so the result
identity changes through the existing dictionary version and SHA-256 fields.

## Approved aliases

| Family | Added aliases |
| --- | --- |
| `algorithm_research` | `Posttrain`, `搜推`, `搜推AI` |
| `backend` | `数据库`, `Agent开发`, `Agent研发` |
| `data_engineering` | `数据处理` |
| `operations_sre` | `网络工程师`, `k8s`, `Kubernetes` |
| `platform_infra` | `AI`, `agent`, `大模型`, `推理优化`, `云原生`, `MaaS` |

The ASCII aliases above are case-insensitive and identifier-bounded, except
for composite `搜推AI`, whose whole literal is case-insensitive. This allows
`AI` in `游戏AI研发` while preventing a platform match inside `AIOps`.
The approved `Agent开发`/`Agent研发` aliases still add a backend match; because
they end after the bounded `agent` match, they remain primary for those titles.
Likewise `K8S云原生开发` retains both the operations `k8s` and platform
`云原生` matches, with platform primary because its effective match ends later.
`搜推AI` is deliberately more specific than `搜推`: it preserves the reviewed
`algorithm_research` primary family for `资深搜推AI工程师` even though the title
also retains the general platform `AI` match.

`云游戏技术专家` remains deliberately unclassified: it describes an industry
domain and contains none of the approved functional aliases.

## Compatibility and rollout

`job_family_dictionary_v1.json` remains in source control and existing V1
database records remain immutable. The parser embeds V2 as its current asset;
the established pending processor detects the new version/digest and can
produce V2 results independently for historical titles after a separately
authorized provider-free run. This change itself performs no database write or
deployment.
