# Design

## Context

See [proposal.md](proposal.md) for motivation and [job-synchronization spec](specs/job-synchronization/spec.md) for behavior. The Python collector already writes a validated, hash-addressed task directory, and `cmd/worker` currently stops after validating it. PostgreSQL already has run, job, JD-version, and run-observation tables, but no operational state machine, SCD2 period tables, audit event table, archive client, or scheduled runtime. The remote deployment currently builds and deploys only API and migration images through a root-owned allowlist.

The CVM is intentionally replaceable and has a 50 GiB system disk. PostgreSQL is private on the Compose backend network; the API is loopback-only. Raw source evidence must leave the CVM for private Tencent COS, and credentials cannot enter Git, images, logs, or audit rows.

## Goals / Non-Goals

**Goals:**

- Extend the existing collector-to-manifest boundary into a complete, idempotent synchronization transaction.
- Keep raw batch evidence in COS while keeping version-level records and queryable observation history in PostgreSQL.
- Model job identity, presence, and content changes independently so historical facts remain intact.
- Run a bounded, operator-auditable daily job on the CVM without exposing an additional network port.

**Non-Goals:**

- Do not add visitor-facing APIs, pages, dashboards, aggregates, comparison calculations, or WeChat alerts.
- Do not implement configurable multi-company/multi-scope administration; the current miHoYo technical social-recruitment filter is the only supported production scope.
- Do not create cloud resources, archive lifecycle deletion rules, or cloud credentials in this change.
- Do not generate field-level diff events or populate the existing `change_events` table; later presentation work can derive diffs from stored versions and periods.

## Decisions

### 1. One worker image owns collection through publication

Add a dedicated `mrolescope-worker` image and evolve `cmd/worker` into a one-shot synchronization command. Its runtime image contains the statically built Go worker, Python 3, and the existing dependency-free collector script. The worker runs the collector into a per-run temporary directory, validates the manifest again in Go, uploads verified files, and persists state.

The API remains long-running and does not run scheduled collection. The worker joins the private Compose backend network for PostgreSQL and the ordinary `edge` network solely for outbound HTTPS to the public source and COS; it has no published port, so this does not create an inbound service surface. A root-owned host wrapper creates a private temporary directory, invokes `docker compose run --rm` for the worker, and removes the directory on exit. This prevents a named Docker volume or the CVM filesystem from becoming a second long-term raw-data store.

Alternative considered: execute the Python collector directly from `systemd`. Rejected because it would split runtime dependencies, secrets, and database/archive logic across host scripts and containers. A single worker image gives initialization and scheduled runs identical behavior.

### 2. Add an explicit observation-scope and SCD2 period model

Add an `observation_scopes` table keyed by source company plus normalized filter hash/config. Each `collection_runs` row gains a scope reference, `data_date`, trigger kind (`initialization`, `scheduled`, or `retry`), attempt number, and archive-manifest digest. The current fixed scope creates one row; retaining the model prevents a future filter change from falsely closing jobs in the old scope.

Add two tables:

- `job_presence_periods`: one appearance interval per `(observation_scope, job)`, with opening, last-seen, first-missing, closure evidence, closure reason, and one partial-unique open period.
- `job_content_periods`: one SCD2 interval within a presence period, referencing `jd_versions`; a partial-unique active interval prevents two current content versions in one presence period.

`jobs` remains the stable `(source_company, source_job_id)` identity. `jd_versions` remains deduplicated by `(job_id, content_sha256)` and retains the merged per-version `raw_fields` plus normalized display/business fields. `run_jobs` remains the immutable per-complete-run observation fact.

Alternative considered: use only `jd_versions` plus `run_jobs` and derive every historical interval during reads. Rejected because reliable presence closure and current-state querying would repeatedly reimplement the same state machine. Alternative considered: put content and presence in one SCD2 table. Rejected because a title/JD change would be indistinguishable from a disappearance/reappearance.

