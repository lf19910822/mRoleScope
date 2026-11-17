DROP TRIGGER IF EXISTS technology_analysis_configuration_exclusions_immutable ON technology_analysis_configuration_exclusions;
DROP TRIGGER IF EXISTS technology_analysis_configuration_emissions_immutable ON technology_analysis_configuration_emissions;
DROP TRIGGER IF EXISTS technology_dictionary_exclusion_patterns_frozen_immutable ON technology_dictionary_exclusion_patterns;
DROP TRIGGER IF EXISTS technology_dictionary_rule_emissions_frozen_immutable ON technology_dictionary_rule_emissions;

DROP FUNCTION IF EXISTS reject_technology_analysis_configuration_exclusion_mutation();
DROP FUNCTION IF EXISTS reject_technology_analysis_configuration_emission_mutation();
DROP FUNCTION IF EXISTS reject_frozen_technology_dictionary_exclusion_mutation();
DROP FUNCTION IF EXISTS reject_frozen_technology_dictionary_emission_mutation();

DROP TABLE IF EXISTS technology_analysis_configuration_exclusions;
DROP TABLE IF EXISTS technology_analysis_configuration_emissions;
DROP TABLE IF EXISTS technology_dictionary_exclusion_patterns;
DROP TABLE IF EXISTS technology_dictionary_rule_emissions;

CREATE OR REPLACE FUNCTION reject_frozen_technology_dictionary_version_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status = 'published' AND NEW.status = 'retired'
       AND NEW.id = OLD.id
       AND NEW.version_label = OLD.version_label
       AND NEW.content_sha256 = OLD.content_sha256
       AND NEW.source_document = OLD.source_document
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

ALTER TABLE technology_dictionary_versions
    DROP COLUMN IF EXISTS matcher_policy_version;
