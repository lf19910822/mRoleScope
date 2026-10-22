# Design

## Context

See [proposal.md](proposal.md) and the two capability specs for the required behavior. The existing one-shot worker publishes a complete source batch to COS before atomically writing `jobs`, a merged `jd_versions` representation, `run_jobs`, and visibility/content periods. `jd_versions` currently hashes title, locations, selected source fields, and three JD body fields together. Its reserved parse tables are unused and are keyed by collection run, so they cannot reuse a result for unchanged body text or retain more than one evidence location for a skill.

The CVM and PostgreSQL are remote-only for real JD text. The repository, CI logs, development machine, and OpenSpec artifacts must contain only synthetic text, approved dictionary rules, and non-reconstructable summaries. The existing root-owned sync helper, worker image, Compose definition, and systemd timer must continue to collect each daily batch even if all downstream work fails.

## Goals / Non-Goals

**Goals:**

- Migrate the initialized baseline to separate source-metadata and body-content identities without losing source runs, COS references, or SCD2 visibility facts.
- Make one approved dictionary/parser configuration produce one reusable result per body-content version, with many independently verifiable evidence locations.
- Give analysis its own durable queue, audit state, review records, publication pointer, and remote one-shot runner.
- Make parser/dictionary upgrades reproducible, complete before publication, and distinguishable from source changes.

**Non-Goals:**

- Do not introduce a general workflow engine, a visitor API, a review UI, analytics, chart rendering, AI/fuzzy extraction, or any analysis of title, `jobSummary`, or `source_tags`.
- Do not change the existing 02:00 source-collection schedule, retry policy, source request scope, COS retention policy, or visibility/recheck semantics.
- Do not copy raw source responses or body snippets to new local files, containers, logs, Git, CI, or OpenSpec artifacts.

## Decisions

### 1. Add canonical metadata/body versions beside the legacy merged version model

Add forward-only tables for `job_source_metadata_versions` and `job_body_content_versions`. The former stores only the approved metadata whitelist and a deterministic metadata business fingerprint. The latter stores the three normalized body fields, a body fingerprint, the normalization-rule version, and a SHA-256 for each normalized field. Both are scoped to the stable `jobs` identity and deduplicated by their respective fingerprint.

Add a one-to-one version-reference fact for each existing and future `(collection_run, job)` observation, and a body-content SCD2 period that references `job_body_content_versions`. The reference fact is the authoritative answer to “what did this job look like in this successful run”; the body period accelerates content-history queries. No separate metadata period is needed in this slice: adjacent run observations can derive metadata changes without duplicating a second state machine.

The existing `jd_versions`, `run_jobs.jd_version_id`, and `job_content_periods` remain during the additive migration as legacy compatibility facts. A remote-only backfill derives new references from their normalized payloads and preserves old IDs/periods. The synchronized baseline is small, but the backfill is idempotent and emits only safe row counts/check results. New synchronization code writes the new canonical model and no longer relies on the merged hash for downstream-analysis decisions.

Alternative considered: alter the existing merged `jd_versions` hash and foreign keys in place. Rejected because metadata-only historical versions would have to be destructively collapsed, making rollback and existing SCD2 references unsafe. Alternative considered: create metadata and body SCD2 periods. Rejected for now because per-run metadata references are sufficient and avoid a redundant interval state machine.

### 2. Keep normalization as a versioned input contract

Body normalization occurs before matching: fixed HTML-entity decoding/tag treatment, line-ending/whitespace treatment, and deterministic handling of the three source fields. Its implementation version and field-level normalized hashes are stored with the body-content version. Once a version exists, its normalized fields are immutable.

Metadata normalization preserves typed absence: absent, `null`, empty string, empty array, valid scalar/array, and invalid type remain distinguishable. Location and source-tag arrays use the confirmed de-duplicated set rules only when their source types are valid arrays; source order is not significant. Volatile/request/visitor fields remain in COS/raw evidence and source-field observation diagnostics, but do not affect either business fingerprint.

Alternative considered: retain only a normalizer version label. Rejected because code labels alone do not prove which exact text was matched. Alternative considered: normalize every empty form to one value. Rejected because it hides source contract changes and creates false “unchanged” versions.

