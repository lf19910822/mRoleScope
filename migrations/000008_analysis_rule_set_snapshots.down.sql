DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM technology_analysis_configurations
        GROUP BY parser_version, dictionary_version_id
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot remove analysis rule-set snapshots after multiple snapshots exist';
    END IF;
END $$;

DROP INDEX IF EXISTS technology_analysis_results_configuration_id_status_idx;

ALTER TABLE technology_analysis_results
    DROP CONSTRAINT technology_analysis_results_configuration_unique,
    DROP CONSTRAINT technology_analysis_results_configuration_id_fkey,
    ADD CONSTRAINT technology_analysis_results_body_content_version_id_parser__key
        UNIQUE (body_content_version_id, parser_version, dictionary_version_id),
    DROP COLUMN configuration_id;

DROP TABLE IF EXISTS technology_analysis_configuration_rules;

ALTER TABLE technology_analysis_configurations
    DROP CONSTRAINT technology_analysis_configurations_rule_set_unique,
    ADD CONSTRAINT technology_analysis_configura_parser_version_dictionary_ver_key
        UNIQUE (parser_version, dictionary_version_id),
    DROP COLUMN rule_set_sha256;
