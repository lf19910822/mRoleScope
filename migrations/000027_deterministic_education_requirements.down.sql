DROP TRIGGER IF EXISTS education_requirement_evidence_immutable ON education_requirement_evidence;
DROP TRIGGER IF EXISTS education_requirement_results_immutable ON education_requirement_results;
DROP FUNCTION IF EXISTS reject_education_requirement_mutation();

DROP TABLE IF EXISTS education_requirement_evidence;
DROP TABLE IF EXISTS education_requirement_results;

DROP TYPE IF EXISTS education_requirement_modality;
DROP TYPE IF EXISTS deterministic_education_level;
DROP TYPE IF EXISTS education_requirement_result_status;
