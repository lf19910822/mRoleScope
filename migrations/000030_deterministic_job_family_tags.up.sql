-- Deterministic job families are title-only, provider-free facts. They are
-- separate from project tags, body analysis, technology analysis, and AI.

CREATE TYPE job_family_observation_status AS ENUM (
    'in_scope', 'out_of_scope', 'scope_unknown', 'invalid_title'
);
CREATE TYPE job_family_result_status AS ENUM ('classified', 'unclassified');

CREATE TABLE job_family_title_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title_sha256 CHAR(64) NOT NULL CHECK (title_sha256 ~ '^[0-9a-f]{64}$'),
    normalization_version TEXT NOT NULL CHECK (char_length(normalization_version) BETWEEN 1 AND 128),
    normalized_title TEXT NOT NULL CHECK (char_length(normalized_title) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (normalization_version, title_sha256)
);

CREATE TABLE job_family_observations (
    source_metadata_version_id UUID PRIMARY KEY REFERENCES job_source_metadata_versions(id),
    job_family_title_identity_id UUID REFERENCES job_family_title_identities(id),
    status job_family_observation_status NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT job_family_observations_title_identity_matches_status CHECK (
        (status = 'in_scope' AND job_family_title_identity_id IS NOT NULL)
        OR (status = 'invalid_title' AND job_family_title_identity_id IS NULL)
        OR status IN ('out_of_scope', 'scope_unknown')
    )
);

CREATE INDEX job_family_observations_identity_idx
    ON job_family_observations (job_family_title_identity_id, source_metadata_version_id);

CREATE TABLE job_family_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_family_title_identity_id UUID NOT NULL REFERENCES job_family_title_identities(id),
    parser_version TEXT NOT NULL CHECK (char_length(parser_version) BETWEEN 1 AND 128),
    dictionary_version TEXT NOT NULL CHECK (char_length(dictionary_version) BETWEEN 1 AND 128),
    dictionary_sha256 CHAR(64) NOT NULL CHECK (dictionary_sha256 ~ '^[0-9a-f]{64}$'),
    project_mask_parser_version TEXT NOT NULL CHECK (char_length(project_mask_parser_version) BETWEEN 1 AND 128),
    project_mask_dictionary_version TEXT NOT NULL CHECK (char_length(project_mask_dictionary_version) BETWEEN 1 AND 128),
    project_mask_dictionary_sha256 CHAR(64) NOT NULL CHECK (project_mask_dictionary_sha256 ~ '^[0-9a-f]{64}$'),
    status job_family_result_status NOT NULL,
    primary_family_key TEXT CHECK (char_length(primary_family_key) BETWEEN 1 AND 128),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT job_family_results_primary_matches_status CHECK (
        (status = 'classified' AND primary_family_key IS NOT NULL)
        OR (status = 'unclassified' AND primary_family_key IS NULL)
    ),
    UNIQUE (
        job_family_title_identity_id, parser_version, dictionary_version, dictionary_sha256,
        project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256
    )
);

CREATE INDEX job_family_results_pending_identity_idx
    ON job_family_results (
        parser_version, dictionary_version, dictionary_sha256,
        project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256,
        job_family_title_identity_id
    );

CREATE TABLE job_family_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_family_result_id UUID NOT NULL REFERENCES job_family_results(id) ON DELETE CASCADE,
    family_key TEXT NOT NULL CHECK (char_length(family_key) BETWEEN 1 AND 128),
    match_count INTEGER NOT NULL CHECK (match_count > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_family_result_id, family_key)
);

CREATE TABLE job_family_match_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_family_match_id UUID NOT NULL REFERENCES job_family_matches(id) ON DELETE CASCADE,
    matched_alias TEXT NOT NULL CHECK (char_length(matched_alias) BETWEEN 1 AND 256),
    normalization_version TEXT NOT NULL CHECK (char_length(normalization_version) BETWEEN 1 AND 128),
    normalized_title_sha256 CHAR(64) NOT NULL CHECK (normalized_title_sha256 ~ '^[0-9a-f]{64}$'),
    codepoint_start INTEGER NOT NULL CHECK (codepoint_start >= 0),
    codepoint_end INTEGER NOT NULL CHECK (codepoint_end > codepoint_start),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_family_match_id, matched_alias, codepoint_start, codepoint_end)
);

