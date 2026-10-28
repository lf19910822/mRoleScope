DROP TRIGGER IF EXISTS experience_requirement_evidence_immutable ON experience_requirement_evidence;
DROP TRIGGER IF EXISTS experience_requirement_results_immutable ON experience_requirement_results;
DROP FUNCTION IF EXISTS reject_experience_requirement_mutation();

DROP TABLE IF EXISTS experience_requirement_evidence;
DROP TABLE IF EXISTS experience_requirement_results;

DROP TYPE IF EXISTS experience_requirement_modality;
DROP TYPE IF EXISTS experience_requirement_result_status;
