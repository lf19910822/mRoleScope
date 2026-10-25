ALTER TYPE ai_safe_error_code ADD VALUE IF NOT EXISTS 'output_audit_capture_failed';

ALTER TABLE ai_analysis_configurations
    DROP CONSTRAINT ai_analysis_configurations_safe_failure_code_allowed,
    ADD CONSTRAINT ai_analysis_configurations_safe_failure_code_allowed CHECK (
        safe_failure_code IS NULL OR safe_failure_code IN (
            'input_too_large', 'output_audit_capture_failed', 'budget_exhausted',
            'provider_timeout', 'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
ALTER TABLE ai_semantic_results
    DROP CONSTRAINT ai_semantic_results_safe_error_code_allowed,
    ADD CONSTRAINT ai_semantic_results_safe_error_code_allowed CHECK (
        safe_error_code IS NULL OR safe_error_code IN (
            'input_too_large', 'output_audit_capture_failed', 'budget_exhausted',
            'provider_timeout', 'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
ALTER TABLE ai_analysis_runs
    DROP CONSTRAINT ai_analysis_runs_safe_error_code_allowed,
    ADD CONSTRAINT ai_analysis_runs_safe_error_code_allowed CHECK (
        safe_error_code IS NULL OR safe_error_code IN (
            'input_too_large', 'output_audit_capture_failed', 'budget_exhausted',
            'provider_timeout', 'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
ALTER TABLE ai_analysis_work_items
    DROP CONSTRAINT ai_analysis_work_items_last_safe_error_code_allowed,
    ADD CONSTRAINT ai_analysis_work_items_last_safe_error_code_allowed CHECK (
        last_safe_error_code IS NULL OR last_safe_error_code IN (
            'input_too_large', 'output_audit_capture_failed', 'budget_exhausted',
            'provider_timeout', 'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
ALTER TABLE ai_analysis_attempts
    DROP CONSTRAINT ai_analysis_attempts_safe_error_code_allowed,
    ADD CONSTRAINT ai_analysis_attempts_safe_error_code_allowed CHECK (
        safe_error_code IS NULL OR safe_error_code IN (
            'input_too_large', 'output_audit_capture_failed', 'budget_exhausted',
            'provider_timeout', 'provider_rate_limited', 'provider_server_error',
            'provider_authentication_failed', 'invalid_response_schema',
            'invalid_evidence_reference', 'policy_rejected', 'configuration_invalid'
        )
    );
