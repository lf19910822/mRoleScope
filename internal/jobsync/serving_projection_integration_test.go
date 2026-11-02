package jobsync

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Serving source seam map:
//
//	collection_runs / observation_scope_publication_state -> published run
//	job_presence_periods                               -> activity lifecycle
//	run_job_version_references                         -> observed metadata/body
//	education_requirement_results                      -> body formal result
//	experience_requirement_results                     -> body formal result
//	project_tag_results                                -> metadata formal result
//	job_family_results                                 -> metadata formal result
//
// The forthcoming DWD relation must retain these immutable source identities.
// Its population and current/fallback resolution are separate behaviors, so
// this test intentionally inserts one membership directly after the migration
// has created the relation.
func TestServingMembershipRetainsPublishedVersionAndFormalResultReferences(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 10, 30, 2, 0, 0, 0, time.UTC)
	run := publishFixtureRun(t, repository, scope, at, 1, []Observation{
		integrationObservationWithTitleAndBody("服务端工程师", "serving-v1", at),
	}, nil)

	var jobID, presencePeriodID, metadataID, bodyID string
	if err := repository.pool.QueryRow(ctx, `
SELECT reference.job_id::text,
       presence.id::text,
       reference.source_metadata_version_id::text,
       reference.body_content_version_id::text
FROM run_job_version_references AS reference
JOIN job_presence_periods AS presence
  ON presence.observation_scope_id = $2
 AND presence.job_id = reference.job_id
 AND presence.closed_at IS NULL
WHERE reference.run_id = $1`, run.ID, scope.ID).Scan(&jobID, &presencePeriodID, &metadataID, &bodyID); err != nil {
		t.Fatalf("load fixture version identities: %v", err)
	}

	educationID, experienceID, projectID, familyID := insertServingFormalFixtures(t, repository, bodyID, metadataID)

	var membershipID, storedRunID, storedScopeID, storedJobID, storedPresenceID, storedMetadataID, storedBodyID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO serving_jd_inventory_memberships (
    published_run_id,
    observation_scope_id,
    job_id,
    presence_period_id,
    source_metadata_version_id,
    body_content_version_id
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id::text,
          published_run_id::text,
          observation_scope_id::text,
          job_id::text,
          presence_period_id::text,
          source_metadata_version_id::text,
          body_content_version_id::text`,
		run.ID, scope.ID, jobID, presencePeriodID, metadataID, bodyID,
	).Scan(
		&membershipID, &storedRunID, &storedScopeID, &storedJobID, &storedPresenceID, &storedMetadataID, &storedBodyID,
	); err != nil {
		t.Fatalf("insert serving membership: %v", err)
	}

	got := []string{
		storedRunID, storedScopeID, storedJobID, storedPresenceID, storedMetadataID, storedBodyID,
	}
	want := []string{
		run.ID, scope.ID, jobID, presencePeriodID, metadataID, bodyID,
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("serving membership identity[%d] = %q, want %q", index, got[index], want[index])
		}
	}

	assertServingDimensionResolution(t, repository, membershipID, "education", educationID)
	assertServingDimensionResolution(t, repository, membershipID, "experience", experienceID)
	assertServingDimensionResolution(t, repository, membershipID, "project", projectID)
	assertServingDimensionResolution(t, repository, membershipID, "job_family", familyID)
}

func assertServingDimensionResolution(t *testing.T, repository *Repository, membershipID, dimension, resultID string) {
	t.Helper()
	ctx := context.Background()
	queries := map[string]string{
		"education": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    education_requirement_result_id
) VALUES ($1, 'education', 0, $2, $3)
RETURNING education_requirement_result_id::text`,
		"experience": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    experience_requirement_result_id
) VALUES ($1, 'experience', 0, $2, $3)
RETURNING experience_requirement_result_id::text`,
		"project": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    project_tag_result_id
) VALUES ($1, 'project', 0, $2, $3)
RETURNING project_tag_result_id::text`,
		"job_family": `INSERT INTO serving_jd_dimension_resolutions (
    serving_jd_inventory_membership_id, dimension, selection_priority, resolution_key,
    job_family_result_id
) VALUES ($1, 'job_family', 0, $2, $3)
RETURNING job_family_result_id::text`,
	}
	query, exists := queries[dimension]
	if !exists {
		t.Fatalf("unsupported serving resolution dimension %q", dimension)
	}
	var storedResultID string
	if err := repository.pool.QueryRow(ctx, query, membershipID, dimension+":"+resultID, resultID).Scan(&storedResultID); err != nil {
		t.Fatalf("insert %s serving resolution: %v", dimension, err)
	}
	if storedResultID != resultID {
		t.Fatalf("%s serving result = %q, want %q", dimension, storedResultID, resultID)
	}
}