### 3. Version the dictionary as structured, approved rules

Model stable technology identities separately from dictionary versions and rule entries. A dictionary version has immutable provenance and a content digest. Each rule has a canonical technology identity, raw alias, explicit case behavior, deterministic boundary/overlap rule, and one state: `enabled`, `conditional`, or `candidate`. The initial v0.1 map is an input proposal, not an automatically enabled runtime configuration; implementation must make its per-rule enablement explicit and reviewable.

Only `enabled` rules participate in matching. Conditional/candidate entries and `source_tags` may populate remote review candidates but never a technology mention. Rules remain typed so a concrete language/tool/product can be distinguished from a broad direction; the latter is not eligible in this slice. Longest valid registered match wins in an overlapping span, and a rule never adds implied ecosystem skills.

Alternative considered: ship the Markdown mapping as an unversioned parser constant. Rejected because no immutable approval state, publication identity, or audit trail would exist. Alternative considered: enable every plausible v0.1 entry and correct later. Rejected by the confirmed “宁少勿错” policy.

### 4. Separate one result per body/configuration from its many mentions and evidence rows

Introduce a technology-analysis run model with explicit trigger (`new-body`, `reanalysis`, or controlled manual retry), status, safe counts/error code, parser version, dictionary version, and a publication relation. A unique result identity is `(body_content_version_id, parser_version, dictionary_version)`, not a collection run ID.

For a complete result, store one `technology_mention` per `(analysis_result, technology_identity)` and any number of `technology_mention_evidence` records below it. Evidence contains source field, the referenced field-text hash/normalization version, matched alias/rule ID, and zero-based Unicode-code-point `[start, end)` coordinates. It stores no snippet. The matcher can use byte offsets internally, but it must convert them through a tested rune/code-point index before persistence; validation replays the coordinates against the remote normalized field and checks the alias/rule.

An immutable `technology_mention_review` records `correct`, `incorrect`, or `uncertain`, reviewer/time, error category, and the exact result/evidence/configuration reviewed. It never changes automated output and does not carry to a different body or configuration.

Alternative considered: adapt the unused `parse_batches`/`parsed_versions`/`job_skills` tables unchanged. Rejected because their collection-run key duplicates unchanged work, their skill uniqueness discards multiple evidence locations, and their required/responsibility/bonus field exceeds the first-slice semantics. The migration retires or supersedes these unused reservations with the explicit result/mention/evidence model.

### 5. Use a database-backed downstream queue and independently invoked analyzer

Successful source publication enqueues eligible new or changed body-content versions in the same database transaction that writes their version references. The enqueue is a small durable fact, not parsing itself, so no analysis fault can invalidate the published source run. A dedicated analyzer command consumes pending work using a configuration-specific database lock; it reads only PostgreSQL's remote body versions and writes only analysis tables/audit records.

Deploy the analyzer as a no-port, backend-only Compose one-shot service using the reviewed worker image (with an explicit analysis subcommand). A root-owned fixed helper starts it independently after a successful sync publication on a best-effort basis and exposes a fixed `run-pending-analysis` operation for controlled recovery. If the analyzer exits unsuccessfully, the sync helper reports source success, leaves queue work pending, and records only a safe analysis failure. The analyzer has no source/COS request path and no bind-mounted raw-data directory.

Alternative considered: parse within the source publication transaction. Rejected because parser/dictionary failure would delay or fail a valid raw-data publication. Alternative considered: add an all-purpose scheduler/queue for future jobs. Rejected because the first task needs only a narrow, testable queue and runner; future processors will reuse the input/version/audit/publication conventions without sharing this task's schema.

### 6. Publish configuration upgrades atomically and preserve old output

The active analysis publication points to one completed parser/dictionary configuration. A new parser or dictionary version creates a reanalysis candidate and queues every historical body-content version. Before promotion, the analyzer catches up all body versions that became eligible while the candidate ran; promotion is a single transaction that verifies no required result is pending/failed and advances the publication pointer. New bodies received after promotion use the new active configuration; bodies received during reanalysis are included before promotion.

