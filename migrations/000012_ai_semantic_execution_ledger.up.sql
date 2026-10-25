-- The execution ledger is independent from semantic conclusion state. It
-- records only opaque identifiers, bounded safe categories and usage totals;
-- it never stores model input, model output, source text, or credentials.

CREATE TYPE ai_analysis_work_status AS ENUM (
    'pending', 'running', 'complete', 'failed_safely'
);
CREATE TYPE ai_analysis_run_status AS ENUM (
    'running', 'complete', 'failed_safely'
);
CREATE TYPE ai_analysis_attempt_status AS ENUM (
    'running', 'succeeded', 'failed'
);
CREATE TYPE ai_analysis_trigger AS ENUM (
    'trial', 'expansion', 'new_body', 'manual_retry'
);

CREATE TABLE ai_analysis_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_analysis_configuration_id UUID NOT NULL REFERENCES ai_analysis_configurations(id),
    trigger ai_analysis_trigger NOT NULL,
    status ai_analysis_run_status NOT NULL DEFAULT 'running',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    claimed_count INTEGER NOT NULL DEFAULT 0 CHECK (claimed_count >= 0),
    completed_count INTEGER NOT NULL DEFAULT 0 CHECK (completed_count >= 0),
    failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    safe_error_code TEXT CHECK (
        safe_error_code IS NULL OR char_length(safe_error_code) BETWEEN 1 AND 128
    ),
    UNIQUE (id, ai_analysis_configuration_id),
    CONSTRAINT ai_analysis_runs_state_matches_completion CHECK (
        (status = 'running' AND completed_at IS NULL AND safe_error_code IS NULL)
        OR (status = 'complete' AND completed_at IS NOT NULL AND safe_error_code IS NULL)
        OR (status = 'failed_safely' AND completed_at IS NOT NULL AND safe_error_code IS NOT NULL)
    )
);

CREATE INDEX ai_analysis_runs_configuration_time_idx
    ON ai_analysis_runs (ai_analysis_configuration_id, started_at DESC);

CREATE TABLE ai_analysis_work_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_analysis_configuration_id UUID NOT NULL REFERENCES ai_analysis_configurations(id),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    trigger ai_analysis_trigger NOT NULL,
    status ai_analysis_work_status NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0 AND attempt_count <= 3),
    last_run_id UUID,
    last_safe_error_code TEXT CHECK (
        last_safe_error_code IS NULL OR char_length(last_safe_error_code) BETWEEN 1 AND 128
    ),
    queued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, ai_analysis_configuration_id),
    UNIQUE (body_content_version_id, ai_analysis_configuration_id),
    FOREIGN KEY (last_run_id, ai_analysis_configuration_id)
        REFERENCES ai_analysis_runs (id, ai_analysis_configuration_id),
    CONSTRAINT ai_analysis_work_items_state_matches_completion CHECK (
        (status IN ('pending', 'running') AND completed_at IS NULL)
        OR (status = 'complete' AND completed_at IS NOT NULL AND last_safe_error_code IS NULL)
        OR (status = 'failed_safely' AND completed_at IS NOT NULL AND last_safe_error_code IS NOT NULL)
    )
);

CREATE INDEX ai_analysis_work_items_pending_idx
    ON ai_analysis_work_items (ai_analysis_configuration_id, queued_at, id)
    WHERE status = 'pending';

CREATE TABLE ai_analysis_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_analysis_configuration_id UUID NOT NULL REFERENCES ai_analysis_configurations(id),
    work_item_id UUID NOT NULL,
    run_id UUID NOT NULL,
    attempt_number INTEGER NOT NULL CHECK (attempt_number BETWEEN 1 AND 3),
    status ai_analysis_attempt_status NOT NULL DEFAULT 'running',
    input_token_limit INTEGER NOT NULL CHECK (input_token_limit BETWEEN 1 AND 1000000),
    output_token_limit INTEGER NOT NULL CHECK (output_token_limit BETWEEN 1 AND 1000000),
    safe_error_code TEXT CHECK (
        safe_error_code IS NULL OR char_length(safe_error_code) BETWEEN 1 AND 128
    ),
    provider_request_id UUID NOT NULL DEFAULT gen_random_uuid(),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    FOREIGN KEY (work_item_id, ai_analysis_configuration_id)
        REFERENCES ai_analysis_work_items (id, ai_analysis_configuration_id),
    FOREIGN KEY (run_id, ai_analysis_configuration_id)
        REFERENCES ai_analysis_runs (id, ai_analysis_configuration_id),
    UNIQUE (work_item_id, attempt_number),
    UNIQUE (provider_request_id),
    CONSTRAINT ai_analysis_attempts_state_matches_completion CHECK (
        (status = 'running' AND completed_at IS NULL AND safe_error_code IS NULL)
        OR (status = 'succeeded' AND completed_at IS NOT NULL AND safe_error_code IS NULL)
        OR (status = 'failed' AND completed_at IS NOT NULL AND safe_error_code IS NOT NULL)
    )
);

CREATE INDEX ai_analysis_attempts_work_time_idx
    ON ai_analysis_attempts (work_item_id, started_at DESC);
CREATE INDEX ai_analysis_attempts_run_time_idx
    ON ai_analysis_attempts (run_id, started_at DESC);

CREATE TABLE ai_analysis_usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id UUID NOT NULL UNIQUE REFERENCES ai_analysis_attempts(id),
    input_tokens INTEGER NOT NULL CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL CHECK (output_tokens >= 0),
    estimated_charge_rmb NUMERIC(14, 6) NOT NULL CHECK (estimated_charge_rmb >= 0),
    actual_charge_rmb NUMERIC(14, 6) NOT NULL CHECK (actual_charge_rmb >= 0),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