### 3. Canonical content, raw evidence, and archive layout are separate concerns

The synchronization parser builds a canonical business object from fields that affect display or analysis (title, location, category/nature, project, description, requirements, and other defined business fields). It normalizes nulls, whitespace, line endings, HTML representation, and key order before SHA-256 hashing. Request identifiers, pagination/ordering values, timestamps, retry metadata, and other transport fields are excluded.

For each unique content hash, PostgreSQL stores one merged raw-field JSON payload and normalized fields. Every complete task directory is independently stored in COS under an immutable prefix such as `<company>/<filter-sha256>/<data-date>/<run-id>/`, including the manifest. Uploads attach the expected SHA-256 as object metadata; the uploader verifies object existence, size, and metadata before returning the archive location and manifest digest.

Alternative considered: store all daily raw responses in PostgreSQL. Rejected because its storage grows with every run and conflicts with the CVM capacity boundary. Alternative considered: store no raw fields in PostgreSQL. Rejected because later parsing a distinct JD version would require a remote archive fetch despite the data being small and deduplicated.

### 4. Persistence uses a staged state machine and one publication transaction

The worker creates a `collection_runs` row before collection so every executable attempt can be audited. It advances through collecting, validating, archiving, persisting, and published/failed stages, appending audit events at each boundary.

For a complete directory, archive upload and verification happen before the history transaction. The transaction obtains the scope lock and publication-state row, upserts jobs/JD versions, writes `run_jobs`, updates presence/content periods, writes final audit events, and atomically updates `publication_state` plus the run status. A failure before this transaction leaves prior usable state unchanged. A storage-success/database-failure case can leave an unreferenced immutable COS prefix; it is safe evidence, not a published run, and can be inspected or cleaned later by an explicit operational procedure.

The worker holds a PostgreSQL advisory lock derived from the scope for its full state-changing lifetime; the host wrapper additionally uses `flock`. The database lock is authoritative across independently started containers, while the host lock gives clear local operator feedback.

Alternative considered: archive and database changes in one distributed transaction. Rejected because COS and PostgreSQL do not share a transaction manager. Archive-first plus a single DB publication transaction preserves the required all-or-not-published outcome.

### 5. Missing-list revalidation is conservative and classifier-gated

After a complete current list and its normal details validate, the worker computes the difference from open presence periods in the same scope. It invokes a low-concurrency collector detail revalidation path for each missing ID and records outcome in the run audit trail.

- A valid matching detail response updates the observation/last-seen fact, preserves the open presence period, and records `list_omitted_detail_available` diagnostics.
- A source result classified as definitively unavailable records first-missing evidence and closes the presence period and its active content period in the same transaction.
- Timeouts, rate limits, server errors, and unrecognized source results are `indeterminate`; they never close a period.

Before enabling a definitive-unavailable classifier, add a controlled in-memory test of the source response for an invalid ID and fixture coverage for the recognized semantic. If the source contract cannot be established, the classifier remains disabled and all missing rechecks are indeterminate; this preserves correctness without changing the schema or scheduler design.

Alternative considered: close a job on its first missing list row. Rejected because a list omission or transient source fault would corrupt historical state. Alternative considered: always require another day's run. Rejected because an authoritative same-run detail response is stronger evidence and reduces false-open stale periods.

### 6. Audit data is append-only, redacted, and operationally separate from raw data

Add `collection_run_audit_events` with run reference, timestamp, stage, severity, stable code, attempt number, a length-limited message, and a small JSON diagnostics column. Index by `(run_id, occurred_at)` and a recent-failure access path. The writer sanitizes errors before insertion and log output: no passwords, tokens, DSNs, full headers, raw JSON, or JD text.

`collection_runs` remains the summary card. The audit table is the ordered flight recorder for retries, archive checks, collection/validation stages, and upstream errors. This avoids overloading `failure_summary` while keeping raw source evidence in COS only.

