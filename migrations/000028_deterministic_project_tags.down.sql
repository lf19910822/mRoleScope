DROP TRIGGER IF EXISTS project_tag_evidence_immutable ON project_tag_evidence;
DROP TRIGGER IF EXISTS project_tag_assignments_immutable ON project_tag_assignments;
DROP TRIGGER IF EXISTS project_tag_results_immutable ON project_tag_results;
DROP TRIGGER IF EXISTS project_title_observations_immutable ON project_title_observations;
DROP TRIGGER IF EXISTS project_title_identities_immutable ON project_title_identities;
DROP FUNCTION IF EXISTS reject_project_tag_mutation();

DROP TABLE IF EXISTS project_tag_evidence;
DROP TABLE IF EXISTS project_tag_assignments;
DROP TABLE IF EXISTS project_tag_results;
DROP TABLE IF EXISTS project_title_observations;
DROP TABLE IF EXISTS project_title_identities;

DROP TYPE IF EXISTS project_tag_classification;
DROP TYPE IF EXISTS project_tag_result_status;
