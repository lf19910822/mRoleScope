# Proposal

## Why

The source exposes only the official top-level `程序&技术类` category and no
reliable structured frontend, backend, client, or similar subdivision. The
product needs an explainable, reproducible job-family dimension derived from
the stored canonical title, without making JD bodies, source project fields,
or an AI model a hidden classification input.

## What Changes

- Add a deterministic, versioned, title-only job-family parser with the
  approved 13-family dictionary and one explicit `primary_family_key` per
  in-scope title at most.
- Preserve complete per-family match counts and match evidence while resolving
  the primary family by count, then last match end position, then stable
  `family_key` order.
- Apply the existing reviewed project dictionary as an exact-title-fragment
  mask before job-family matching; project, source metadata, JD text,
  technology records, and AI output must never otherwise classify a job.
- Persist immutable title-level parser results and source-metadata observations
  so history is replayable, title-only changes are incremental, and a parser,
  family dictionary, or project-mask dictionary revision catches up the known
  title corpus.
- Distinguish in-scope classified and unclassified titles from `invalid_title`,
  `out_of_scope`, and `scope_unknown` observations. Process each eligible job
  independently, retaining safe audit/retry evidence for unexpected failures
  without blocking other deterministic stages or synchronization.
- Run the provider-free historical baseline and daily increment through the
  existing fixed `worker analyze` operation. No chart aggregation, AI request,
  model configuration, prompt, or source-sync behavior changes in this scope.

## Capabilities

### New Capabilities

- `deterministic-job-family-tagging`: Derive, persist, audit, and incrementally
  process an explainable primary job family and per-family title matches for
  in-scope technical-recruitment JD titles without AI.

### Modified Capabilities

- None.

## Impact

- Additive PostgreSQL migrations, embedded family-dictionary asset, title
  matcher, repository read/write APIs, and an independent worker stage in
  `internal/jdanalysis`.
- The existing deterministic project-tag parser is used only to provide
  reviewed project-title mask coordinates; its project assignments are not
  reinterpreted as job families.
- JD-modelling guidance is updated with the executable, provider-free
  job-family lifecycle. Existing source metadata, project tags, education,
  experience, technology, AI records, and future chart work remain separate.
