-- The Serving inventory is a DWD projection of immutable source and formal
-- analysis facts. It is deliberately append-only: every successfully
-- reconciled published run keeps its own member set for future chart queries,
-- while the current view gives the application a bounded latest-data lookup.
-- No ODS table or source-publication transaction is changed by this migration.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TYPE serving_jd_activity_state AS ENUM ('effective', 'confirmed_closed');
CREATE TYPE serving_jd_formal_dimension AS ENUM (
    'education', 'experience', 'technology', 'project', 'job_family'
);

CREATE TABLE serving_jd_inventory_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    published_run_id UUID NOT NULL REFERENCES collection_runs(id),
    observation_scope_id UUID NOT NULL REFERENCES observation_scopes(id),
    job_id UUID NOT NULL REFERENCES jobs(id),
    presence_period_id UUID NOT NULL REFERENCES job_presence_periods(id),
    source_metadata_version_id UUID NOT NULL REFERENCES job_source_metadata_versions(id),
    body_content_version_id UUID NOT NULL REFERENCES job_body_content_versions(id),
    activity_state serving_jd_activity_state NOT NULL DEFAULT 'effective',
    title TEXT NOT NULL DEFAULT '',
    locations TEXT[] NOT NULL DEFAULT '{}',
    is_urgent BOOLEAN NOT NULL DEFAULT FALSE,
    first_discovered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT serving_jd_inventory_memberships_one_job_per_run UNIQUE (published_run_id, job_id)
);

CREATE INDEX serving_jd_inventory_memberships_current_lookup_idx
    ON serving_jd_inventory_memberships (observation_scope_id, published_run_id, activity_state, job_id);
CREATE INDEX serving_jd_inventory_memberships_job_history_idx
    ON serving_jd_inventory_memberships (job_id, published_run_id DESC);
CREATE INDEX serving_jd_inventory_memberships_title_trgm_idx
    ON serving_jd_inventory_memberships USING GIN (title gin_trgm_ops);
CREATE INDEX serving_jd_inventory_memberships_locations_idx
    ON serving_jd_inventory_memberships USING GIN (locations);

-- A later deterministic result must not rewrite the source snapshot. Every
-- reconciliation therefore appends its selected formal result here. A query
-- prefers selection_priority 0 (same observed version) to 1 (same-job
-- fallback), then the newest selection within that priority.
CREATE TABLE serving_jd_dimension_resolutions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    serving_jd_inventory_membership_id UUID NOT NULL
        REFERENCES serving_jd_inventory_memberships(id),
    dimension serving_jd_formal_dimension NOT NULL,
    selection_priority SMALLINT NOT NULL CHECK (selection_priority IN (0, 1)),
    resolution_key TEXT NOT NULL CHECK (char_length(resolution_key) BETWEEN 1 AND 256),
    education_requirement_result_id UUID REFERENCES education_requirement_results(id),
    experience_requirement_result_id UUID REFERENCES experience_requirement_results(id),
    technology_analysis_result_id UUID REFERENCES technology_analysis_results(id),
    project_tag_result_id UUID REFERENCES project_tag_results(id),
    job_family_result_id UUID REFERENCES job_family_results(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT serving_jd_dimension_resolutions_one_typed_result CHECK (
        (dimension = 'education'
            AND education_requirement_result_id IS NOT NULL
            AND experience_requirement_result_id IS NULL
            AND technology_analysis_result_id IS NULL
            AND project_tag_result_id IS NULL
            AND job_family_result_id IS NULL)
        OR (dimension = 'experience'
            AND education_requirement_result_id IS NULL
            AND experience_requirement_result_id IS NOT NULL
            AND technology_analysis_result_id IS NULL
            AND project_tag_result_id IS NULL
            AND job_family_result_id IS NULL)
        OR (dimension = 'technology'
            AND education_requirement_result_id IS NULL
            AND experience_requirement_result_id IS NULL
            AND technology_analysis_result_id IS NOT NULL
            AND project_tag_result_id IS NULL
            AND job_family_result_id IS NULL)
        OR (dimension = 'project'
            AND education_requirement_result_id IS NULL
            AND experience_requirement_result_id IS NULL
            AND technology_analysis_result_id IS NULL
            AND project_tag_result_id IS NOT NULL
            AND job_family_result_id IS NULL)
        OR (dimension = 'job_family'
            AND education_requirement_result_id IS NULL
            AND experience_requirement_result_id IS NULL
            AND technology_analysis_result_id IS NULL
            AND project_tag_result_id IS NULL
            AND job_family_result_id IS NOT NULL)
    ),
    CONSTRAINT serving_jd_dimension_resolutions_unique_result
        UNIQUE (serving_jd_inventory_membership_id, dimension, resolution_key)
);

