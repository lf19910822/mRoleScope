-- Deterministic experience requirements are provider-free facts. They are
-- intentionally independent from AI semantic projections and retain only
-- versioned parser provenance plus replayable source coordinates.

CREATE TYPE experience_requirement_result_status AS ENUM (
    'not_configured', 'uncertain', 'configured'
);
CREATE TYPE experience_requirement_modality AS ENUM (
    'required', 'preferred', 'conditional', 'uncertain'
);

CREATE TABLE experience_requirement_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    parser_version TEXT NOT NULL CHECK (char_length(parser_version) BETWEEN 1 AND 128),
    lexicon_version TEXT NOT NULL CHECK (char_length(lexicon_version) BETWEEN 1 AND 128),
    lexicon_sha256 CHAR(64) NOT NULL CHECK (lexicon_sha256 ~ '^[0-9a-f]{64}$'),
    status experience_requirement_result_status NOT NULL,
    base_year INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT experience_requirement_results_base_year_matches_status CHECK (
        (status = 'configured' AND base_year IS NOT NULL AND base_year > 0)
        OR (status IN ('not_configured', 'uncertain') AND base_year IS NULL)
    ),
    UNIQUE (body_content_version_id, parser_version, lexicon_version, lexicon_sha256)
);

CREATE INDEX experience_requirement_results_pending_identity_idx
    ON experience_requirement_results (parser_version, lexicon_version, lexicon_sha256, body_content_version_id);

CREATE TABLE experience_requirement_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    experience_requirement_result_id UUID NOT NULL REFERENCES experience_requirement_results(id) ON DELETE CASCADE,
    rule_key TEXT NOT NULL CHECK (char_length(rule_key) BETWEEN 1 AND 128),
    modality experience_requirement_modality NOT NULL,
    minimum_years INTEGER NOT NULL CHECK (minimum_years > 0),
    source_field TEXT NOT NULL CHECK (source_field IN ('description', 'jobRequire', 'addition')),
    normalization_version TEXT NOT NULL CHECK (char_length(normalization_version) BETWEEN 1 AND 128),
    normalized_field_sha256 CHAR(64) NOT NULL CHECK (normalized_field_sha256 ~ '^[0-9a-f]{64}$'),
    codepoint_start INTEGER NOT NULL CHECK (codepoint_start >= 0),
    codepoint_end INTEGER NOT NULL CHECK (codepoint_end > codepoint_start),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (experience_requirement_result_id, source_field, rule_key, codepoint_start, codepoint_end)
);

CREATE INDEX experience_requirement_evidence_result_coordinate_idx
    ON experience_requirement_evidence (experience_requirement_result_id, source_field, codepoint_start);

-- Deterministic interpretations are append-only historical facts. A parser or
-- lexicon revision creates a distinct identity rather than rewriting history.
CREATE FUNCTION reject_experience_requirement_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'deterministic experience requirements are immutable';
END;
$$;

CREATE TRIGGER experience_requirement_results_immutable
BEFORE UPDATE OR DELETE ON experience_requirement_results
FOR EACH ROW EXECUTE FUNCTION reject_experience_requirement_mutation();

CREATE TRIGGER experience_requirement_evidence_immutable
BEFORE UPDATE OR DELETE ON experience_requirement_evidence
FOR EACH ROW EXECUTE FUNCTION reject_experience_requirement_mutation();
