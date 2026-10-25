-- AI semantic configurations are intentionally separate from deterministic
-- technology-analysis configurations.  Their frozen digest set is sufficient
-- to reproduce the executable contract without retaining a prompt, JD text,
-- request, response, or credential in PostgreSQL.

CREATE TYPE ai_analysis_provider AS ENUM ('bailian');
CREATE TYPE ai_analysis_configuration_status AS ENUM (
    'candidate', 'comparison', 'published', 'failed', 'retired'
);

CREATE TABLE ai_analysis_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    technology_analysis_configuration_id UUID NOT NULL
        REFERENCES technology_analysis_configurations(id),
    provider ai_analysis_provider NOT NULL,
    model TEXT NOT NULL CHECK (char_length(model) BETWEEN 1 AND 128),
    configuration_sha256 CHAR(64) NOT NULL UNIQUE
        CHECK (configuration_sha256 ~ '^[0-9a-f]{64}$'),
    request_profile_sha256 CHAR(64) NOT NULL
        CHECK (request_profile_sha256 ~ '^[0-9a-f]{64}$'),
    prompt_sha256 CHAR(64) NOT NULL CHECK (prompt_sha256 ~ '^[0-9a-f]{64}$'),
    response_schema_version TEXT NOT NULL
        CHECK (char_length(response_schema_version) BETWEEN 1 AND 128),
    response_schema_sha256 CHAR(64) NOT NULL
        CHECK (response_schema_sha256 ~ '^[0-9a-f]{64}$'),
    input_contract_version TEXT NOT NULL
        CHECK (char_length(input_contract_version) BETWEEN 1 AND 128),
    input_contract_sha256 CHAR(64) NOT NULL
        CHECK (input_contract_sha256 ~ '^[0-9a-f]{64}$'),
    validator_version TEXT NOT NULL
        CHECK (char_length(validator_version) BETWEEN 1 AND 128),
    validator_sha256 CHAR(64) NOT NULL
        CHECK (validator_sha256 ~ '^[0-9a-f]{64}$'),
    pricing_profile_version TEXT NOT NULL
        CHECK (char_length(pricing_profile_version) BETWEEN 1 AND 128),
    pricing_profile_sha256 CHAR(64) NOT NULL
        CHECK (pricing_profile_sha256 ~ '^[0-9a-f]{64}$'),
    status ai_analysis_configuration_status NOT NULL DEFAULT 'candidate',
    created_by TEXT NOT NULL CHECK (char_length(created_by) BETWEEN 1 AND 256),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    safe_failure_code TEXT CHECK (
        safe_failure_code IS NULL OR char_length(safe_failure_code) BETWEEN 1 AND 128
    ),
    CONSTRAINT ai_analysis_configurations_status_timestamps_match CHECK (
        (status IN ('candidate', 'comparison')
            AND published_at IS NULL AND failed_at IS NULL AND retired_at IS NULL
            AND safe_failure_code IS NULL)
        OR (status = 'published'
            AND published_at IS NOT NULL AND failed_at IS NULL AND retired_at IS NULL
            AND safe_failure_code IS NULL)
        OR (status = 'failed'
            AND published_at IS NULL AND failed_at IS NOT NULL AND retired_at IS NULL
            AND safe_failure_code IS NOT NULL)
        OR (status = 'retired'
            AND failed_at IS NULL AND retired_at IS NOT NULL)
    )
);

CREATE INDEX ai_analysis_configurations_technology_configuration_idx
    ON ai_analysis_configurations (technology_analysis_configuration_id, created_at DESC);

-- A semantic configuration must be based on an actual released deterministic
-- evidence snapshot.  Later deterministic releases do not mutate prior AI
-- provenance because this guard only applies when the reference is created.
CREATE FUNCTION require_published_ai_technology_configuration()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    referenced_status technology_analysis_configuration_status;
BEGIN
    SELECT status INTO referenced_status
    FROM technology_analysis_configurations
    WHERE id = NEW.technology_analysis_configuration_id;

    IF referenced_status IS DISTINCT FROM 'published' THEN
        RAISE EXCEPTION 'AI analysis configuration requires a published technology analysis configuration';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER ai_analysis_configurations_require_published_technology_configuration
BEFORE INSERT OR UPDATE OF technology_analysis_configuration_id ON ai_analysis_configurations
FOR EACH ROW EXECUTE FUNCTION require_published_ai_technology_configuration();

-- Configuration inputs are immutable.  Lifecycle fields may move only along
-- the controlled state graph; release policy validation remains in the
-- repository/operation layer and is added before any promotion path exists.
CREATE FUNCTION reject_ai_analysis_configuration_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'AI analysis configurations are immutable';
    END IF;

    IF OLD.technology_analysis_configuration_id IS DISTINCT FROM NEW.technology_analysis_configuration_id
       OR OLD.provider IS DISTINCT FROM NEW.provider
       OR OLD.model IS DISTINCT FROM NEW.model
       OR OLD.configuration_sha256 IS DISTINCT FROM NEW.configuration_sha256
       OR OLD.request_profile_sha256 IS DISTINCT FROM NEW.request_profile_sha256
       OR OLD.prompt_sha256 IS DISTINCT FROM NEW.prompt_sha256
       OR OLD.response_schema_version IS DISTINCT FROM NEW.response_schema_version
       OR OLD.response_schema_sha256 IS DISTINCT FROM NEW.response_schema_sha256
       OR OLD.input_contract_version IS DISTINCT FROM NEW.input_contract_version
       OR OLD.input_contract_sha256 IS DISTINCT FROM NEW.input_contract_sha256
       OR OLD.validator_version IS DISTINCT FROM NEW.validator_version
       OR OLD.validator_sha256 IS DISTINCT FROM NEW.validator_sha256
       OR OLD.pricing_profile_version IS DISTINCT FROM NEW.pricing_profile_version
       OR OLD.pricing_profile_sha256 IS DISTINCT FROM NEW.pricing_profile_sha256
       OR OLD.created_by IS DISTINCT FROM NEW.created_by
       OR OLD.created_at IS DISTINCT FROM NEW.created_at THEN
        RAISE EXCEPTION 'AI analysis configuration inputs are immutable';
    END IF;

    IF OLD.status = NEW.status THEN
        RAISE EXCEPTION 'AI analysis configuration lifecycle update has no state transition';
    END IF;

    IF NOT (
        (OLD.status = 'candidate' AND NEW.status IN ('published', 'failed', 'retired'))
        OR (OLD.status = 'comparison' AND NEW.status IN ('failed', 'retired'))
        OR (OLD.status = 'published' AND NEW.status = 'retired')
    ) THEN
        RAISE EXCEPTION 'AI analysis configuration lifecycle transition is invalid';
    END IF;

    IF OLD.status = 'candidate' AND NEW.status = 'published' AND NEW.model <> 'qwen-plus' THEN
        RAISE EXCEPTION 'only qwen-plus candidates can be published';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER ai_analysis_configurations_immutable
BEFORE UPDATE OR DELETE ON ai_analysis_configurations
FOR EACH ROW EXECUTE FUNCTION reject_ai_analysis_configuration_mutation();
