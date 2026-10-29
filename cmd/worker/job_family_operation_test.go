package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRunPendingAnalysisProcessesJobFamiliesWithoutPublishedTechnologyConfiguration(t *testing.T) {
	if os.Getenv("GO_WANT_JOB_FAMILY_ANALYSIS_HELPER") == "1" {
		runPendingAnalysis(nil)
		return
	}
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `
	TRUNCATE job_family_observation_retry_audits, job_family_retry_audits, job_family_match_evidence, job_family_matches,
         job_family_results, job_family_observations, job_family_title_identities,
         project_tag_evidence, project_tag_assignments, project_tag_results,
         project_title_observations, project_title_identities,
         technology_analysis_configurations CASCADE;
TRUNCATE jobs CASCADE`); err != nil {
		t.Fatal(err)
	}
	const (
		jobID      = "00000000-0000-4000-8000-000000000921"
		metadataID = "00000000-0000-4000-8000-000000000922"
	)
	if _, err := pool.Exec(context.Background(), `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'test', 'worker-job-family')`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
INSERT INTO job_source_metadata_versions (
    id, job_id, metadata_sha256, normalization_version, metadata_payload
) VALUES ($1, $2, $3, 'source-metadata-v1',
          '{"title":{"state":"string","value":"服务端运维"},"official_category":{"id":{"state":"string","value":"1"}}}'::jsonb)`,
		metadataID, jobID, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestRunPendingAnalysisProcessesJobFamiliesWithoutPublishedTechnologyConfiguration$")
	command.Env = append(os.Environ(),
		"GO_WANT_JOB_FAMILY_ANALYSIS_HELPER=1",
		"DATABASE_URL="+databaseURL,
		"SYNC_TIMEOUT=10s",
	)
	if err := command.Run(); err == nil {
		t.Fatal("analysis unexpectedly succeeded without a published technology configuration")
	}

	var primary string
	if err := pool.QueryRow(context.Background(), `
SELECT result.primary_family_key
FROM job_family_results AS result
JOIN job_family_observations AS observation ON observation.job_family_title_identity_id = result.job_family_title_identity_id
WHERE observation.source_metadata_version_id = $1`, metadataID).Scan(&primary); err != nil {
		t.Fatal(err)
	}
	if primary != "operations_sre" {
		t.Fatalf("worker job family = %q, want operations_sre", primary)
	}
}

func TestRunPendingAnalysisContinuesDeterministicStagesAfterOneJobFamilyFailure(t *testing.T) {
	if os.Getenv("GO_WANT_JOB_FAMILY_FAILURE_ANALYSIS_HELPER") == "1" {
		runPendingAnalysis(nil)
		return
	}
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `
	TRUNCATE job_family_observation_retry_audits, job_family_retry_audits, job_family_match_evidence, job_family_matches,
         job_family_results, job_family_observations, job_family_title_identities,
         project_tag_evidence, project_tag_assignments, project_tag_results,
         project_title_observations, project_title_identities,
         technology_analysis_configurations CASCADE;
TRUNCATE jobs CASCADE`); err != nil {
		t.Fatal(err)
	}
	const (
		jobID      = "00000000-0000-4000-8000-000000000931"
		metadataID = "00000000-0000-4000-8000-000000000932"
		bodyID     = "00000000-0000-4000-8000-000000000933"
	)
	if _, err := pool.Exec(context.Background(), `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'test', 'worker-job-family-failure')`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
INSERT INTO job_source_metadata_versions (
    id, job_id, metadata_sha256, normalization_version, metadata_payload
) VALUES ($2, $1, $3, 'source-metadata-v1',
          '{"title":{"state":"string","value":"后端工程师"},"official_category":{"id":{"state":"string","value":"1"}}}'::jsonb)`,
		jobID, metadataID, strings.Repeat("e", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($2, $1, $3, 'jd-body-v1', '本科及以上', '', '', $4, $5, $5)`,
		jobID, bodyID, strings.Repeat("d", 64), strings.Repeat("c", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
CREATE FUNCTION fail_worker_job_family_test_result()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'synthetic job-family storage failure';
END;
$$;
CREATE TRIGGER fail_worker_job_family_test_result_trigger
BEFORE INSERT ON job_family_results
FOR EACH ROW EXECUTE FUNCTION fail_worker_job_family_test_result();`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `
DROP TRIGGER IF EXISTS fail_worker_job_family_test_result_trigger ON job_family_results;
DROP FUNCTION IF EXISTS fail_worker_job_family_test_result();`); err != nil {
			t.Error(err)
		}
	})

	command := exec.Command(os.Args[0], "-test.run=^TestRunPendingAnalysisContinuesDeterministicStagesAfterOneJobFamilyFailure$")
	command.Env = append(os.Environ(),
		"GO_WANT_JOB_FAMILY_FAILURE_ANALYSIS_HELPER=1",
		"DATABASE_URL="+databaseURL,
		"SYNC_TIMEOUT=10s",
	)
	if err := command.Run(); err == nil {
		t.Fatal("analysis unexpectedly succeeded despite the injected job-family failure and absent technology configuration")
	}

	var retryAuditCount, educationResultCount int
	if err := pool.QueryRow(context.Background(), `
SELECT
  (SELECT count(*) FROM job_family_retry_audits),
  (SELECT count(*) FROM education_requirement_results)`).Scan(&retryAuditCount, &educationResultCount); err != nil {
		t.Fatal(err)
	}
	if retryAuditCount != 1 || educationResultCount != 1 {
		t.Fatalf("worker deterministic-stage continuation = job_family_audits:%d education_results:%d, want 1:1", retryAuditCount, educationResultCount)
	}
}
