# Proposal: Daily Job Synchronization

## Why

mRoleScope can validate a public source and retain a collector task directory, but it cannot yet run a durable daily collection, preserve the source evidence outside the CVM, or turn observations into PostgreSQL version history. The first production data collection must be reliable and auditable before later work can expose analytics or historical views.

## What Changes

- Add a one-shot synchronization worker that runs the existing miHoYo collector, validates its manifest and files, archives each complete raw batch to private Tencent COS, and commits an immutable PostgreSQL record of the successful batch.
- Add PostgreSQL history for stable miHoYo job identities, content-version SCD2 periods, appearance/disappearance periods, and append-only per-run audit events. A job ID remains the identity even when it later reappears with a different title or JD.
- Deduplicate unchanged JD content by a normalized business-content SHA-256 while retaining one version-level `raw_fields` payload in PostgreSQL; retain every complete collector response batch only in COS.
- Revalidate jobs missing from a complete list through the source detail API before closing a visible period. Only a recognizable source-level unavailable response may close it; network, rate-limit, and unknown failures do not.
- Add root-owned CVM deployment assets for an explicit initialization run and a non-persistent `systemd` daily 02:00 Asia/Shanghai timer. The scheduled job runs as an internal, one-shot Docker service with bounded whole-run retries and no catch-up after a missed schedule.
- Add tests and operator documentation for idempotency, archive-before-publication behavior, audit records, timer behavior, and the manual initialization/verification sequence.

## Capabilities

### New Capabilities

- `job-synchronization`: Collects the fixed miHoYo technical social-recruitment scope, archives complete source evidence, persists SCD2 job history and audit evidence, and runs safely on the CVM schedule.

### Modified Capabilities

- None.

## Impact

- **Collector and worker:** `collector/mihoyo_collector.py`, `internal/collector/`, and new Go synchronization/archive packages or commands.
- **Data:** new PostgreSQL migration(s), sqlc queries/models, and tests; existing `collection_runs`, `jobs`, `jd_versions`, and `run_jobs` gain operational use.
- **Runtime:** `deploy/remote-development/compose.yaml`, root-owned deployment entry points, runtime environment validation, and new systemd service/timer assets.
- **Dependencies:** an S3-compatible Tencent COS client/library and runtime configuration for a private bucket and least-privilege credentials.
- **Out of scope:** visitor APIs, pages, charts, period aggregates, custom-date querying, year-over-year/month-over-month calculations, WeChat alerts, cloud purchasing, and any automatic catch-up for missed schedules.
