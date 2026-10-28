package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRunPendingAnalysisProcessesExperienceWithoutPublishedTechnologyConfiguration(t *testing.T) {
	if os.Getenv("GO_WANT_EXPERIENCE_ANALYSIS_HELPER") == "1" {
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
		jobID  = "00000000-0000-4000-8000-000000000911"
		bodyID = "00000000-0000-4000-8000-000000000912"
	)
	if _, err := pool.Exec(context.Background(), `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'test', 'worker-experience')`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, 'jd-body-v1', '', '3年以上工作经验', '', $3, $4, $3)`,
		bodyID, jobID, strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestRunPendingAnalysisProcessesExperienceWithoutPublishedTechnologyConfiguration$")
	command.Env = append(os.Environ(),
		"GO_WANT_EXPERIENCE_ANALYSIS_HELPER=1",
		"DATABASE_URL="+databaseURL,
		"SYNC_TIMEOUT=10s",
	)
	if err := command.Run(); err == nil {
		t.Fatal("analysis unexpectedly succeeded without a published technology configuration")
	}

	var baseYear int
	if err := pool.QueryRow(context.Background(), `
SELECT base_year
FROM experience_requirement_results
WHERE body_content_version_id = $1`, bodyID).Scan(&baseYear); err != nil {
		t.Fatal(err)
	}
	if baseYear != 3 {
		t.Fatalf("worker deterministic experience base year = %d, want 3", baseYear)
	}
}
