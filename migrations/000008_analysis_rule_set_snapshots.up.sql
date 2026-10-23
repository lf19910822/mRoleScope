-- A configuration freezes the exact approved rules it may execute.  The
-- candidate dictionary can receive later approval decisions, so parser and
-- dictionary version alone are not enough to identify a reproducible result.

ALTER TABLE technology_analysis_configurations
    ADD COLUMN rule_set_sha256 CHAR(64) NOT NULL DEFAULT '06d9f50dee091d979f035ae91b601d5633c15bcba1bedfdae4ad1e44e52a3567'
        CHECK (rule_set_sha256 ~ '^[0-9a-f]{64}$');

ALTER TABLE technology_analysis_configurations
    DROP CONSTRAINT technology_analysis_configura_parser_version_dictionary_ver_key,
    ADD CONSTRAINT technology_analysis_configurations_rule_set_unique
        UNIQUE (parser_version, dictionary_version_id, rule_set_sha256);

ALTER TABLE technology_analysis_configurations
    ALTER COLUMN rule_set_sha256 DROP DEFAULT;

CREATE TABLE technology_analysis_configuration_rules (
    configuration_id UUID NOT NULL REFERENCES technology_analysis_configurations(id),
    dictionary_rule_id UUID NOT NULL REFERENCES technology_dictionary_rules(id),
    PRIMARY KEY (configuration_id, dictionary_rule_id)
);

-- Existing configurations predate rule-set snapshots.  Preserve their current
-- executable membership for audit; their sentinel fingerprint deliberately
-- ensures the next preparation creates a fresh complete snapshot.
INSERT INTO technology_analysis_configuration_rules (configuration_id, dictionary_rule_id)
SELECT configuration.id, rule.id
FROM technology_analysis_configurations AS configuration
JOIN technology_dictionary_rules AS rule
  ON rule.dictionary_version_id = configuration.dictionary_version_id
WHERE rule.state = 'enabled'
ON CONFLICT (configuration_id, dictionary_rule_id) DO NOTHING;

ALTER TABLE technology_analysis_results
    ADD COLUMN configuration_id UUID;

UPDATE technology_analysis_results AS result
SET configuration_id = configuration.id
FROM technology_analysis_configurations AS configuration
WHERE configuration.parser_version = result.parser_version
  AND configuration.dictionary_version_id = result.dictionary_version_id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM technology_analysis_results
        WHERE configuration_id IS NULL
    ) THEN
        RAISE EXCEPTION 'analysis result has no matching analysis configuration';
    END IF;
END $$;

ALTER TABLE technology_analysis_results
    ALTER COLUMN configuration_id SET NOT NULL,
    ADD CONSTRAINT technology_analysis_results_configuration_id_fkey
        FOREIGN KEY (configuration_id) REFERENCES technology_analysis_configurations(id),
    DROP CONSTRAINT technology_analysis_results_body_content_version_id_parser__key,
    ADD CONSTRAINT technology_analysis_results_configuration_unique
        UNIQUE (body_content_version_id, configuration_id);

CREATE INDEX technology_analysis_results_configuration_id_status_idx
    ON technology_analysis_results (configuration_id, status);
