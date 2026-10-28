package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRunPendingAnalysisProcessesEducationWithoutPublishedTechnologyConfiguration(t *testing.T) {
	if os.Getenv("GO_WANT_EDUCATION_ANALYSIS_HELPER") == "1" {
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
TRUNCATE project_tag_evidence, project_tag_assignments, project_tag_results,
         project_title_observations, project_title_identities,
         technology_analysis_configurations CASCADE;
TRUNCATE jobs CASCADE`); err != nil {
		t.Fatal(err)
	}
	const (
		jobID  = "00000000-0000-4000-8000-000000000901"
		bodyID = "00000000-0000-4000-8000-000000000902"
	)
	if _, err := pool.Exec(context.Background(), `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'test', 'worker-education')`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, 'jd-body-v1', '', '本科及以上', '', $3, $4, $3)`,
		bodyID, jobID, strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestRunPendingAnalysisProcessesEducationWithoutPublishedTechnologyConfiguration$")
	command.Env = append(os.Environ(),
		"GO_WANT_EDUCATION_ANALYSIS_HELPER=1",
		"DATABASE_URL="+databaseURL,
		"SYNC_TIMEOUT=10s",
	)
	if err := command.Run(); err == nil {
		t.Fatal("analysis unexpectedly succeeded without a published technology configuration")
	}

	var lowerBound string
	if err := pool.QueryRow(context.Background(), `
SELECT lower_bound_level::text
FROM education_requirement_results
WHERE body_content_version_id = $1`, bodyID).Scan(&lowerBound); err != nil {
		t.Fatal(err)
	}
	if lowerBound != "bachelor" {
		t.Fatalf("worker deterministic education lower bound = %q, want bachelor", lowerBound)
	}
}

func TestRunPendingAnalysisProcessesProjectTagsWithoutPublishedTechnologyConfiguration(t *testing.T) {
	if os.Getenv("GO_WANT_PROJECT_TAG_ANALYSIS_HELPER") == "1" {
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
TRUNCATE project_tag_evidence, project_tag_assignments, project_tag_results,
         project_title_observations, project_title_identities,
         technology_analysis_configurations CASCADE;
TRUNCATE jobs CASCADE`); err != nil {
		t.Fatal(err)
	}
	const (
		jobID      = "00000000-0000-4000-8000-000000000a51"
		metadataID = "00000000-0000-4000-8000-000000000a52"
	)
	if _, err := pool.Exec(context.Background(), `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'test', 'worker-project-tags')`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
INSERT INTO job_source_metadata_versions (
    id, job_id, metadata_sha256, normalization_version, metadata_payload
) VALUES ($1, $2, $3, 'source-metadata-v1',
          '{"title":{"state":"string","value":"绝区零-客户端"}}'::jsonb)`,
		metadataID, jobID, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestRunPendingAnalysisProcessesProjectTagsWithoutPublishedTechnologyConfiguration$")
	command.Env = append(os.Environ(),
		"GO_WANT_PROJECT_TAG_ANALYSIS_HELPER=1",
		"DATABASE_URL="+databaseURL,
		"SYNC_TIMEOUT=10s",
	)
	if err := command.Run(); err == nil {
		t.Fatal("analysis unexpectedly succeeded without a published technology configuration")
	}

	var projectKey string
	if err := pool.QueryRow(context.Background(), `
SELECT assignment.project_key
FROM project_tag_results AS result
JOIN project_title_observations AS observation ON observation.project_title_identity_id = result.project_title_identity_id
JOIN project_tag_assignments AS assignment ON assignment.project_tag_result_id = result.id
WHERE observation.source_metadata_version_id = $1`, metadataID).Scan(&projectKey); err != nil {
		t.Fatal(err)
	}
	if projectKey != "game.zenless-zone-zero" {
		t.Fatalf("worker project tag = %q, want game.zenless-zone-zero", projectKey)
	}
}
