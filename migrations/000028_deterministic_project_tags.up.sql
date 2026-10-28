-- Deterministic project tags are title-only facts. They are separate from
-- source project metadata, JD bodies, technology analysis, and AI output.

CREATE TYPE project_tag_result_status AS ENUM ('complete');
CREATE TYPE project_tag_classification AS ENUM ('named_game', 'source_direction');

CREATE TABLE project_title_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title_sha256 CHAR(64) NOT NULL CHECK (title_sha256 ~ '^[0-9a-f]{64}$'),
    normalization_version TEXT NOT NULL CHECK (char_length(normalization_version) BETWEEN 1 AND 128),
    normalized_title TEXT NOT NULL CHECK (char_length(normalized_title) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (normalization_version, title_sha256)
);

CREATE TABLE project_title_observations (
    source_metadata_version_id UUID PRIMARY KEY REFERENCES job_source_metadata_versions(id),
    project_title_identity_id UUID NOT NULL REFERENCES project_title_identities(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX project_title_observations_identity_idx
    ON project_title_observations (project_title_identity_id, source_metadata_version_id);

CREATE TABLE project_tag_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_title_identity_id UUID NOT NULL REFERENCES project_title_identities(id),
    parser_version TEXT NOT NULL CHECK (char_length(parser_version) BETWEEN 1 AND 128),
    dictionary_version TEXT NOT NULL CHECK (char_length(dictionary_version) BETWEEN 1 AND 128),
    dictionary_sha256 CHAR(64) NOT NULL CHECK (dictionary_sha256 ~ '^[0-9a-f]{64}$'),
    status project_tag_result_status NOT NULL DEFAULT 'complete',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_title_identity_id, parser_version, dictionary_version, dictionary_sha256)
);

CREATE INDEX project_tag_results_pending_identity_idx
    ON project_tag_results (parser_version, dictionary_version, dictionary_sha256, project_title_identity_id);

CREATE TABLE project_tag_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_tag_result_id UUID NOT NULL REFERENCES project_tag_results(id) ON DELETE CASCADE,
    project_key TEXT NOT NULL CHECK (char_length(project_key) BETWEEN 1 AND 128),
    display_name TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 256),
    classification project_tag_classification NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_tag_result_id, project_key)
);

CREATE TABLE project_tag_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_tag_assignment_id UUID NOT NULL REFERENCES project_tag_assignments(id) ON DELETE CASCADE,
    matched_keyword TEXT NOT NULL CHECK (char_length(matched_keyword) BETWEEN 1 AND 256),
    normalization_version TEXT NOT NULL CHECK (char_length(normalization_version) BETWEEN 1 AND 128),
    normalized_title_sha256 CHAR(64) NOT NULL CHECK (normalized_title_sha256 ~ '^[0-9a-f]{64}$'),
    codepoint_start INTEGER NOT NULL CHECK (codepoint_start >= 0),
    codepoint_end INTEGER NOT NULL CHECK (codepoint_end > codepoint_start),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_tag_assignment_id, matched_keyword, codepoint_start, codepoint_end)
);

CREATE INDEX project_tag_evidence_assignment_coordinate_idx
    ON project_tag_evidence (project_tag_assignment_id, codepoint_start, codepoint_end);

-- Source observation links and every result/provenance record are immutable
-- historical facts. Reanalysis creates a new parser/dictionary identity.
CREATE FUNCTION reject_project_tag_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'deterministic project tags are immutable';
END;
$$;

CREATE TRIGGER project_title_identities_immutable
BEFORE UPDATE OR DELETE ON project_title_identities
FOR EACH ROW EXECUTE FUNCTION reject_project_tag_mutation();

CREATE TRIGGER project_title_observations_immutable
BEFORE UPDATE OR DELETE ON project_title_observations
FOR EACH ROW EXECUTE FUNCTION reject_project_tag_mutation();

CREATE TRIGGER project_tag_results_immutable
BEFORE UPDATE OR DELETE ON project_tag_results
FOR EACH ROW EXECUTE FUNCTION reject_project_tag_mutation();

CREATE TRIGGER project_tag_assignments_immutable
BEFORE UPDATE OR DELETE ON project_tag_assignments
FOR EACH ROW EXECUTE FUNCTION reject_project_tag_mutation();

CREATE TRIGGER project_tag_evidence_immutable
BEFORE UPDATE OR DELETE ON project_tag_evidence
FOR EACH ROW EXECUTE FUNCTION reject_project_tag_mutation();
