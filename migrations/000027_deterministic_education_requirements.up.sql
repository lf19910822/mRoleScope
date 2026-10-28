-- Deterministic education requirements are independent from AI semantic
-- projections. They retain only versioned rule provenance and coordinates;
-- no duplicate JD snippet, prompt, model response, or credential is stored.

CREATE TYPE education_requirement_result_status AS ENUM (
    'not_configured', 'uncertain', 'configured'
);
CREATE TYPE deterministic_education_level AS ENUM (
    'associate_or_lower', 'bachelor', 'master', 'doctor'
);
CREATE TYPE education_requirement_modality AS ENUM (
    'required', 'preferred', 'unrestricted'
);

CREATE TABLE education_requirement_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    parser_version TEXT NOT NULL CHECK (char_length(parser_version) BETWEEN 1 AND 128),
    lexicon_version TEXT NOT NULL CHECK (char_length(lexicon_version) BETWEEN 1 AND 128),
    lexicon_sha256 CHAR(64) NOT NULL CHECK (lexicon_sha256 ~ '^[0-9a-f]{64}$'),
    status education_requirement_result_status NOT NULL,
    lower_bound_level deterministic_education_level,
    upper_bound_level deterministic_education_level,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT education_requirement_results_bounds_match_status CHECK (
        (status = 'configured' AND (lower_bound_level IS NOT NULL OR upper_bound_level IS NOT NULL))
        OR (status IN ('not_configured', 'uncertain')
            AND lower_bound_level IS NULL
            AND upper_bound_level IS NULL)
    ),
    UNIQUE (body_content_version_id, parser_version, lexicon_version, lexicon_sha256)
);

CREATE INDEX education_requirement_results_pending_identity_idx
    ON education_requirement_results (parser_version, lexicon_version, lexicon_sha256, body_content_version_id);

CREATE TABLE education_requirement_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    education_requirement_result_id UUID NOT NULL REFERENCES education_requirement_results(id) ON DELETE CASCADE,
    rule_key TEXT NOT NULL CHECK (char_length(rule_key) BETWEEN 1 AND 128),
    modality education_requirement_modality NOT NULL,
    source_field TEXT NOT NULL CHECK (source_field IN ('description', 'jobRequire', 'addition')),
    normalization_version TEXT NOT NULL CHECK (char_length(normalization_version) BETWEEN 1 AND 128),
    normalized_field_sha256 CHAR(64) NOT NULL CHECK (normalized_field_sha256 ~ '^[0-9a-f]{64}$'),
    codepoint_start INTEGER NOT NULL CHECK (codepoint_start >= 0),
    codepoint_end INTEGER NOT NULL CHECK (codepoint_end > codepoint_start),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (education_requirement_result_id, source_field, rule_key, codepoint_start, codepoint_end)
);

CREATE INDEX education_requirement_evidence_result_coordinate_idx
    ON education_requirement_evidence (education_requirement_result_id, source_field, codepoint_start);

-- The result identity and its evidence are append-only historical facts. A
-- later parser or lexicon release deliberately creates a new identity rather
-- than rewriting an earlier interpretation.
CREATE FUNCTION reject_education_requirement_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'deterministic education requirements are immutable';
END;
$$;

CREATE TRIGGER education_requirement_results_immutable
BEFORE UPDATE OR DELETE ON education_requirement_results
FOR EACH ROW EXECUTE FUNCTION reject_education_requirement_mutation();

CREATE TRIGGER education_requirement_evidence_immutable
BEFORE UPDATE OR DELETE ON education_requirement_evidence
FOR EACH ROW EXECUTE FUNCTION reject_education_requirement_mutation();
