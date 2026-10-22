-- Daily job synchronization state.  This migration is deliberately additive so
-- an existing M0 database can be upgraded without rewriting historical rows.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE collection_trigger_kind AS ENUM ('initialization', 'scheduled', 'retry');
ALTER TYPE collection_run_status ADD VALUE IF NOT EXISTS 'persisting';

CREATE TABLE observation_scopes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_company TEXT NOT NULL,
    filter_config JSONB NOT NULL,
    filter_sha256 CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_company, filter_sha256)
);

-- Preserve a one-to-one mapping for every existing M0 run before adding the
-- mandatory foreign key.  A later filter change therefore creates a new scope
-- rather than creating false job additions or removals.
INSERT INTO observation_scopes (source_company, filter_config, filter_sha256)
SELECT DISTINCT source_company, filter_config, filter_sha256
FROM collection_runs
ON CONFLICT (source_company, filter_sha256) DO NOTHING;

ALTER TABLE collection_runs
    ADD COLUMN observation_scope_id UUID REFERENCES observation_scopes(id),
    ADD COLUMN data_date DATE,
    ADD COLUMN trigger_kind collection_trigger_kind NOT NULL DEFAULT 'initialization',
    ADD COLUMN attempt_number INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN archive_manifest_sha256 CHAR(64);

UPDATE collection_runs AS run
SET observation_scope_id = scope.id,
    data_date = (COALESCE(run.started_at, run.created_at) AT TIME ZONE 'Asia/Shanghai')::date
FROM observation_scopes AS scope
WHERE scope.source_company = run.source_company
  AND scope.filter_sha256 = run.filter_sha256;

ALTER TABLE collection_runs
    ALTER COLUMN observation_scope_id SET NOT NULL,
    ALTER COLUMN data_date SET NOT NULL,
    ADD CONSTRAINT collection_runs_attempt_number_positive CHECK (attempt_number > 0),
    ADD CONSTRAINT collection_runs_archive_manifest_sha256_valid CHECK (
        archive_manifest_sha256 IS NULL OR archive_manifest_sha256 ~ '^[0-9a-f]{64}$'
    );

DROP INDEX collection_runs_schedule_key_active_idx;
CREATE UNIQUE INDEX collection_runs_scope_active_idx
    ON collection_runs (observation_scope_id)
    WHERE status NOT IN ('published', 'failed', 'incomplete');
CREATE INDEX collection_runs_scope_data_date_idx
    ON collection_runs (observation_scope_id, data_date DESC, started_at DESC);

-- The singleton M0 publication row is retained for backward compatibility.
-- Daily synchronization publishes independently for each observation scope.
CREATE TABLE observation_scope_publication_state (
    observation_scope_id UUID PRIMARY KEY REFERENCES observation_scopes(id),
    published_run_id UUID REFERENCES collection_runs(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE job_presence_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    observation_scope_id UUID NOT NULL REFERENCES observation_scopes(id),
    job_id UUID NOT NULL REFERENCES jobs(id),
    opened_run_id UUID NOT NULL REFERENCES collection_runs(id),
    opened_at TIMESTAMPTZ NOT NULL,
    last_seen_run_id UUID NOT NULL REFERENCES collection_runs(id),
    last_seen_at TIMESTAMPTZ NOT NULL,
    first_missing_run_id UUID REFERENCES collection_runs(id),
    first_missing_at TIMESTAMPTZ,
    closed_run_id UUID REFERENCES collection_runs(id),
    closed_at TIMESTAMPTZ,
    closure_reason TEXT,
    closure_evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT job_presence_periods_closed_together CHECK (
        (closed_at IS NULL AND closed_run_id IS NULL AND closure_reason IS NULL)
        OR (closed_at IS NOT NULL AND closed_run_id IS NOT NULL AND closure_reason IS NOT NULL)
    ),
    CONSTRAINT job_presence_periods_missing_together CHECK (
        (first_missing_at IS NULL AND first_missing_run_id IS NULL)
        OR (first_missing_at IS NOT NULL AND first_missing_run_id IS NOT NULL)
    ),
    CONSTRAINT job_presence_periods_order CHECK (last_seen_at >= opened_at)
);

CREATE UNIQUE INDEX job_presence_periods_one_open_idx
    ON job_presence_periods (observation_scope_id, job_id)
    WHERE closed_at IS NULL;
CREATE INDEX job_presence_periods_scope_current_idx
    ON job_presence_periods (observation_scope_id, last_seen_at DESC)
    WHERE closed_at IS NULL;
CREATE INDEX job_presence_periods_job_idx ON job_presence_periods (job_id, opened_at DESC);

CREATE TABLE job_content_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    presence_period_id UUID NOT NULL REFERENCES job_presence_periods(id),
    jd_version_id UUID NOT NULL REFERENCES jd_versions(id),
    valid_from_run_id UUID NOT NULL REFERENCES collection_runs(id),
    valid_from TIMESTAMPTZ NOT NULL,
    valid_to_run_id UUID REFERENCES collection_runs(id),
    valid_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT job_content_periods_closed_together CHECK (
        (valid_to IS NULL AND valid_to_run_id IS NULL)
        OR (valid_to IS NOT NULL AND valid_to_run_id IS NOT NULL)
    ),
    CONSTRAINT job_content_periods_range CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE UNIQUE INDEX job_content_periods_one_active_idx
    ON job_content_periods (presence_period_id)
    WHERE valid_to IS NULL;
CREATE INDEX job_content_periods_version_idx ON job_content_periods (jd_version_id, valid_from DESC);

CREATE TABLE collection_run_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES collection_runs(id),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    stage TEXT NOT NULL CHECK (stage IN (
        'collecting', 'validating', 'missing-recheck', 'archiving', 'persisting', 'published', 'failed', 'retry-scheduled', 'contention'
    )),
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'error')),
    code TEXT NOT NULL,
    attempt_number INTEGER NOT NULL CHECK (attempt_number > 0),
    message TEXT NOT NULL CHECK (char_length(message) <= 2048),
    diagnostics JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT collection_run_audit_events_diagnostics_small CHECK (octet_length(diagnostics::text) <= 8192)
);

CREATE INDEX collection_run_audit_events_run_time_idx
    ON collection_run_audit_events (run_id, occurred_at);
CREATE INDEX collection_run_audit_events_recent_failures_idx
    ON collection_run_audit_events (occurred_at DESC)
    WHERE severity IN ('warning', 'error');

COMMENT ON COLUMN collection_runs.data_date IS
    'Asia/Shanghai calendar date on which the attempt started; exact facts remain TIMESTAMPTZ.';
COMMENT ON COLUMN job_content_periods.valid_to IS
    'Exclusive SCD2 boundary: a content period is [valid_from, valid_to).';
