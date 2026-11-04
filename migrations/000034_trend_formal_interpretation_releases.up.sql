-- A trend release is a compact, immutable selection of the existing formal
-- deterministic identities. It deliberately contains no JD snapshot, source
-- payload, body text, result payload, or per-day aggregate.

CREATE TYPE trend_formal_release_status AS ENUM ('candidate', 'published', 'superseded');
CREATE TYPE trend_formal_release_dimension AS ENUM (
    'technology', 'education', 'experience', 'project', 'job_family'
);

CREATE TABLE trend_formal_releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_sha256 CHAR(64) NOT NULL CHECK (identity_sha256 ~ '^[0-9a-f]{64}$'),
    status trend_formal_release_status NOT NULL DEFAULT 'candidate',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    superseded_at TIMESTAMPTZ,
    CONSTRAINT trend_formal_releases_status_timestamps CHECK (
        (status = 'candidate' AND published_at IS NULL AND superseded_at IS NULL)
        OR (status = 'published' AND published_at IS NOT NULL AND superseded_at IS NULL)
        OR (status = 'superseded' AND published_at IS NOT NULL AND superseded_at IS NOT NULL)
    ),
    UNIQUE (identity_sha256)
);

CREATE UNIQUE INDEX trend_formal_releases_one_published_idx
    ON trend_formal_releases ((status = 'published'))
    WHERE status = 'published';

-- One row per formal dimension makes the exact interpretation selection
-- inspectable and foreign-keys the technology release without flattening all
-- deterministic identities into a mutable JSON document.
CREATE TABLE trend_formal_release_components (
    trend_formal_release_id UUID NOT NULL REFERENCES trend_formal_releases(id),
    dimension trend_formal_release_dimension NOT NULL,
    technology_analysis_configuration_id UUID REFERENCES technology_analysis_configurations(id),
    parser_version TEXT CHECK (parser_version IS NULL OR char_length(parser_version) BETWEEN 1 AND 128),
    lexicon_version TEXT CHECK (lexicon_version IS NULL OR char_length(lexicon_version) BETWEEN 1 AND 128),
    lexicon_sha256 CHAR(64) CHECK (lexicon_sha256 IS NULL OR lexicon_sha256 ~ '^[0-9a-f]{64}$'),
    dictionary_version TEXT CHECK (dictionary_version IS NULL OR char_length(dictionary_version) BETWEEN 1 AND 128),
    dictionary_sha256 CHAR(64) CHECK (dictionary_sha256 IS NULL OR dictionary_sha256 ~ '^[0-9a-f]{64}$'),
    project_mask_parser_version TEXT CHECK (
        project_mask_parser_version IS NULL OR char_length(project_mask_parser_version) BETWEEN 1 AND 128
    ),
    project_mask_dictionary_version TEXT CHECK (
        project_mask_dictionary_version IS NULL OR char_length(project_mask_dictionary_version) BETWEEN 1 AND 128
    ),
    project_mask_dictionary_sha256 CHAR(64) CHECK (
        project_mask_dictionary_sha256 IS NULL OR project_mask_dictionary_sha256 ~ '^[0-9a-f]{64}$'
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (trend_formal_release_id, dimension),
    CONSTRAINT trend_formal_release_components_typed_identity CHECK (
        (dimension = 'technology'
            AND technology_analysis_configuration_id IS NOT NULL
            AND parser_version IS NULL
            AND lexicon_version IS NULL
            AND lexicon_sha256 IS NULL
            AND dictionary_version IS NULL
            AND dictionary_sha256 IS NULL
            AND project_mask_parser_version IS NULL
            AND project_mask_dictionary_version IS NULL
            AND project_mask_dictionary_sha256 IS NULL)
        OR (dimension IN ('education', 'experience')
            AND technology_analysis_configuration_id IS NULL
            AND parser_version IS NOT NULL
            AND lexicon_version IS NOT NULL
            AND lexicon_sha256 IS NOT NULL
            AND dictionary_version IS NULL
            AND dictionary_sha256 IS NULL
            AND project_mask_parser_version IS NULL
            AND project_mask_dictionary_version IS NULL
            AND project_mask_dictionary_sha256 IS NULL)
        OR (dimension = 'project'
            AND technology_analysis_configuration_id IS NULL
            AND parser_version IS NOT NULL
            AND lexicon_version IS NULL
            AND lexicon_sha256 IS NULL
            AND dictionary_version IS NOT NULL
            AND dictionary_sha256 IS NOT NULL
            AND project_mask_parser_version IS NULL
            AND project_mask_dictionary_version IS NULL
            AND project_mask_dictionary_sha256 IS NULL)
        OR (dimension = 'job_family'
            AND technology_analysis_configuration_id IS NULL
            AND parser_version IS NOT NULL
            AND lexicon_version IS NULL
            AND lexicon_sha256 IS NULL
            AND dictionary_version IS NOT NULL
            AND dictionary_sha256 IS NOT NULL
            AND project_mask_parser_version IS NOT NULL
            AND project_mask_dictionary_version IS NOT NULL
            AND project_mask_dictionary_sha256 IS NOT NULL)
    )
);

CREATE INDEX trend_formal_release_components_technology_idx
    ON trend_formal_release_components (technology_analysis_configuration_id)
    WHERE technology_analysis_configuration_id IS NOT NULL;

CREATE FUNCTION reject_trend_formal_release_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    component_count INTEGER;
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'trend formal releases are immutable';
    END IF;

    IF OLD.id <> NEW.id
       OR OLD.identity_sha256 <> NEW.identity_sha256
       OR OLD.created_at <> NEW.created_at THEN
        RAISE EXCEPTION 'trend formal releases are immutable';
    END IF;

    IF OLD.status = 'candidate'
       AND NEW.status = 'published'
       AND NEW.published_at IS NOT NULL
       AND NEW.superseded_at IS NULL THEN
        SELECT count(*) INTO component_count
        FROM trend_formal_release_components
        WHERE trend_formal_release_id = OLD.id;
        IF component_count <> 5 THEN
            RAISE EXCEPTION 'incomplete trend formal release cannot be published';
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.status = 'candidate'
       AND NEW.status = 'superseded'
       AND NEW.published_at IS NOT NULL
       AND NEW.superseded_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    IF OLD.status = 'published'
       AND NEW.status = 'superseded'
       AND NEW.published_at = OLD.published_at
       AND NEW.superseded_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'trend formal release status transition is invalid';
END;
$$;

CREATE TRIGGER trend_formal_releases_immutable
BEFORE UPDATE OR DELETE ON trend_formal_releases
FOR EACH ROW EXECUTE FUNCTION reject_trend_formal_release_mutation();

CREATE FUNCTION reject_trend_formal_release_component_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'trend formal release components are immutable';
    END IF;

    RAISE EXCEPTION 'trend formal release components are immutable';
END;
$$;

CREATE TRIGGER trend_formal_release_components_immutable
BEFORE UPDATE OR DELETE ON trend_formal_release_components
FOR EACH ROW EXECUTE FUNCTION reject_trend_formal_release_component_mutation();

COMMENT ON TABLE trend_formal_releases IS
    'Append-only complete global selection of deterministic interpretation identities for historical trend reads.';
COMMENT ON TABLE trend_formal_release_components IS
    'Immutable typed identities selected together by one trend formal release; never stores JD snapshots or result payloads.';
