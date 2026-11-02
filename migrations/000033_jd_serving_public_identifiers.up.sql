-- Stable, random public identifiers stay in the serving boundary. They make
-- a JD addressable by the browser without returning a source or provenance ID.

CREATE TABLE serving_jd_public_identifiers (
    job_id UUID PRIMARY KEY REFERENCES jobs(id),
    public_id UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX serving_jd_public_identifiers_public_id_idx
    ON serving_jd_public_identifiers (public_id);

CREATE TRIGGER serving_jd_public_identifiers_immutable
BEFORE UPDATE OR DELETE ON serving_jd_public_identifiers
FOR EACH ROW EXECUTE FUNCTION reject_serving_jd_inventory_mutation();

COMMENT ON TABLE serving_jd_public_identifiers IS
    'Stable opaque browser identifiers, isolated from source and provenance identifiers.';
