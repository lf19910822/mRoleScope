-- Safe diagnostics are a closed set. Existing status columns stay textual for
-- backwards-compatible query interfaces, but database checks prevent callers
-- from storing provider errors, JD text, prompts, responses or credentials.

CREATE TYPE ai_safe_error_code AS ENUM (
    'input_too_large', 'budget_exhausted', 'provider_timeout',
    'provider_rate_limited', 'provider_server_error',
    'provider_authentication_failed', 'invalid_response_schema',
    'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
);

ALTER TABLE ai_analysis_configurations
    ADD CONSTRAINT ai_analysis_configurations_safe_failure_code_allowed CHECK (
        safe_failure_code IS NULL
        OR safe_failure_code IN (
            'input_too_large', 'budget_exhausted', 'provider_timeout',
            'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
ALTER TABLE ai_semantic_results
    ADD CONSTRAINT ai_semantic_results_safe_error_code_allowed CHECK (
        safe_error_code IS NULL
        OR safe_error_code IN (
            'input_too_large', 'budget_exhausted', 'provider_timeout',
            'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
ALTER TABLE ai_analysis_runs
    ADD CONSTRAINT ai_analysis_runs_safe_error_code_allowed CHECK (
        safe_error_code IS NULL
        OR safe_error_code IN (
            'input_too_large', 'budget_exhausted', 'provider_timeout',
            'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
ALTER TABLE ai_analysis_work_items
    ADD CONSTRAINT ai_analysis_work_items_last_safe_error_code_allowed CHECK (
        last_safe_error_code IS NULL
        OR last_safe_error_code IN (
            'input_too_large', 'budget_exhausted', 'provider_timeout',
            'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
ALTER TABLE ai_analysis_attempts
    ADD CONSTRAINT ai_analysis_attempts_safe_error_code_allowed CHECK (
        safe_error_code IS NULL
        OR safe_error_code IN (
            'input_too_large', 'budget_exhausted', 'provider_timeout',
            'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );

CREATE TYPE ai_audit_code AS ENUM (
    'candidate_prepared', 'work_queued', 'attempt_started',
    'attempt_completed', 'safe_failure_recorded', 'configuration_published'
);
CREATE TYPE ai_audit_severity AS ENUM ('info', 'warning', 'error');

CREATE TABLE ai_analysis_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_analysis_configuration_id UUID NOT NULL REFERENCES ai_analysis_configurations(id),
    run_id UUID REFERENCES ai_analysis_runs(id),
    work_item_id UUID REFERENCES ai_analysis_work_items(id),
    severity ai_audit_severity NOT NULL DEFAULT 'info',
    code ai_audit_code NOT NULL,
    safe_error_code ai_safe_error_code,
    logical_task_count INTEGER NOT NULL DEFAULT 0 CHECK (logical_task_count >= 0),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    input_token_count INTEGER NOT NULL DEFAULT 0 CHECK (input_token_count >= 0),
    output_token_count INTEGER NOT NULL DEFAULT 0 CHECK (output_token_count >= 0),
    charge_rmb NUMERIC(14, 6) NOT NULL DEFAULT 0 CHECK (charge_rmb >= 0),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ai_analysis_audit_events_failure_code_matches_event CHECK (
        (code = 'safe_failure_recorded' AND safe_error_code IS NOT NULL)
        OR (code <> 'safe_failure_recorded' AND safe_error_code IS NULL)
    )
);

CREATE INDEX ai_analysis_audit_events_configuration_time_idx
    ON ai_analysis_audit_events (ai_analysis_configuration_id, occurred_at DESC);
CREATE INDEX ai_analysis_audit_events_run_time_idx
    ON ai_analysis_audit_events (run_id, occurred_at DESC);