func insertServingFormalFixtures(t *testing.T, repository *Repository, bodyID, metadataID string) (educationID, experienceID, projectID, familyID string) {
	t.Helper()
	ctx := context.Background()

	if err := repository.pool.QueryRow(ctx, `
INSERT INTO education_requirement_results (
    body_content_version_id, parser_version, lexicon_version, lexicon_sha256,
    status, lower_bound_level
) VALUES ($1, 'serving-test-education', 'serving-test-lexicon', $2, 'configured', 'bachelor')
RETURNING id::text`, bodyID, strings.Repeat("a", 64)).Scan(&educationID); err != nil {
		t.Fatalf("insert deterministic education fixture: %v", err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO experience_requirement_results (
    body_content_version_id, parser_version, lexicon_version, lexicon_sha256,
    status, base_year
) VALUES ($1, 'serving-test-experience', 'serving-test-lexicon', $2, 'configured', 3)
RETURNING id::text`, bodyID, strings.Repeat("b", 64)).Scan(&experienceID); err != nil {
		t.Fatalf("insert deterministic experience fixture: %v", err)
	}

	var projectTitleID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO project_title_identities (title_sha256, normalization_version, normalized_title)
VALUES (encode(digest('serving-project-' || $1, 'sha256'), 'hex'), 'serving-test-normalization', '服务端工程师')
RETURNING id::text`, metadataID).Scan(&projectTitleID); err != nil {
		t.Fatalf("insert deterministic project title fixture: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO project_title_observations (source_metadata_version_id, project_title_identity_id)
VALUES ($1, $2)`, metadataID, projectTitleID); err != nil {
		t.Fatalf("insert deterministic project observation fixture: %v", err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO project_tag_results (
    project_title_identity_id, parser_version, dictionary_version, dictionary_sha256, status
) VALUES ($1, 'serving-test-project', 'serving-test-dictionary', $2, 'complete')
RETURNING id::text`, projectTitleID, strings.Repeat("d", 64)).Scan(&projectID); err != nil {
		t.Fatalf("insert deterministic project fixture: %v", err)
	}

	var familyTitleID string
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO job_family_title_identities (title_sha256, normalization_version, normalized_title)
VALUES (encode(digest('serving-family-' || $1, 'sha256'), 'hex'), 'serving-test-normalization', '服务端工程师')
RETURNING id::text`, metadataID).Scan(&familyTitleID); err != nil {
		t.Fatalf("insert deterministic family title fixture: %v", err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_family_observations (
    source_metadata_version_id, job_family_title_identity_id, status
) VALUES ($1, $2, 'in_scope')`, metadataID, familyTitleID); err != nil {
		t.Fatalf("insert deterministic family observation fixture: %v", err)
	}
	if err := repository.pool.QueryRow(ctx, `
INSERT INTO job_family_results (
    job_family_title_identity_id,
    parser_version, dictionary_version, dictionary_sha256,
    project_mask_parser_version, project_mask_dictionary_version, project_mask_dictionary_sha256,
    status, primary_family_key
) VALUES ($1, 'serving-test-family', 'serving-test-dictionary', $2,
          'serving-test-project-mask', 'serving-test-project-dictionary', $3,
          'classified', 'backend')
RETURNING id::text`, familyTitleID, strings.Repeat("f", 64), strings.Repeat("0", 64)).Scan(&familyID); err != nil {
		t.Fatalf("insert deterministic job-family fixture: %v", err)
	}

	return educationID, experienceID, projectID, familyID
}
