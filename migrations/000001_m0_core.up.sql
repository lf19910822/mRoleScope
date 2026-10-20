CREATE TYPE collection_run_status AS ENUM (
    'queued', 'collecting', 'validating', 'archiving', 'parsing', 'aggregating', 'published', 'failed', 'incomplete'
);

CREATE TABLE collection_runs (
    id UUID PRIMARY KEY,
    schedule_key TEXT NOT NULL,
    source_company TEXT NOT NULL,
    filter_config JSONB NOT NULL,
    filter_sha256 CHAR(64) NOT NULL,
    status collection_run_status NOT NULL DEFAULT 'queued',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    list_reported_total INTEGER,
    list_items INTEGER,
    unique_source_job_ids INTEGER,
    detail_succeeded INTEGER,
    detail_failed INTEGER NOT NULL DEFAULT 0,
    archive_uri TEXT,
    archive_sha256 CHAR(64),
    failure_stage TEXT,
    failure_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT collection_runs_counts_nonnegative CHECK (
        COALESCE(list_reported_total, 0) >= 0 AND COALESCE(list_items, 0) >= 0
        AND COALESCE(unique_source_job_ids, 0) >= 0 AND detail_succeeded >= 0 AND detail_failed >= 0
    )
);

CREATE UNIQUE INDEX collection_runs_schedule_key_active_idx
    ON collection_runs (schedule_key)
    WHERE status NOT IN ('published', 'failed', 'incomplete');

CREATE TABLE publication_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    published_run_id UUID REFERENCES collection_runs(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO publication_state (singleton) VALUES (true);

CREATE TABLE jobs (
    id UUID PRIMARY KEY,
    source_company TEXT NOT NULL,
    source_job_id TEXT NOT NULL,
    first_seen_run_id UUID REFERENCES collection_runs(id),
    last_seen_run_id UUID REFERENCES collection_runs(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_company, source_job_id)
);

CREATE TABLE jd_versions (
    id UUID PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES jobs(id),
    content_sha256 CHAR(64) NOT NULL,
    raw_fields JSONB NOT NULL,
    display_text JSONB NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id, content_sha256)
);

CREATE TABLE run_jobs (
    run_id UUID NOT NULL REFERENCES collection_runs(id),
    job_id UUID NOT NULL REFERENCES jobs(id),
    jd_version_id UUID NOT NULL REFERENCES jd_versions(id),
    observed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (run_id, job_id)
);

CREATE TABLE parse_batches (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES collection_runs(id),
    parser_version TEXT NOT NULL,
    dictionary_version TEXT NOT NULL,
    classification_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'complete', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE (run_id, parser_version, dictionary_version, classification_version)
);

CREATE TABLE parsed_versions (
    id UUID PRIMARY KEY,
    parse_batch_id UUID NOT NULL REFERENCES parse_batches(id),
    jd_version_id UUID NOT NULL REFERENCES jd_versions(id),
    status TEXT NOT NULL CHECK (status IN ('complete', 'review', 'failed')),
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (parse_batch_id, jd_version_id)
);

CREATE TABLE job_classifications (
    id UUID PRIMARY KEY,
    parsed_version_id UUID NOT NULL REFERENCES parsed_versions(id),
    taxonomy_version TEXT NOT NULL,
    official_category TEXT,
    primary_family TEXT,
    secondary_tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence JSONB NOT NULL,
    UNIQUE (parsed_version_id, taxonomy_version)
);

CREATE TABLE experience_requirements (
    id UUID PRIMARY KEY,
    parsed_version_id UUID NOT NULL REFERENCES parsed_versions(id),
    minimum_years NUMERIC(4,1),
    maximum_years NUMERIC(4,1),
    comparison_operator TEXT,
    experience_domain TEXT,
    relation_group TEXT,
    evidence JSONB NOT NULL
);

CREATE TABLE job_skills (
    id UUID PRIMARY KEY,
    parsed_version_id UUID NOT NULL REFERENCES parsed_versions(id),
    canonical_skill TEXT NOT NULL,
    matched_text TEXT NOT NULL,
    requirement_kind TEXT NOT NULL CHECK (requirement_kind IN ('required', 'responsibility', 'bonus', 'unknown')),
    evidence JSONB NOT NULL,
    UNIQUE (parsed_version_id, canonical_skill)
);

CREATE TABLE job_locations (
    id UUID PRIMARY KEY,
    parsed_version_id UUID NOT NULL REFERENCES parsed_versions(id),
    city TEXT NOT NULL,
    evidence JSONB NOT NULL,
    UNIQUE (parsed_version_id, city)
);

CREATE TABLE education_requirements (
    id UUID PRIMARY KEY,
    parsed_version_id UUID NOT NULL REFERENCES parsed_versions(id),
    minimum_level TEXT,
    replacement_conditions TEXT,
    evidence JSONB NOT NULL,
    UNIQUE (parsed_version_id)
);

CREATE TABLE change_events (
    id UUID PRIMARY KEY,
    previous_run_id UUID NOT NULL REFERENCES collection_runs(id),
    current_run_id UUID NOT NULL REFERENCES collection_runs(id),
    job_id UUID NOT NULL REFERENCES jobs(id),
    event_kind TEXT NOT NULL CHECK (event_kind IN ('added', 'removed', 'changed', 'unchanged')),
    previous_jd_version_id UUID REFERENCES jd_versions(id),
    current_jd_version_id UUID REFERENCES jd_versions(id),
    changed_fields JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (previous_run_id, current_run_id, job_id)
);

CREATE TABLE aggregate_snapshots (
    id UUID PRIMARY KEY,
    run_id UUID NOT NULL REFERENCES collection_runs(id),
    metric_key TEXT NOT NULL,
    methodology_version TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, metric_key, methodology_version)
);

CREATE INDEX run_jobs_job_id_idx ON run_jobs (job_id);
CREATE INDEX jd_versions_job_id_idx ON jd_versions (job_id);
CREATE INDEX change_events_pair_idx ON change_events (previous_run_id, current_run_id);