CREATE INDEX job_family_match_evidence_coordinate_idx
    ON job_family_match_evidence (job_family_match_id, codepoint_start, codepoint_end);

CREATE TABLE job_family_retry_audits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_family_title_identity_id UUID NOT NULL REFERENCES job_family_title_identities(id),
    parser_version TEXT NOT NULL CHECK (char_length(parser_version) BETWEEN 1 AND 128),
    dictionary_version TEXT NOT NULL CHECK (char_length(dictionary_version) BETWEEN 1 AND 128),
    dictionary_sha256 CHAR(64) NOT NULL CHECK (dictionary_sha256 ~ '^[0-9a-f]{64}$'),
    project_mask_parser_version TEXT NOT NULL CHECK (char_length(project_mask_parser_version) BETWEEN 1 AND 128),
    project_mask_dictionary_version TEXT NOT NULL CHECK (char_length(project_mask_dictionary_version) BETWEEN 1 AND 128),
    project_mask_dictionary_sha256 CHAR(64) NOT NULL CHECK (project_mask_dictionary_sha256 ~ '^[0-9a-f]{64}$'),
    safe_error_code TEXT NOT NULL CHECK (char_length(safe_error_code) BETWEEN 1 AND 128),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX job_family_retry_audits_identity_idx
    ON job_family_retry_audits (job_family_title_identity_id, occurred_at);

-- An observation can fail before a title identity exists. Retain only the
-- source-metadata reference, processing identity, and stable safe code so
-- this local failure remains retryable without copying source text.
CREATE TABLE job_family_observation_retry_audits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_metadata_version_id UUID NOT NULL REFERENCES job_source_metadata_versions(id),
    parser_version TEXT NOT NULL CHECK (char_length(parser_version) BETWEEN 1 AND 128),
    dictionary_version TEXT NOT NULL CHECK (char_length(dictionary_version) BETWEEN 1 AND 128),
    dictionary_sha256 CHAR(64) NOT NULL CHECK (dictionary_sha256 ~ '^[0-9a-f]{64}$'),
    project_mask_parser_version TEXT NOT NULL CHECK (char_length(project_mask_parser_version) BETWEEN 1 AND 128),
    project_mask_dictionary_version TEXT NOT NULL CHECK (char_length(project_mask_dictionary_version) BETWEEN 1 AND 128),
    project_mask_dictionary_sha256 CHAR(64) NOT NULL CHECK (project_mask_dictionary_sha256 ~ '^[0-9a-f]{64}$'),
    safe_error_code TEXT NOT NULL CHECK (char_length(safe_error_code) BETWEEN 1 AND 128),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX job_family_observation_retry_audits_metadata_idx
    ON job_family_observation_retry_audits (source_metadata_version_id, occurred_at);

CREATE FUNCTION reject_job_family_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'deterministic job-family records are immutable';
END;
$$;

CREATE TRIGGER job_family_title_identities_immutable
BEFORE UPDATE OR DELETE ON job_family_title_identities
FOR EACH ROW EXECUTE FUNCTION reject_job_family_mutation();

CREATE TRIGGER job_family_observations_immutable
BEFORE UPDATE OR DELETE ON job_family_observations
FOR EACH ROW EXECUTE FUNCTION reject_job_family_mutation();

CREATE TRIGGER job_family_results_immutable
BEFORE UPDATE OR DELETE ON job_family_results
FOR EACH ROW EXECUTE FUNCTION reject_job_family_mutation();

CREATE TRIGGER job_family_matches_immutable
BEFORE UPDATE OR DELETE ON job_family_matches
FOR EACH ROW EXECUTE FUNCTION reject_job_family_mutation();

CREATE TRIGGER job_family_match_evidence_immutable
BEFORE UPDATE OR DELETE ON job_family_match_evidence
FOR EACH ROW EXECUTE FUNCTION reject_job_family_mutation();

CREATE TRIGGER job_family_retry_audits_immutable
BEFORE UPDATE OR DELETE ON job_family_retry_audits
FOR EACH ROW EXECUTE FUNCTION reject_job_family_mutation();

CREATE TRIGGER job_family_observation_retry_audits_immutable
BEFORE UPDATE OR DELETE ON job_family_observation_retry_audits
FOR EACH ROW EXECUTE FUNCTION reject_job_family_mutation();
