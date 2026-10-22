CREATE TYPE technology_analysis_configuration_status AS ENUM ('candidate', 'published', 'superseded', 'failed', 'retired');
CREATE TYPE technology_analysis_run_status AS ENUM ('running', 'complete', 'failed');
CREATE TYPE technology_analysis_work_status AS ENUM ('pending', 'running', 'complete');
CREATE TYPE technology_analysis_trigger AS ENUM ('new_body', 'reanalysis', 'manual_retry');

CREATE TABLE technology_analysis_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parser_version TEXT NOT NULL CHECK (char_length(parser_version) BETWEEN 1 AND 128),
    dictionary_version_id UUID NOT NULL REFERENCES technology_dictionary_versions(id),
    status technology_analysis_configuration_status NOT NULL DEFAULT 'candidate',
    created_by TEXT NOT NULL CHECK (char_length(created_by) BETWEEN 1 AND 256),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    safe_failure_code TEXT CHECK (safe_failure_code IS NULL OR char_length(safe_failure_code) <= 128),
    UNIQUE (parser_version, dictionary_version_id),
    CONSTRAINT technology_analysis_configurations_published_together CHECK (
        (status = 'published' AND published_at IS NOT NULL)
        OR (status <> 'published')
    )
);

CREATE TABLE technology_analysis_publication_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    configuration_id UUID REFERENCES technology_analysis_configurations(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO technology_analysis_publication_state (singleton) VALUES (true);

CREATE TABLE technology_analysis_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    configuration_id UUID NOT NULL REFERENCES technology_analysis_configurations(id),
    trigger technology_analysis_trigger NOT NULL,
    status technology_analysis_run_status NOT NULL DEFAULT 'running',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    claimed_count INTEGER NOT NULL DEFAULT 0 CHECK (claimed_count >= 0),
    completed_count INTEGER NOT NULL DEFAULT 0 CHECK (completed_count >= 0),
    failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    safe_failure_code TEXT CHECK (safe_failure_code IS NULL OR char_length(safe_failure_code) <= 128),
    CONSTRAINT technology_analysis_runs_finished_together CHECK (
        (status = 'running' AND completed_at IS NULL)
        OR (status IN ('complete', 'failed') AND completed_at IS NOT NULL)
    )
);

CREATE INDEX technology_analysis_runs_configuration_time_idx
    ON technology_analysis_runs (configuration_id, started_at DESC);

CREATE TABLE technology_analysis_work_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    configuration_id UUID NOT NULL REFERENCES technology_analysis_configurations(id),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    trigger technology_analysis_trigger NOT NULL,
    status technology_analysis_work_status NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    current_run_id UUID REFERENCES technology_analysis_runs(id),
    last_attempt_run_id UUID REFERENCES technology_analysis_runs(id),
    last_safe_error_code TEXT CHECK (last_safe_error_code IS NULL OR char_length(last_safe_error_code) <= 128),
    queued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (configuration_id, body_content_version_id),
    CONSTRAINT technology_analysis_work_items_completed_together CHECK (
        (status = 'complete' AND completed_at IS NOT NULL)
        OR (status <> 'complete' AND completed_at IS NULL)
    )
);

CREATE INDEX technology_analysis_work_items_pending_idx
    ON technology_analysis_work_items (configuration_id, queued_at, id)
    WHERE status = 'pending';

CREATE TABLE technology_analysis_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    configuration_id UUID NOT NULL REFERENCES technology_analysis_configurations(id),
    run_id UUID REFERENCES technology_analysis_runs(id),
    work_item_id UUID REFERENCES technology_analysis_work_items(id),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'error')),
    code TEXT NOT NULL CHECK (char_length(code) BETWEEN 1 AND 128),
    message TEXT NOT NULL CHECK (char_length(message) <= 512),
    diagnostics JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT technology_analysis_audit_events_diagnostics_small CHECK (octet_length(diagnostics::text) <= 8192)
);

CREATE INDEX technology_analysis_audit_events_run_time_idx
    ON technology_analysis_audit_events (run_id, occurred_at);
CREATE INDEX technology_analysis_audit_events_configuration_time_idx
    ON technology_analysis_audit_events (configuration_id, occurred_at DESC);
