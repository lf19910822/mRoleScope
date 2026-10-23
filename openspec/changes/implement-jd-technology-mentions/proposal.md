# Proposal: Versioned JD Technology Mentions

## Why

The daily synchronization now preserves source evidence and job history, but it cannot distinguish source metadata changes from changes to the three JD body sections, nor can it produce a verifiable technical-keyword result. The first analysis slice must turn versioned JD body text into auditable “technology mentions” without exposing real JD text outside CVM/COS or treating changing dictionary coverage as a market trend.

## What Changes

- Split the current merged JD-version meaning into independently versioned source metadata and normalized JD body content, while preserving stable job identity, presence history, COS evidence, and every successful run observation.
- Add a versioned, remote-only technology-mention analysis task. It processes only `description`, `jobRequire`, and `addition`; unchanged body content reuses results, while source-metadata-only changes do not trigger body parsing.
- Persist parser and dictionary versions, an immutable effective-approved-rule snapshot, a reusable analysis-result identity, deduplicated job-level mentions, and every separate source-field evidence location needed to verify a mention remotely.
- Define an approved-dictionary lifecycle: only human-approved, deterministic mappings can run; conditional and candidate entries remain non-statistical review inputs. A parser, dictionary, or effective-approved-rule-set change creates a full historical reanalysis and atomically publishes only a complete, verified result version.
- Add minimal remote-only review records, downstream-task audit/publication state, redaction safeguards, synthetic tests, and operational documentation for the first analysis task.
- Keep daily collection as an independent upstream producer: analysis failure cannot roll back, block, or re-run a completed source synchronization.

## Capabilities

### New Capabilities

- `jd-versioned-content`: Preserves a job observation’s source metadata version and JD body-content version independently, with deterministic normalization and history links suitable for downstream processing.
- `jd-technology-mentions`: Produces, validates, reviews, versions, and publishes remote-only technology-mention results from approved dictionary rules and versioned JD body content.

### Modified Capabilities

- None.

## Impact

- **Data:** additive PostgreSQL migration(s), backfill of the existing initial synchronized baseline, `sqlc` models/queries, and integration tests. Existing raw evidence remains in private COS and existing visibility history remains intact.
- **Worker/runtime:** a distinct, one-shot analysis entry point and root-owned operational trigger/audit path; the existing daily collection timer remains the sole source-collection scheduler and does not become dependent on successful analysis.
- **Rules and tests:** a versioned approved-dictionary representation derived from the reviewed v0.1 mapping, synthetic fixtures only, Unicode-code-point evidence validation, and redacted logging.
- **Out of scope:** visitor APIs, pages, charts, trend statistics, job-family classification, qualifications/experience extraction, generic workflow orchestration, image generation, automatic fuzzy/AI skill inference, and any local storage or log output of real JD text.
