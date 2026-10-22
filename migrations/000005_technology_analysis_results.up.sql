CREATE TYPE technology_analysis_result_status AS ENUM ('pending', 'running', 'complete', 'failed');

CREATE TABLE technology_analysis_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    parser_version TEXT NOT NULL CHECK (char_length(parser_version) BETWEEN 1 AND 128),
    dictionary_version_id UUID NOT NULL REFERENCES technology_dictionary_versions(id),
    status technology_analysis_result_status NOT NULL DEFAULT 'pending',
    safe_failure_code TEXT CHECK (safe_failure_code IS NULL OR char_length(safe_failure_code) <= 128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT technology_analysis_results_complete_together CHECK (
        (status = 'complete' AND completed_at IS NOT NULL AND safe_failure_code IS NULL)
        OR (status <> 'complete')
    ),
    UNIQUE (body_content_version_id, parser_version, dictionary_version_id)
);

CREATE INDEX technology_analysis_results_configuration_idx
    ON technology_analysis_results (parser_version, dictionary_version_id, status);

CREATE TABLE technology_mentions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_result_id UUID NOT NULL REFERENCES technology_analysis_results(id) ON DELETE CASCADE,
    technology_identity_id UUID NOT NULL REFERENCES technology_identities(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (analysis_result_id, technology_identity_id)
);

CREATE TABLE technology_mention_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    technology_mention_id UUID NOT NULL REFERENCES technology_mentions(id) ON DELETE CASCADE,
    source_field TEXT NOT NULL CHECK (source_field IN ('description', 'jobRequire', 'addition')),
    normalization_version TEXT NOT NULL CHECK (char_length(normalization_version) BETWEEN 1 AND 128),
    normalized_field_sha256 CHAR(64) NOT NULL CHECK (normalized_field_sha256 ~ '^[0-9a-f]{64}$'),
    dictionary_rule_id UUID NOT NULL REFERENCES technology_dictionary_rules(id),
    matched_alias TEXT NOT NULL CHECK (char_length(matched_alias) BETWEEN 1 AND 256),
    codepoint_start INTEGER NOT NULL CHECK (codepoint_start >= 0),
    codepoint_end INTEGER NOT NULL CHECK (codepoint_end > codepoint_start),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (technology_mention_id, source_field, dictionary_rule_id, codepoint_start, codepoint_end)
);

CREATE INDEX technology_mention_evidence_result_lookup_idx
    ON technology_mention_evidence (technology_mention_id, source_field, codepoint_start);
