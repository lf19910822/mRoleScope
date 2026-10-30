# Proposal

## Why

The V2 title dictionary recognizes AI platform signals and broad algorithm or
backend work, but cannot make the reviewed distinction between work that
improves model capability and work that delivers model capability to a product
or business workflow. Two stable, title-only family keys make this distinction
available without introducing an AI call or changing the established matching
contract.

## What Changes

- Publish immutable dictionary V3 with two new stable family keys:
  `ai_algorithm` (AI 算法) and `ai_application` (AI 应用).
- Move the approved, complete aliases for model research/training/evaluation
  from `algorithm_research` to `ai_algorithm`, and approved AI application and
  Agent delivery aliases from `backend` to `ai_application`.
- Add the reviewed AI algorithm and AI application composite aliases while
  retaining broad AI, Agent, model-platform, backend, and general-algorithm
  aliases in their existing families.
- Keep title-only literal matching, project masking, cross-family overlap,
  one-to-one alias ownership, and primary-family selection unchanged.
- Publish `Docs/开发指导/JD建模/岗位族词典_latest.md` as the readable V3
  dictionary source of truth, retaining V1/V2 JSON assets for history and
  audit.
- Add parser regressions for approved historical titles and preserve the
  deliberate no-match boundary for `云游戏技术专家`.

## Capabilities

### New Capabilities

- `deterministic-job-family-tagging`: Classifies in-scope job titles with a
  reviewed, versioned, title-only job-family dictionary and preserves
  auditable cross-family evidence.

### Modified Capabilities

- None.

## Impact

- `internal/jdanalysis/job_family_parser.go` and its parser tests.
- A new embedded `job_family_dictionary_v3.json`; V1 and V2 remain immutable.
- The maintained latest-dictionary and workflow documentation.
- A dictionary-version/digest change makes existing provider-free pending
  processing eligible to produce V3 history results after separately
  authorized deployment; this change sends no JD to Qwen or another external
  model.
