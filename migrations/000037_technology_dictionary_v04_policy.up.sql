-- v0.4 freezes not only aliases but also explicit exclusions and emitted
-- identities. Existing v0.1-v0.3 rows keep their historical semantics.

ALTER TABLE technology_dictionary_versions
    ADD COLUMN matcher_policy_version TEXT NOT NULL DEFAULT 'technology-mentions-v1'
        CHECK (char_length(matcher_policy_version) BETWEEN 1 AND 128);

CREATE TABLE technology_dictionary_rule_emissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_rule_id UUID NOT NULL REFERENCES technology_dictionary_rules(id),
    emitted_technology_identity_id UUID NOT NULL REFERENCES technology_identities(id),
    relation_type TEXT NOT NULL CHECK (relation_type IN ('self', 'inherited')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_rule_id, emitted_technology_identity_id)
);

CREATE INDEX technology_dictionary_rule_emissions_source_idx
    ON technology_dictionary_rule_emissions (source_rule_id, relation_type, id);

CREATE TABLE technology_dictionary_exclusion_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dictionary_version_id UUID NOT NULL REFERENCES technology_dictionary_versions(id),
    source_rule_id UUID NOT NULL REFERENCES technology_dictionary_rules(id),
    pattern_key TEXT NOT NULL CHECK (char_length(pattern_key) BETWEEN 1 AND 128),
    pattern TEXT NOT NULL CHECK (char_length(pattern) BETWEEN 1 AND 256),
    state technology_rule_state NOT NULL DEFAULT 'candidate',
    case_mode TEXT NOT NULL CHECK (case_mode IN ('sensitive', 'insensitive')),
    boundary_mode TEXT NOT NULL CHECK (boundary_mode IN ('exact', 'ascii_identifier', 'custom')),
    window_codepoints INTEGER NOT NULL CHECK (window_codepoints BETWEEN 1 AND 96),
    provenance TEXT NOT NULL CHECK (char_length(provenance) BETWEEN 1 AND 512),
    approved_by TEXT CHECK (char_length(approved_by) BETWEEN 1 AND 256),
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dictionary_version_id, pattern_key),
    CONSTRAINT technology_dictionary_exclusions_enabled_requires_approval CHECK (
        state <> 'enabled' OR (approved_by IS NOT NULL AND approved_at IS NOT NULL)
    )
);

CREATE INDEX technology_dictionary_exclusion_patterns_rule_idx
    ON technology_dictionary_exclusion_patterns (dictionary_version_id, source_rule_id, pattern_key)
    WHERE state = 'enabled';

CREATE TABLE technology_analysis_configuration_emissions (
    configuration_id UUID NOT NULL REFERENCES technology_analysis_configurations(id),
    dictionary_emission_id UUID NOT NULL REFERENCES technology_dictionary_rule_emissions(id),
    PRIMARY KEY (configuration_id, dictionary_emission_id)
);

CREATE TABLE technology_analysis_configuration_exclusions (
    configuration_id UUID NOT NULL REFERENCES technology_analysis_configurations(id),
    dictionary_exclusion_pattern_id UUID NOT NULL REFERENCES technology_dictionary_exclusion_patterns(id),
    PRIMARY KEY (configuration_id, dictionary_exclusion_pattern_id)
);

CREATE FUNCTION reject_frozen_technology_dictionary_emission_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_status technology_dictionary_status;
BEGIN
    SELECT version.status INTO target_status
    FROM technology_dictionary_rules AS source_rule
    JOIN technology_dictionary_versions AS version ON version.id = source_rule.dictionary_version_id
    WHERE source_rule.id = CASE WHEN TG_OP = 'DELETE' THEN OLD.source_rule_id ELSE NEW.source_rule_id END;
    IF target_status IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published or retired dictionary emissions are immutable';
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER technology_dictionary_rule_emissions_frozen_immutable
BEFORE INSERT OR UPDATE OR DELETE ON technology_dictionary_rule_emissions
FOR EACH ROW EXECUTE FUNCTION reject_frozen_technology_dictionary_emission_mutation();

CREATE FUNCTION reject_frozen_technology_dictionary_exclusion_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_status technology_dictionary_status;
    target_version_id UUID;
BEGIN
    target_version_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.dictionary_version_id ELSE NEW.dictionary_version_id END;
    SELECT status INTO target_status
    FROM technology_dictionary_versions
    WHERE id = target_version_id;
    IF target_status IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published or retired dictionary exclusions are immutable';
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER technology_dictionary_exclusion_patterns_frozen_immutable
BEFORE INSERT OR UPDATE OR DELETE ON technology_dictionary_exclusion_patterns
FOR EACH ROW EXECUTE FUNCTION reject_frozen_technology_dictionary_exclusion_mutation();

CREATE FUNCTION reject_technology_analysis_configuration_emission_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_configuration_id UUID;
    target_status technology_analysis_configuration_status;
BEGIN
    target_configuration_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.configuration_id ELSE NEW.configuration_id END;
    SELECT status INTO target_status
    FROM technology_analysis_configurations
    WHERE id = target_configuration_id;
    IF TG_OP = 'INSERT' AND target_status = 'candidate' THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'analysis configuration emission snapshots are immutable';
END;
$$;

CREATE TRIGGER technology_analysis_configuration_emissions_immutable
BEFORE INSERT OR UPDATE OR DELETE ON technology_analysis_configuration_emissions
FOR EACH ROW EXECUTE FUNCTION reject_technology_analysis_configuration_emission_mutation();

CREATE FUNCTION reject_technology_analysis_configuration_exclusion_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_configuration_id UUID;
    target_status technology_analysis_configuration_status;
BEGIN
    target_configuration_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.configuration_id ELSE NEW.configuration_id END;
    SELECT status INTO target_status
    FROM technology_analysis_configurations
    WHERE id = target_configuration_id;
    IF TG_OP = 'INSERT' AND target_status = 'candidate' THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'analysis configuration exclusion snapshots are immutable';
END;
$$;

CREATE TRIGGER technology_analysis_configuration_exclusions_immutable
BEFORE INSERT OR UPDATE OR DELETE ON technology_analysis_configuration_exclusions
FOR EACH ROW EXECUTE FUNCTION reject_technology_analysis_configuration_exclusion_mutation();

CREATE OR REPLACE FUNCTION reject_frozen_technology_dictionary_version_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status = 'published' AND NEW.status = 'retired'
       AND NEW.id = OLD.id
       AND NEW.version_label = OLD.version_label
       AND NEW.content_sha256 = OLD.content_sha256
       AND NEW.status = 'retired'
       AND NEW.source_document = OLD.source_document
       AND NEW.matcher_policy_version = OLD.matcher_policy_version
       AND NEW.created_by = OLD.created_by
       AND NEW.created_at = OLD.created_at
       AND NEW.published_at = OLD.published_at
       AND NEW.retired_at IS NOT NULL THEN
        RETURN NEW;
    END IF;
    IF OLD.status IN ('published', 'retired') THEN
        RAISE EXCEPTION 'published or retired dictionary versions are immutable';
    END IF;
    RETURN NEW;
END;
$$;
