DROP TABLE IF EXISTS ai_analysis_audit_events;
DROP TYPE IF EXISTS ai_audit_severity;
DROP TYPE IF EXISTS ai_audit_code;

ALTER TABLE ai_analysis_attempts DROP CONSTRAINT IF EXISTS ai_analysis_attempts_safe_error_code_allowed;
ALTER TABLE ai_analysis_work_items DROP CONSTRAINT IF EXISTS ai_analysis_work_items_last_safe_error_code_allowed;
ALTER TABLE ai_analysis_runs DROP CONSTRAINT IF EXISTS ai_analysis_runs_safe_error_code_allowed;
ALTER TABLE ai_semantic_results DROP CONSTRAINT IF EXISTS ai_semantic_results_safe_error_code_allowed;
ALTER TABLE ai_analysis_configurations DROP CONSTRAINT IF EXISTS ai_analysis_configurations_safe_failure_code_allowed;

DROP TYPE IF EXISTS ai_safe_error_code;
