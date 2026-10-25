-- Execution state and semantic state are deliberately orthogonal. A work item
-- can fail safely without falsely claiming that a JD has no requirement.

CREATE TYPE ai_semantic_execution_status AS ENUM (
    'pending', 'running', 'complete', 'failed_safely'
);
CREATE TYPE ai_semantic_value_status AS ENUM (
    'not_analyzed', 'not_configured', 'uncertain', 'configured'
);

CREATE TABLE ai_semantic_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    ai_analysis_configuration_id UUID NOT NULL REFERENCES ai_analysis_configurations(id),
    execution_status ai_semantic_execution_status NOT NULL DEFAULT 'pending',
    semantic_status ai_semantic_value_status NOT NULL DEFAULT 'not_analyzed',
    safe_error_code TEXT CHECK (
        safe_error_code IS NULL OR char_length(safe_error_code) BETWEEN 1 AND 128
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT ai_semantic_results_state_matches_execution CHECK (
        (execution_status IN ('pending', 'running')
            AND semantic_status = 'not_analyzed'
            AND safe_error_code IS NULL
            AND completed_at IS NULL)
        OR (execution_status = 'complete'
            AND semantic_status IN ('not_configured', 'uncertain', 'configured')
            AND safe_error_code IS NULL
            AND completed_at IS NOT NULL)
        OR (execution_status = 'failed_safely'
            AND semantic_status = 'not_analyzed'
            AND safe_error_code IS NOT NULL
            AND completed_at IS NOT NULL)
    ),
    UNIQUE (body_content_version_id, ai_analysis_configuration_id)
);

CREATE INDEX ai_semantic_results_configuration_status_idx
    ON ai_semantic_results (ai_analysis_configuration_id, execution_status, created_at);