Results for ordinary new bodies under the already active configuration become usable when their own complete result is committed. A configuration upgrade, by contrast, is all-or-nothing so a downstream statistic cannot mix configurations. Failed/cancelled candidates leave the prior publication pointer unchanged. Analysis audits label version coverage changes separately from source observation changes.

Alternative considered: reprocess only future bodies when rules change. Rejected because it converts a dictionary expansion into a false historical demand trend. Alternative considered: delete old result sets after promotion. Rejected because historical review and reproducibility require their retention; structured coordinates/IDs are small compared with raw archives.

### 7. Build validation and redaction into the release gate

Unit tests use synthetic Chinese/ASCII/punctuation/combining-character text to validate normalization, boundaries, overlap, byte-to-code-point conversion, interval validity, and idempotency. PostgreSQL integration tests cover baseline backfill, metadata-only change reuse, body change queueing, analysis failure isolation, stale/new work during reanalysis, and atomic publication promotion.

The first remote release stays candidate until controlled review verifies each enabled rule with at least one actual match (where one exists), high-risk boundary/multiple-hit cases, a sample of no-match bodies, and no raw text in analysis audit, container output, operations output, CI output, or error stack. Reviewer evidence is viewed only on CVM/COS. The release action records counts, identifiers, coordinates, versions, and stable error categories—not snippets.

Alternative considered: consider a zero-exit analyzer automatically published. Rejected because execution success is not quality validation and cannot detect a bad alias or coordinate contract.

## Risks / Trade-offs

- [The current merged baseline cannot express every newly tracked historic metadata value] → Backfill what is available from the remote database/current raw version, retain COS as authoritative batch evidence, and record safe backfill coverage rather than inventing history.
- [A dictionary rule remains context-dependent] → Keep it `conditional` or `candidate` until an explicit deterministic rule and synthetic boundary tests are approved.
- [UTF-8 offsets are accidentally stored as code points] → Centralize conversion/validation and require mixed-script synthetic cases plus remote coordinate replay before release.
- [Reanalysis races with a new daily body version] → Use durable queue state, a candidate configuration watermark/catch-up query, and transactional promotion only when eligible work is complete.
- [Old result versions consume storage] → Store body text once per body-content version and evidence as IDs/coordinates only; monitor result/evidence counts in safe task audits before defining a separate retention policy.
- [Analysis logs leak content] → Reuse strict structured redaction/allowlists; make tests fail if synthetic body text or secrets appear in logs/audit values.
- [Analyzer is unavailable after source success] → Leave durable work pending and use the fixed recovery operation; do not re-collect or modify the source run.

## Migration Plan

1. Add additive PostgreSQL schema and sqlc/query support for separated versions, per-observation references, body periods, dictionary/configuration records, queue/run/audit/publication records, mentions/evidence, and reviews. Supersede the unused reserved parsing tables without deleting published source facts.
2. Implement deterministic normalization/version conversion and an idempotent remote-only baseline backfill. Verify safe coverage counts, foreign keys, source-run/presence preservation, and no raw-text diagnostic output before switching new synchronization writes.
3. Implement the analyzer, deterministic rule matcher, configuration/reanalysis state machine, redaction, and synthetic/unit/integration tests. Keep all dictionary entries disabled/conditional until the structured v0.1 approval list is populated.
4. Extend the worker image, Compose profiles, root-owned deploy assets, fixed operation vocabulary, and documentation. Deploy the migration/image assets with analysis invocation disabled until the baseline and configuration seed are verified.
5. Seed a candidate dictionary version from the reviewed mapping, explicitly approve only eligible rules, run the initial remote analysis, perform the release-gate review, and atomically publish it. Then enable best-effort post-sync analysis invocation and verify a daily source publication remains successful when an induced analyzer failure is recorded.

Rollback: stop the analyzer helper/service and disable its fixed operation while leaving daily source synchronization enabled. Keep additive database records, existing source history, COS archives, and prior analysis publication pointers. If a candidate configuration fails, mark its analysis run failed and retain the last published configuration; do not delete source or analysis evidence as rollback.