CREATE INDEX serving_jd_dimension_resolutions_current_idx
    ON serving_jd_dimension_resolutions (
        serving_jd_inventory_membership_id, dimension, selection_priority,
        created_at DESC, id DESC
    );
CREATE INDEX serving_jd_dimension_resolutions_education_idx
    ON serving_jd_dimension_resolutions (education_requirement_result_id)
    WHERE education_requirement_result_id IS NOT NULL;
CREATE INDEX serving_jd_dimension_resolutions_experience_idx
    ON serving_jd_dimension_resolutions (experience_requirement_result_id)
    WHERE experience_requirement_result_id IS NOT NULL;
CREATE INDEX serving_jd_dimension_resolutions_technology_idx
    ON serving_jd_dimension_resolutions (technology_analysis_result_id)
    WHERE technology_analysis_result_id IS NOT NULL;
CREATE INDEX serving_jd_dimension_resolutions_project_idx
    ON serving_jd_dimension_resolutions (project_tag_result_id)
    WHERE project_tag_result_id IS NOT NULL;
CREATE INDEX serving_jd_dimension_resolutions_family_idx
    ON serving_jd_dimension_resolutions (job_family_result_id)
    WHERE job_family_result_id IS NOT NULL;

CREATE VIEW serving_jd_current_inventory_memberships AS
SELECT membership.*
FROM serving_jd_inventory_memberships AS membership
JOIN observation_scope_publication_state AS publication
  ON publication.observation_scope_id = membership.observation_scope_id
 AND publication.published_run_id = membership.published_run_id;

CREATE TABLE serving_jd_reconciliation_audits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    published_run_id UUID NOT NULL REFERENCES collection_runs(id),
    observation_scope_id UUID NOT NULL REFERENCES observation_scopes(id),
    job_id UUID REFERENCES jobs(id),
    stage TEXT NOT NULL CHECK (stage IN ('reconcile', 'retry')),
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'error')),
    code TEXT NOT NULL CHECK (char_length(code) BETWEEN 1 AND 128),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    diagnostics JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT serving_jd_reconciliation_audits_diagnostics_small
        CHECK (octet_length(diagnostics::text) <= 8192)
);

CREATE INDEX serving_jd_reconciliation_audits_run_time_idx
    ON serving_jd_reconciliation_audits (published_run_id, occurred_at);
CREATE INDEX serving_jd_reconciliation_audits_retry_idx
    ON serving_jd_reconciliation_audits (observation_scope_id, job_id, occurred_at DESC)
    WHERE severity IN ('warning', 'error');

CREATE FUNCTION reject_serving_jd_inventory_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'serving JD inventory history is immutable';
END;
$$;

CREATE TRIGGER serving_jd_inventory_memberships_immutable
BEFORE UPDATE OR DELETE ON serving_jd_inventory_memberships
FOR EACH ROW EXECUTE FUNCTION reject_serving_jd_inventory_mutation();

CREATE TRIGGER serving_jd_dimension_resolutions_immutable
BEFORE UPDATE OR DELETE ON serving_jd_dimension_resolutions
FOR EACH ROW EXECUTE FUNCTION reject_serving_jd_inventory_mutation();

CREATE TRIGGER serving_jd_reconciliation_audits_immutable
BEFORE UPDATE OR DELETE ON serving_jd_reconciliation_audits
FOR EACH ROW EXECUTE FUNCTION reject_serving_jd_inventory_mutation();

COMMENT ON TABLE serving_jd_inventory_memberships IS
    'Append-only DWD snapshot members for each reconciled source publication; source and formal result identities remain directly traceable.';
COMMENT ON TABLE serving_jd_dimension_resolutions IS
    'Append-only selected formal-result references; later parsing appends a new resolution without mutating the source snapshot.';
COMMENT ON VIEW serving_jd_current_inventory_memberships IS
    'Latest published Serving membership for each observation scope; applications must not reconstruct it from ODS tables.';
