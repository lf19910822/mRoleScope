# Proposal

## Why

The current minimum-education value is a Qwen semantic projection.  Explicit
JD education requirements are a bounded, high-confidence vocabulary problem;
using a model for them consumes tokens and leaves the product-facing value
less deterministic and less explainable than it needs to be.

The JD modelling pipeline needs one versioned, source-evidenced education
result for every new or changed body version, while preserving the historical
AI projection and its audit trail unchanged.

## What Changes

- Add a deterministic education-requirement pipeline that recognizes approved
  Chinese and English degree terms, normalizes them to an ordered education
  level, and records source evidence and rule provenance.
- Model lower-bound and upper-bound education constraints separately.  This
  preserves the meaning of expressions such as `本科及以上` and `本科及以下`
  instead of treating both as one minimum-level value.
- Treat `研究生` as the product-defined `master` level, while giving explicit
  doctorate expressions precedence over the generic term.
- Make the deterministic result the formal JD-modeling education result for
  the product and the incremental processing pipeline.  Existing AI education
  projections remain immutable historical analysis and audit data, but are no
  longer the product-facing source of truth.
- Process only newly created or changed JD body-content versions for a given
  immutable parser and lexicon version; repeated daily runs are idempotent and
  require no Qwen call.
- Add parser, persistence, incremental-processing, and product-read tests
  before implementation.

## Capabilities

### New Capabilities

- `deterministic-education-requirements`: Derive, store, and expose one
  evidence-backed, versioned education constraint result for each eligible JD
  body-content version without using an AI provider.

### Modified Capabilities

- None.

## Impact

- New database migration and repository persistence/read APIs for
  deterministic education results and evidence.
- A new deterministic worker operation that joins the existing incremental JD
  processing lifecycle without provider credentials or model calls.
- Product-facing education reads switch from the AI semantic projection to the
  deterministic result when it is available.
- Qwen input/output and existing AI result tables are not rewritten by this
  change; their records remain available for historical audit.
