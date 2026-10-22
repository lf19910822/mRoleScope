-- Canonical JD versions are deliberately additive.  The legacy merged
-- jd_versions/run_jobs/job_content_periods facts remain authoritative for
-- existing readers while the synchronization writer is moved in a later
-- change step.

CREATE TABLE job_source_metadata_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id),
    metadata_sha256 CHAR(64) NOT NULL CHECK (metadata_sha256 ~ '^[0-9a-f]{64}$'),
    normalization_version TEXT NOT NULL,
    metadata_payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id, metadata_sha256)
);

CREATE INDEX job_source_metadata_versions_job_created_idx
    ON job_source_metadata_versions (job_id, created_at DESC);

CREATE TABLE job_body_content_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id),
    body_sha256 CHAR(64) NOT NULL CHECK (body_sha256 ~ '^[0-9a-f]{64}$'),
    normalization_version TEXT NOT NULL,
    description_text TEXT NOT NULL,
    job_require_text TEXT NOT NULL,
    addition_text TEXT NOT NULL,
    description_sha256 CHAR(64) NOT NULL CHECK (description_sha256 ~ '^[0-9a-f]{64}$'),
    job_require_sha256 CHAR(64) NOT NULL CHECK (job_require_sha256 ~ '^[0-9a-f]{64}$'),
    addition_sha256 CHAR(64) NOT NULL CHECK (addition_sha256 ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id, body_sha256)
);

CREATE INDEX job_body_content_versions_job_created_idx
    ON job_body_content_versions (job_id, created_at DESC);

-- A run observation records the two independently-versioned facts that were
-- observed together.  It does not replace the legacy run_jobs fact.
CREATE TABLE run_job_version_references (
    run_id UUID NOT NULL REFERENCES collection_runs(id),
    job_id UUID NOT NULL REFERENCES jobs(id),
    source_metadata_version_id UUID NOT NULL REFERENCES job_source_metadata_versions(id),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, job_id)
);

CREATE INDEX run_job_version_references_body_idx
    ON run_job_version_references (body_content_version_id, observed_at DESC);
CREATE INDEX run_job_version_references_metadata_idx
    ON run_job_version_references (source_metadata_version_id, observed_at DESC);

-- This parallel SCD2 period is intentionally kept beside the legacy
-- job_content_periods relation until downstream readers have migrated.
CREATE TABLE job_body_content_periods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    presence_period_id UUID NOT NULL REFERENCES job_presence_periods(id),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    valid_from_run_id UUID NOT NULL REFERENCES collection_runs(id),
    valid_from TIMESTAMPTZ NOT NULL,
    valid_to_run_id UUID REFERENCES collection_runs(id),
    valid_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (presence_period_id, valid_from_run_id, body_content_version_id),
    CONSTRAINT job_body_content_periods_closed_together CHECK (
        (valid_to IS NULL AND valid_to_run_id IS NULL)
        OR (valid_to IS NOT NULL AND valid_to_run_id IS NOT NULL)
    ),
    CONSTRAINT job_body_content_periods_range CHECK (valid_to IS NULL OR valid_to > valid_from)
);

CREATE UNIQUE INDEX job_body_content_periods_one_active_idx
    ON job_body_content_periods (presence_period_id)
    WHERE valid_to IS NULL;
CREATE INDEX job_body_content_periods_version_idx
    ON job_body_content_periods (body_content_version_id, valid_from DESC);

COMMENT ON TABLE run_job_version_references IS
    'Canonical metadata/body version references for each successfully published job observation.';
COMMENT ON COLUMN job_body_content_periods.valid_to IS
    'Exclusive SCD2 boundary: a body-content period is [valid_from, valid_to).';
