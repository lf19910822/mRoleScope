package jdanalysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryPublishesReusableResultsAndRejectsInvalidEvidence(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	bodyID, dictionaryID, technologyID, ruleID := insertSyntheticAnalysisPrerequisites(t, repository)
	configurationID := prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-parser-v1")
	identity := ResultIdentity{BodyContentVersionID: bodyID, ConfigurationID: configurationID}
	mentions := []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence: []EvidenceInput{
			{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 0, End: 2},
			{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 4, End: 6},
		},
	}}
	first, err := repository.PublishCompleteResult(ctx, identity, mentions)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused || first.ResultID == "" {
		t.Fatalf("first publication = %#v", first)
	}
	second, err := repository.PublishCompleteResult(ctx, identity, mentions)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Reused || second.ResultID != first.ResultID {
		t.Fatalf("second publication was not reused: first=%#v second=%#v", first, second)
	}
	var results, mentionsCount, evidenceCount int
	var status string
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM technology_analysis_results),
  (SELECT count(*) FROM technology_mentions),
  (SELECT count(*) FROM technology_mention_evidence),
  (SELECT status::text FROM technology_analysis_results WHERE id = $1)`, first.ResultID).Scan(&results, &mentionsCount, &evidenceCount, &status); err != nil {
		t.Fatal(err)
	}
	if results != 1 || mentionsCount != 1 || evidenceCount != 2 || status != "complete" {
		t.Fatalf("published result counts = results:%d mentions:%d evidence:%d status:%s", results, mentionsCount, evidenceCount, status)
	}

	badIdentity := ResultIdentity{
		BodyContentVersionID: bodyID,
		ConfigurationID:      prepareSyntheticAnalysisConfiguration(t, repository, dictionaryID, "synthetic-parser-invalid"),
	}
	_, err = repository.PublishCompleteResult(ctx, badIdentity, []MentionInput{{
		TechnologyIdentityID: technologyID,
		Evidence:             []EvidenceInput{{SourceField: SourceFieldDescription, RuleID: ruleID, Start: 2, End: 3}},
	}})
	if err == nil {
		t.Fatal("invalid evidence was accepted")
	}
	if strings.Contains(err.Error(), "Go, Go") {
		t.Fatal("invalid evidence error leaked normalized body text")
	}
	var invalidResults int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM technology_analysis_results WHERE parser_version = 'synthetic-parser-invalid'`).Scan(&invalidResults); err != nil {
		t.Fatal(err)
	}
	if invalidResults != 0 {
		t.Fatalf("invalid evidence left a publishable result: %d", invalidResults)
	}
}

func newIntegrationRepository(t *testing.T) *Repository {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	repository, err := NewRepository(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(repository.Close)
	acquireIntegrationTestLock(t, repository.pool)
	if _, err := repository.pool.Exec(context.Background(), `
		TRUNCATE job_family_observation_retry_audits, job_family_retry_audits, job_family_match_evidence, job_family_matches,
	         job_family_results, job_family_observations, job_family_title_identities,
	         project_tag_evidence, project_tag_assignments, project_tag_results,
         project_title_observations, project_title_identities,
         technology_mention_evidence, technology_mentions, technology_analysis_results,
         technology_analysis_configuration_rules,
         technology_dictionary_candidates, technology_dictionary_rule_decisions,
         technology_dictionary_rules, technology_dictionary_versions,
         technology_identities, job_body_content_periods, run_job_version_references,
         job_body_content_versions, job_source_metadata_versions, job_content_periods,
         job_presence_periods, run_jobs, jd_versions, jobs CASCADE`); err != nil {
		t.Fatalf("clear disposable test database: %v", err)
	}
	return repository
}

func prepareSyntheticAnalysisConfiguration(t *testing.T, repository *Repository, dictionaryID, parserVersion string) string {
	t.Helper()
	preparation, err := repository.PrepareCandidateConfiguration(context.Background(), CandidateConfigurationInput{
		ParserVersion: parserVersion, DictionaryVersionID: dictionaryID, CreatedBy: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return preparation.ConfigurationID
}

func insertSyntheticAnalysisConfiguration(t *testing.T, repository *Repository, configurationID, parserVersion, dictionaryID string) {
	t.Helper()
	ctx := context.Background()
	snapshot, err := repository.loadEnabledRuleSet(ctx, repository.pool, dictionaryID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_analysis_configurations (
    id, parser_version, dictionary_version_id, rule_set_sha256, created_by
) VALUES ($1, $2, $3, $4, 'test')`, configurationID, parserVersion, dictionaryID, snapshot.SHA256); err != nil {
		t.Fatal(err)
	}
	for _, ruleID := range snapshot.RuleIDs {
		if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_analysis_configuration_rules (configuration_id, dictionary_rule_id)
VALUES ($1, $2)`, configurationID, ruleID); err != nil {
			t.Fatal(err)
		}
	}
}

func acquireIntegrationTestLock(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	connection, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Exec(context.Background(), `SELECT pg_advisory_lock(873210004)`); err != nil {
		connection.Release()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = connection.Exec(context.Background(), `SELECT pg_advisory_unlock(873210004)`)
		connection.Release()
	})
}

func insertSyntheticAnalysisPrerequisites(t *testing.T, repository *Repository) (bodyID, dictionaryID, technologyID, ruleID string) {
	t.Helper()
	ctx := context.Background()
	const (
		jobID       = "00000000-0000-4000-8000-000000000501"
		bodyVersion = "00000000-0000-4000-8000-000000000502"
		technology  = "00000000-0000-4000-8000-000000000503"
		dictionary  = "00000000-0000-4000-8000-000000000504"
		rule        = "00000000-0000-4000-8000-000000000505"
		description = "Go, Go"
		jobRequire  = ""
		addition    = ""
	)
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO jobs (id, source_company, source_job_id)
VALUES ($1, 'synthetic-source', 'synthetic-job')`, jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO job_body_content_versions (
    id, job_id, body_sha256, normalization_version,
    description_text, job_require_text, addition_text,
    description_sha256, job_require_sha256, addition_sha256
) VALUES ($1, $2, $3, 'jd-body-v1', $4, $5, $6, $7, $8, $9)`,
		bodyVersion, jobID, testSHA256(description+"|"+jobRequire+"|"+addition),
		description, jobRequire, addition,
		testSHA256(description), testSHA256(jobRequire), testSHA256(addition)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_identities (id, technology_key, canonical_name, technology_kind)
VALUES ($1, 'go', 'Go', 'programming-language')`, technology); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_dictionary_versions (id, version_label, content_sha256, status, source_document, created_by)
VALUES ($1, 'synthetic-dictionary-v1', $2, 'candidate', 'synthetic test', 'test')`, dictionary, testSHA256("dictionary")); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_dictionary_rules (
    id, dictionary_version_id, technology_identity_id, rule_key, alias, state,
    case_mode, boundary_mode, rule_summary, approved_by, approved_at
) VALUES ($1, $2, $3, 'go.01', 'Go', 'enabled', 'sensitive', 'ascii_identifier', 'synthetic test rule', 'reviewer', now())`, rule, dictionary, technology); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_dictionary_rule_decisions (rule_id, decision, reviewer, rationale)
VALUES ($1, 'approved_enabled', 'reviewer', 'synthetic approval')`, rule); err != nil {
		t.Fatal(err)
	}
	return bodyVersion, dictionary, technology, rule
}

func testSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
