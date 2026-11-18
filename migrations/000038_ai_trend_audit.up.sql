CREATE TYPE ai_trend_audit_status AS ENUM (
    'running', 'success', 'partial', 'failure', 'user_stopped'
);

CREATE TYPE ai_trend_audit_tool_status AS ENUM (
    'running', 'completed', 'failed', 'skipped'
);

CREATE TABLE ai_trend_request_audits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_hash CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    intent_hash CHAR(64) CHECK (intent_hash IS NULL OR intent_hash ~ '^[0-9a-f]{64}$'),
    draft_hash CHAR(64) CHECK (draft_hash IS NULL OR draft_hash ~ '^[0-9a-f]{64}$'),
    contract_version TEXT NOT NULL CHECK (char_length(contract_version) BETWEEN 1 AND 128),
    validation_status TEXT NOT NULL CHECK (validation_status IN ('not_started', 'valid', 'invalid', 'corrected')),
    status ai_trend_audit_status NOT NULL,
    safe_error_code TEXT CHECK (safe_error_code IS NULL OR safe_error_code ~ '^[a-z][a-z0-9_]{0,127}$'),
    ai_attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (ai_attempt_count BETWEEN 0 AND 3),
    tool_call_count INTEGER NOT NULL DEFAULT 0 CHECK (tool_call_count BETWEEN 0 AND 20),
    tool_failure_count INTEGER NOT NULL DEFAULT 0 CHECK (tool_failure_count >= 0),
    input_token_count INTEGER NOT NULL DEFAULT 0 CHECK (input_token_count >= 0),
    output_token_count INTEGER NOT NULL DEFAULT 0 CHECK (output_token_count >= 0),
    duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    draft_curve_count INTEGER NOT NULL DEFAULT 0 CHECK (draft_curve_count BETWEEN 0 AND 8),
    draft_selected_count INTEGER NOT NULL DEFAULT 0 CHECK (draft_selected_count BETWEEN 0 AND 8),
    draft_warning_count INTEGER NOT NULL DEFAULT 0 CHECK (draft_warning_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT ai_trend_request_audits_status_times CHECK (
        (status = 'running' AND completed_at IS NULL)
        OR (status <> 'running' AND completed_at IS NOT NULL)
    ),
    CONSTRAINT ai_trend_request_audits_expiry_after_creation CHECK (expires_at >= created_at)
);

CREATE INDEX ai_trend_request_audits_expiry_idx
    ON ai_trend_request_audits (expires_at, id);

CREATE INDEX ai_trend_request_audits_created_idx
    ON ai_trend_request_audits (created_at DESC, id DESC);

CREATE TABLE ai_trend_request_audit_tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_audit_id UUID NOT NULL REFERENCES ai_trend_request_audits(id) ON DELETE CASCADE,
    sequence_number INTEGER NOT NULL CHECK (sequence_number BETWEEN 1 AND 20),
    tool_name TEXT NOT NULL CHECK (tool_name ~ '^[a-z][a-z0-9_]{0,127}$'),
    status ai_trend_audit_tool_status NOT NULL,
    safe_error_code TEXT CHECK (safe_error_code IS NULL OR safe_error_code ~ '^[a-z][a-z0-9_]{0,127}$'),
    duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE (request_audit_id, sequence_number),
    CONSTRAINT ai_trend_request_audit_tools_status_times CHECK (
        (status = 'running' AND completed_at IS NULL)
        OR (status <> 'running' AND completed_at IS NOT NULL)
    )
);

CREATE INDEX ai_trend_request_audit_tools_request_idx
    ON ai_trend_request_audit_tools (request_audit_id, sequence_number);

CREATE INDEX ai_trend_request_audit_tools_status_idx
    ON ai_trend_request_audit_tools (status, started_at DESC);
