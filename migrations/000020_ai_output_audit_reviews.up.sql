CREATE TYPE ai_output_audit_verdict AS ENUM (
    'prompt_noncompliance', 'schema_contract_mismatch',
    'evidence_contract_mismatch', 'policy_contract_mismatch',
    'unable_to_classify'
);

CREATE TABLE ai_output_audit_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_analysis_configuration_id UUID NOT NULL REFERENCES ai_analysis_configurations(id),
    work_item_id UUID NOT NULL REFERENCES ai_analysis_work_items(id),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    attempt_number INTEGER NOT NULL CHECK (attempt_number BETWEEN 1 AND 3),
    reviewer TEXT NOT NULL CHECK (reviewer = 'FAN'),
    output_sha256 CHAR(64) NOT NULL CHECK (output_sha256 ~ '^[0-9a-f]{64}$'),
    verdict ai_output_audit_verdict NOT NULL,
    reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (work_item_id, attempt_number, reviewer)
);

CREATE INDEX ai_output_audit_reviews_configuration_time_idx
    ON ai_output_audit_reviews (ai_analysis_configuration_id, reviewed_at DESC);

CREATE FUNCTION reject_ai_output_audit_review_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'AI output audit reviews are immutable';
END;
$$;

CREATE TRIGGER ai_output_audit_reviews_immutable
BEFORE UPDATE OR DELETE ON ai_output_audit_reviews
FOR EACH ROW EXECUTE FUNCTION reject_ai_output_audit_review_mutation();
