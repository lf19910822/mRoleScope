DROP TRIGGER IF EXISTS job_family_observation_retry_audits_immutable ON job_family_observation_retry_audits;
DROP TRIGGER IF EXISTS job_family_retry_audits_immutable ON job_family_retry_audits;
DROP TRIGGER IF EXISTS job_family_match_evidence_immutable ON job_family_match_evidence;
DROP TRIGGER IF EXISTS job_family_matches_immutable ON job_family_matches;
DROP TRIGGER IF EXISTS job_family_results_immutable ON job_family_results;
DROP TRIGGER IF EXISTS job_family_observations_immutable ON job_family_observations;
DROP TRIGGER IF EXISTS job_family_title_identities_immutable ON job_family_title_identities;
DROP FUNCTION IF EXISTS reject_job_family_mutation();

DROP TABLE IF EXISTS job_family_observation_retry_audits;
DROP TABLE IF EXISTS job_family_retry_audits;
DROP TABLE IF EXISTS job_family_match_evidence;
DROP TABLE IF EXISTS job_family_matches;
DROP TABLE IF EXISTS job_family_results;
DROP TABLE IF EXISTS job_family_observations;
DROP TABLE IF EXISTS job_family_title_identities;

DROP TYPE IF EXISTS job_family_result_status;
DROP TYPE IF EXISTS job_family_observation_status;