### 7. Scheduling is root-owned, non-persistent, and deployable by immutable image digest

Add a root-owned `mrolescope-sync` wrapper and `systemd` oneshot service/timer assets. The timer uses the Asia/Shanghai 02:00 schedule with persistent catch-up disabled. The wrapper runs at most one attempt at a time and, after a complete failure, invokes at most two whole-run retries after 15 and 45 minutes. Request-level retries remain inside the collector. A missed timer occurrence runs nothing on recovery. The one-shot worker has no published port; its `edge` attachment provides outbound source/COS HTTPS only, while PostgreSQL remains available solely through the internal backend network.

Initialization uses the same wrapper with an explicit `initialize` argument. Operators run it once, inspect the archive, run summary, and audit rows, then enable the timer. The first usable run establishes a baseline without deriving job changes.

Extend the Docker Hub build/deploy workflow and root deployment allowlist to build, verify, and accept an immutable worker image digest alongside API and migration images. Compose defines the worker as a profile/one-shot service so normal API deployment does not start collection.

Alternative considered: GitHub Actions `schedule`. Rejected because it has less predictable timing and cannot directly use the private CVM PostgreSQL network. Alternative considered: a persistent worker daemon. Rejected because scheduled one-shot processes make lock, timeout, failure, and upgrade behavior easier to audit.

### 8. Runtime configuration uses a private COS bucket and root-only secrets

Add explicit worker configuration for the COS bucket endpoint/prefix and credentials, retry delays, temporary path, and source configuration. The remote environment secret file remains root-owned mode 0600 and is read only by the root-owned compose invocation; GitHub Actions passes only image digests. The user provisions a private COS bucket in the CVM region and a least-privilege runtime identity before an initialization run.

## Risks / Trade-offs

- [The source does not expose a stable unavailable response] → Keep all failed revalidations indeterminate and do not close periods until a verified classifier exists.
- [COS succeeds but PostgreSQL fails] → The prefix is unreferenced and harmless; never publish it, retain the run failure where possible, and use an explicit later cleanup review rather than automatic deletion.
- [A job is missing from the list but detail is still available] → Preserve the period and record a safe audit event; do not manufacture a disappearance.
- [CVM disk fills from temporary files] → Use one secure per-run directory, remove it after every exit, and set a bounded worker timeout; COS holds durable raw batches.
- [Worker or host scheduling overlaps deployment] → Use root wrapper `flock`, PostgreSQL advisory locking, Compose project isolation, and deployment/sync service ordering.
- [Credentials or source data leak through diagnostics] → Use root-only runtime configuration and centralized redaction/length limits; tests assert that audit output excludes secret and body values.
- [No data exists for a missed or failed date] → Preserve the absence of a usable run rather than synthesize zero counts or a catch-up snapshot.

## Migration Plan

1. Add schema migration, sqlc/query changes, unit tests, and worker/COS code behind the one-shot command; keep the existing API behavior unchanged.
2. Build and publish the worker image through the existing immutable Docker Hub pipeline; update the root deployment allowlist and Compose definition without enabling the timer.
3. The user creates the private COS bucket and root-only runtime secrets; deploy the images and install the root-owned sync wrapper plus disabled timer.
4. Run the explicit initialization command once. Verify COS object hashes, `collection_runs`, audit events, `run_jobs`, versions, and period state.
5. Enable the timer only after initialization passes. Verify a scheduled run and bounded failure behavior in a controlled environment.

Rollback: disable the timer and stop invoking the worker; retain existing PostgreSQL rows and COS prefixes. Revert only the runtime image/deployment assets if needed. Do not delete historical runs, JD versions, periods, or archives as part of rollback.

## Open Questions

- The exact source response code/body that proves an unavailable miHoYo job must be observed and fixture-tested during implementation. Until then, the conservative classifier fallback is fully specified.
- COS lifecycle/retention class policy remains an operator cost decision; this change sets no automated expiry or deletion.
