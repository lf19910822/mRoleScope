# Design

## Context

Source synchronization already stores immutable source-metadata versions whose
JSON payload includes a normalized title, but the metadata fingerprint also
contains location, hiring type, source project/object fields, and tags.  The
existing technology and education processors intentionally use body-content
versions and therefore are not suitable inputs for title-only project tagging.
See [proposal.md](proposal.md) for the motivation and the capability spec for
observable behavior.

## Goals / Non-Goals

**Goals:**

- Add a deterministic, versioned title-matching result that is reusable when
  different source-metadata versions carry the same normalized title.
- Preserve enough result and evidence provenance to replay a match without a
  JD body, provider request, or AI configuration.
- Treat an empty assignment set as a durable, observable result.
- Run one historical catch-up and then incremental daily processing without
  affecting synchronization, education, technology, or AI release behavior.

**Non-Goals:**

- Inferring a project from `projectId`/`projectName`, technical terms, similar
  jobs, titles that are merely semantically related, or external knowledge.
- Converting a body mention into project ownership; body mentions are outside
  this change's formal result.
- Adding a project-dictionary editing UI, automatic candidate discovery, or a
  separate project-specific AI workflow.
- Changing existing body-version, source history, AI, education, or technology
  result tables.

## Decisions

### Content-addressed title identities separate title input from metadata churn

Create a `project_title_identities` table containing the normalized title text,
its SHA-256 fingerprint, and the source title-normalization version.  A
`project_title_observations` relation maps each immutable source-metadata
version to its title identity.  This allows locations or other source metadata
to change without duplicating analysis while keeping an auditable observation
path back to the source metadata version.

The processor reads only a valid string title from the existing normalized
metadata payload. Source normalization already rejects a missing or empty title
for a valid job observation. The title is not copied from any JD body field.

Alternative considered: identify results by `source_metadata_version_id`.
That would unnecessarily rerun and duplicate project results on a location or
hiring-type-only change. Using `body_content_version_id` has the opposite
problem: title changes would not necessarily create an analysis input.

### Embedded immutable project dictionary with explicit entry classifications

Ship `project_dictionary_v1.json` as a reviewed, embedded asset. Each record
has a stable `project_key`, display name, classification (`named_game` or
`source_direction`), enabled state, explicit aliases, and a matching policy.
The asset's semantic version and SHA-256 digest form the dictionary identity.
The initial asset contains exactly the fifteen reviewed entries from
`项目维度开发.md`: ten named games and five source-direction/research labels.

Chinese aliases match literal title substrings. ASCII aliases are
case-insensitive only where declared and use explicit identifier boundaries,
so `Varsapura` matches case variants but not a longer ASCII identifier. There
is no implicit punctuation, whitespace, abbreviation, transliteration, or
series-name normalization. Parser code examines all enabled rules and retains
all valid distinct entry matches; it does not let scan order choose a winner.

Alternative considered: reuse the technology dictionary lifecycle. It models
body mentions, candidate harvesting, approval snapshots, and publication
gates, all of which would introduce an incorrect AI/technology dependency.
An embedded, reviewed asset matches the small manually maintained initial
catalog and the education lexicon pattern.

### Immutable completed result header makes no-match processing idempotent

Add additive tables:

- `project_tag_results`: one completed result per title identity, parser
  version, dictionary version, and dictionary digest; includes an explicit
  completion status.
- `project_tag_assignments`: one retained stable project key and classification
  snapshot per result and matched dictionary entry.
- `project_tag_evidence`: every assignment match's keyword, title
  normalization provenance, and Unicode-code-point range.

The result header exists even with zero assignments. A unique identity and
append-only protection on parser/dictionary provenance prevent in-place
rewrites. Assignment and evidence rows use cascading cleanup only when their
own unreferenced result is removed by a migration rollback; normal production
operations never delete historical results.

Alternative considered: store only matching rows. It cannot distinguish an
unrecognized title from an unprocessed one and would make each daily run retry
the same absence indefinitely.

### Processor selects unseen title identities and runs independently

`ProcessPendingProjectTags` first upserts title identities/observation links
for available source metadata, then selects only title identities lacking the
current immutable result. It parses and publishes each result transactionally.
The first run creates a one-time baseline; later source runs normally process
only new titles. A dictionary or parser identity change intentionally selects
the full known title corpus again.

Wire the processor into the fixed `worker analyze` operation beside the
provider-free education stage. It reports aggregate processed/reused/no-match
counts only. As with existing downstream analysis, its failure is observable
but does not roll back or mark a successful source synchronization failed.

Alternative considered: queue processing from synchronization only when a
metadata row is newly created. That would miss full dictionary-version
catch-up and adds a fragile coupling to the synchronizer's write path.

### Product read returns only deterministic title assignments

Expose a repository read seam for the active parser and dictionary identity.
It returns the completed assignment set plus classification and evidence
references, or an explicit unavailable/unidentified outcome. It does not read
source `projectId`/`projectName`, JD bodies, AI output, or technology rows.

The initial release can use the embedded dictionary as the active identity;
the read must nevertheless select by its exact version and digest so future
dictionary updates retain historical results and trigger a visible reanalysis.

## Risks / Trade-offs

- [An unlisted or abbreviated title name is not tagged] → return a durable
  `unidentified` result and add the term only through a later human-reviewed
  dictionary version.
- [A generic direction label creates a misleading game grouping] → persist and
  expose the explicit `source_direction` classification separately from named
  games.
- [An entry's display name changes] → retain its stable key; a new dictionary
  identity produces a separate historical result. Merges or splits require an
  explicit later mapping decision.
- [A title contains overlapping aliases] → apply the registered boundary and
  longest-match rules in deterministic parser tests, retain different entry
  matches, and never infer an additional entry merely from a substring.
- [Operational output reveals job data] → safe worker output contains only
  aggregate counts, parser/dictionary identities, and stable error categories;
  evidence text remains in the existing protected source-metadata store.

## Migration Plan

1. Add additive tables, constraints, indexes, and database immutability
   triggers. No source, body, technology, education, or AI table changes.
2. Add the reviewed embedded v1 dictionary and deterministic matcher, with
   table-driven parser tests before implementation.
3. Implement title-identity persistence, result publication, and formal read
   seams with synthetic PostgreSQL integration tests.
4. Wire the independent processor into the standard worker operation and
   validate historical catch-up, a repeated idempotent run, a metadata-only
   update, a title update, and a dictionary-version update.
5. Update product-model and remote-operation documentation. Deploy only after
   review; the first remote standard analysis invocation creates the baseline.

Rollback is deployment of the previous worker/reader. The migration and
completed historical results remain additive evidence; daily source
synchronization continues and no AI configuration or publication pointer is
changed.
