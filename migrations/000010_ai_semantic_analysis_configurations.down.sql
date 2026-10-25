DROP TRIGGER IF EXISTS ai_analysis_configurations_immutable ON ai_analysis_configurations;
DROP FUNCTION IF EXISTS reject_ai_analysis_configuration_mutation();

DROP TRIGGER IF EXISTS ai_analysis_configurations_require_published_technology_configuration
    ON ai_analysis_configurations;
DROP FUNCTION IF EXISTS require_published_ai_technology_configuration();

DROP TABLE IF EXISTS ai_analysis_configurations;
DROP TYPE IF EXISTS ai_analysis_configuration_status;
DROP TYPE IF EXISTS ai_analysis_provider;
