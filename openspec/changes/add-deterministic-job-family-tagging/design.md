# Design

## Context

Existing deterministic project tagging already establishes a title-identity,
immutable-result, dictionary-digest, and incremental-worker pattern. Its
dictionary can locate reviewed complete project-name fragments in a title, but
it cannot represent a unique primary job family. Education and experience
stages are body-version based and are therefore not valid inputs for this
title-only dimension. See [proposal.md](proposal.md) for motivation and the
new capability specification for observable behavior.

The reviewed plan at
`Docs/开发指导/JD建模/job-family-rule-workflow-development-plan.md` is the
authoritative dictionary and matching-policy source for this change.

## Goals / Non-Goals

**Goals:**

- Reuse a normalized title when source metadata changes only in unrelated
  fields, while retaining an immutable observation for every source-metadata
  version.
- Keep scope eligibility separate from title parsing: the present source's
  official category ID `"1"` is in scope; a known different ID is out of
  scope; absent, null, empty, or invalid category IDs are unknown.
- Preserve a complete title-level result, including the zero-match
  `unclassified` outcome, and enough provenance to replay it from existing
  protected title history.
- Process titles independently and report safe aggregate outcomes only.

**Non-Goals:**

- Charts, chart counts, UI filtering, a job-family editing UI, manual
  per-title overrides, or a candidate/approval workflow.
- Classifying scope or job family from a JD body, technology keyword, source
  `projectName`/`objectName`, equivalent title, external knowledge, embedding,
  or any AI output.
- Fuzzy matching, regular expressions, automatic synonym expansion, Chinese
  segmentation, or semantic normalization beyond the existing normalized title
  field.
- Replacing, mutating, or reading through project-tag, education, experience,
  technology, or AI result tables.

## Decisions

### Separate job-family title identities and source observations

Add `job_family_title_identities` for each valid normalized title, keyed by
the existing source title-normalization version and SHA-256. Add immutable
`job_family_observations` for every source-metadata version. An observation
contains a nullable title identity and its scope outcome, allowing a missing
or invalid title to be a durable `invalid_title` result rather than silently
disappearing from processing.

The scope outcome is derived exclusively from
`metadata_payload.official_category.id`. String ID `"1"` is `in_scope`; a
different valid scalar ID is `out_of_scope`; an absent, null, empty, or invalid
ID is `scope_unknown`. Scope does not change title-parser identity. It only
determines whether the observation can use a title result for the current day.

Alternative considered: reusing `project_title_identities` and
`project_title_observations`. Those tables name and constrain the project-tag
domain; a new independent title dimension must not make project tagging an
implicit prerequisite or couple its schema evolution to job families.

### One immutable embedded family dictionary and exact project masking

Ship `job_family_dictionary_v1.json` with the reviewed 13 stable keys:
`algorithm_research`, `backend`, `client`, `data_engineering`,
`engineering_management`, `engine_systems`, `frontend`,
`hardware_embedded`, `operations_sre`, `platform_infra`, `security`,
`testing_quality`, and `toolchain`. Each enabled complete alias maps to one
family only. The asset validator rejects duplicate family keys, duplicate
aliases in an entry, or a complete alias assigned to more than one family.
Overlapping aliases belonging to different families remain valid because they
are different aliases, not a multi-mapped alias.

The family parser identity and family-dictionary version/digest, plus the
existing project-tag parser and dictionary version/digest used for masking,
form a result identity. It calls the project title parser only to obtain
reviewed project-fragment evidence ranges. It masks exactly those ranges and
does not inspect project assignments, raw source-project fields, or any other
input.

Chinese aliases match a contiguous literal title range. ASCII aliases may
declare case-insensitive matching and ASCII identifier boundaries. No alias is
expanded by regex, word segmentation, punctuation rewriting, or fuzzy rules.

Alternative considered: deriving aliases dynamically from technology or
project dictionaries. It would introduce a non-reviewable, cross-domain
classification path and defeat the stated version boundary.

### Deterministic per-family selection and primary resolution

The parser gathers all unmasked alias occurrences. For each family separately,
it selects non-overlapping occurrences, preferring the longest alias where
same-family candidates overlap; an equal-length tie is resolved by alias text
then range for stable evidence. Candidates from different families never
suppress one another.

The result persists selected alias evidence with Unicode-code-point ranges,
the derived count for every matched family, and either no primary family or one
primary family. The primary winner is highest count, then the family with the
last selected match end position, then lexicographically first `family_key`.
No match is a completed `unclassified` title result. Project masks can result
in no remaining family match.

