# Design

## Context

See `proposal.md` for the motivation. The current parser embeds a validated V2
JSON asset. It accepts only an enumerated family set, emits each eligible
cross-family title match, masks reviewed project fragments, and selects the
primary family by count, last match end position, and then lexical key. The
V2 asset and its history must remain immutable.

## Goals / Non-Goals

**Goals:**

- Add the two approved stable keys and an immutable V3 dictionary asset.
- Preserve evidence for independent existing-family matches while assigning
  each full alias to exactly one family.
- Make V3 reviewable in the designated latest-dictionary document.
- Verify representative production-title regressions with no provider call.

**Non-Goals:**

- No title-body parsing, AI/model invocation, schema change, manual title
  override, score weighting, or primary-family priority rule.
- No change to source sync or chart aggregation.

## Decisions

### Version the data asset instead of the parser algorithm

V3 adds an embedded `job_family_dictionary_v3.json` and changes the embed
target. The parser version remains V1 because selection, masking, and literal
matching semantics do not change. The dictionary version/digest therefore
causes the existing provider-free pending processor to consider a new title
identity. Changing the parser version would incorrectly imply an algorithmic
contract change.

### Migrate complete aliases instead of duplicating them

Approved `模型研究`/training/evaluation terms move from
`algorithm_research` to `ai_algorithm`; approved AI application and Agent
delivery terms move from `backend` to `ai_application`. A complete literal
alias cannot have two owners, which validation already enforces. Broader
substrings such as `算法`, `后端`, `AI`, and `agent` remain where they are so
independent cross-family facts still appear. Duplicating a complete phrase
would produce ambiguous ownership; removing the broad aliases would discard
useful evidence.

### Use only reviewed composite signals for the new families

V3 records the approved composite aliases for LLM training/evaluation,
multimodal and recommendation algorithms, AI full-stack work, Agent delivery,
and game-AI delivery. It deliberately does not add standalone `AI`, `LLM`,
`大模型`, `Agent`, `AIGC`, `RAG`, `MCP`, `Prompt`, or `工作流` to either new
family. This holds the title-only boundary while allowing existing V2 platform
signals to continue independently.

### Treat primary results as an effect of the unchanged selection contract

Tests will assert each complete match set and the expected primary result,
including project masking. V3 does not create a precedence override for the
new families. If broader signals later repeatedly win against an approved
phrase, that is a separate weighting/priority change requiring its own
specification and historical comparison.

## Risks / Trade-offs

- [Title vocabulary continues to evolve] → preserve V1/V2/V3 assets and use
  the latest document as the reviewed input to a future versioned revision.
- [English alias boundaries miss unreviewed spelling variants] → list the
  reviewed variants explicitly and rely on regression tests; do not add fuzzy
  matching.
- [V3 changes primary-family distribution] → use the existing immutable
  result identity and provider-free history processing for comparison; retain
  V2 results.
- [Project names contain a matching word] → retain the existing exact reviewed
  project mask before title scanning.

## Migration Plan

1. Add failing parser regressions for the approved V3 titles and two new
   family constants.
2. Add V3, update the enum validation and current embed target, then make the
   regressions pass while retaining V1/V2 assets.
3. Publish the readable latest dictionary and update the workflow plan.
4. Run focused and repository tests, strict OpenSpec validation, generated-code
   hygiene, and diff checks.
5. After review and deployment, invoke only the existing provider-free pending
   analysis path to derive V3 results for eligible historical titles; verify
   aggregate status with read-only queries. Rollback is a deployment rollback
   to the prior image: V2 results remain immutable and no source data is
   deleted.