Alternative considered: global longest-match selection. It would incorrectly
discard a security match in `客户端安全开发` merely because another family has a
longer overlapping alias.

### Immutable results, formal reads, and maximum-effort processing

Add immutable `job_family_results`, `job_family_matches`, and
`job_family_match_evidence` tables. A result is unique for title identity,
job-family parser/dictionary identity, and project-mask parser/dictionary
identity. The result header stores `classified` or `unclassified`, the nullable
primary family, and primary selection provenance. Match rows store each
family's count and evidence rows store alias/range evidence. Results do not
copy title text or JD text.

`ReadFormalJobFamily` first reads the source observation. It returns the
observation statuses `invalid_title`, `out_of_scope`, or `scope_unknown`
without a primary family. For an in-scope valid title it returns the current
complete title result when available, otherwise an explicit unavailable
outcome; it never falls back to a result for a prior title, project tag,
source-project field, JD body, technology result, or AI projection.

`ProcessPendingJobFamilies` creates observations one source-metadata version at
a time, then selects each in-scope valid title lacking the current result
identity. It catches up the full known title corpus after a parser or either
dictionary identity changes, and is otherwise incremental. It publishes each
title independently. An observation failure before title-identity creation
creates a separate safe audit event that references only source metadata and
the processing identity; it does not copy a title or JD field. An unexpected
per-title failure creates the existing title-identity safe audit event. Either
failure leaves only its own item pending for a later run or manual requeue; it
does not become `unclassified` and does not stop remaining titles or downstream
worker stages.

Alternative considered: a single batch transaction and a strict all-or-nothing
completion rule. It would turn a local malformed input or transient storage
failure into an avoidable block on unrelated daily data delivery.

### Fixed worker integration without chart semantics

Wire the processor into `worker analyze` alongside the current deterministic
stages. It runs without provider credentials and emits only aggregate safe
counts by completed status, skipped scope state, and retryable failure. A
stage failure remains observable but does not prevent the other analysis
stages from being attempted. This change deliberately does not decide what a
future chart counts or which version a chart displays.

## Risks / Trade-offs

- [A reviewed alias is too broad] → literal-only matching and immutable
  dictionary revisions make the resulting scope visible and reversible by a
  future full reparse; no automatic synonym discovery expands it.
- [A project fragment hides a legitimate family term] → only existing reviewed
  full project entries create masks, and mask identity is retained with every
  result.
- [A source contract stops supplying a title or category] → store an explicit
  `invalid_title` or `scope_unknown` observation rather than guessing.
- [A single observation or title repeatedly fails] → retain safe retry audit
  events while processing other titles; manual action can correct the source,
  publish a dictionary revision, or requeue only.
- [A future dictionary revision is partly caught up] → successful results are
  retained immediately; future chart-version completeness remains a separate
  product concern and is not introduced here.

## Migration Plan

1. Add additive job-family identity, observation, result, match, evidence, and
   safe retry-audit tables with constraints, indexes, and immutable-history
   guards. Do not alter existing source, project, education, experience,
   technology, or AI tables.
2. Develop the parser at the confirmed public parsing seam using red/green
   table-driven tests for the reviewed dictionary, masks, overlap behavior,
   primary tie-breaks, and no-match results.
3. Add repository publication/read and processor tests for scope outcomes,
   historical catch-up, metadata-only reuse, title changes, version catch-up,
   and isolated observation/title failure and retry behavior.
4. Wire the independent worker stage and verify it runs with no provider
   configuration and does not keep other deterministic stages from running.
5. Update operating guidance, validate the OpenSpec change, and deploy through
   the existing CI/CD flow. The first fixed `worker analyze` invocation creates
   the provider-free historical baseline; a second invocation proves ordinary
   idempotence. Rollback is a prior worker deployment; additive immutable
   results and audit rows remain available.

## TDD Seams Requiring Confirmation

Before tests are written, confirm these public seams:

1. `ParseJobFamilyTitle(normalizedTitle)` — pure title-only parser; catches
   dictionary, mask, overlap, count, and primary-family behavior, but not
   persistence or scope extraction.
2. `ReadFormalJobFamily(sourceMetadataVersionID)` and its paired publication
   API — catches scope statuses, immutable provenance, title-history reuse,
   and the absence of forbidden fallbacks, but not worker scheduling.
3. `ProcessPendingJobFamilies` — catches historical catch-up, incremental
   selection, identity-driven reprocessing, and isolated retry behavior, but
   not command wiring.
4. `worker analyze` — catches provider-free invocation and best-effort stage
   independence, but not chart behavior.
