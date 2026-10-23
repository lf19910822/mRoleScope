package jobsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"mrolescope/internal/collector"
)

// This test intentionally needs a migrated, disposable PostgreSQL database.
// It is skipped during ordinary unit tests; CI or an operator may supply
// TEST_DATABASE_URL after applying migrations to exercise the real constraints.
func TestRepositorySCD2Lifecycle(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()

	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	release, acquired, err := repository.TryAcquireScopeLock(ctx, scope.ID)
	if err != nil || !acquired {
		t.Fatalf("first advisory lock: acquired=%t err=%v", acquired, err)
	}
	_, secondAcquired, err := repository.TryAcquireScopeLock(ctx, scope.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secondAcquired {
		t.Fatal("second concurrent advisory lock unexpectedly acquired")
	}
	release()

	base := time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC)
	publishFixtureRun(t, repository, scope, base, 1, []Observation{integrationObservation("v1", base)}, nil)
	assertCounts(t, repository, ctx, 1, 1, 1, 1, 0)

	publishFixtureRun(t, repository, scope, base.Add(time.Minute), 1, []Observation{integrationObservation("v1", base.Add(time.Minute))}, nil)
	assertCounts(t, repository, ctx, 1, 1, 1, 1, 0)

	publishFixtureRun(t, repository, scope, base.Add(2*time.Minute), 1, []Observation{integrationObservation("v2", base.Add(2*time.Minute))}, nil)
	assertCounts(t, repository, ctx, 1, 2, 1, 2, 0)

	publishFixtureRun(t, repository, scope, base.Add(3*time.Minute), 1, nil, map[string]RecheckResult{"8771": {Outcome: RecheckAvailable, Code: "detail_available"}})
	assertCounts(t, repository, ctx, 1, 2, 1, 2, 0)

	publishFixtureRun(t, repository, scope, base.Add(4*time.Minute), 1, nil, map[string]RecheckResult{"8771": {Outcome: RecheckDefinitelyUnavailable, Code: "fixture_unavailable"}})
	assertCounts(t, repository, ctx, 1, 2, 1, 2, 1)

	publishFixtureRun(t, repository, scope, base.Add(5*time.Minute), 1, []Observation{integrationObservation("v3", base.Add(5*time.Minute))}, nil)
	assertCounts(t, repository, ctx, 1, 3, 2, 3, 1)
	assertPeriodConstraints(t, repository, ctx, scope.ID)
	assertCurrentStateQueryPlan(t, repository, ctx, scope.ID)

	otherScope, err := repository.EnsureScope(ctx, Scope{
		SourceCompany: SourceCompanyMihoyo,
		Filter:        map[string]any{"hireType": float64(1)},
		FilterSHA256:  strings.Repeat("1", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	publishFixtureRun(t, repository, otherScope, base.Add(6*time.Minute), 1, []Observation{integrationObservation("v3", base.Add(6*time.Minute))}, nil)
	publishFixtureRun(t, repository, otherScope, base.Add(7*time.Minute), 1, nil, map[string]RecheckResult{"8771": {Outcome: RecheckDefinitelyUnavailable, Code: "fixture_unavailable"}})
	var stillOpen int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM job_presence_periods WHERE observation_scope_id = $1 AND closed_at IS NULL`, scope.ID).Scan(&stillOpen); err != nil {
		t.Fatal(err)
	}
	if stillOpen != 1 {
		t.Fatalf("scope-isolated run closed the original scope's period; open=%d", stillOpen)
	}
}

func TestRepositoryPublishIsIdempotent(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC)
	run, err := repository.CreateRun(ctx, scope, newUUID(), TriggerInitialization, 1, at)
	if err != nil {
		t.Fatal(err)
	}
	manifest := collector.Manifest{
		SchemaVersion: 1, Status: "complete", RunID: run.ID, StartedAt: at, FinishedAt: at.Add(time.Second),
		Source: collector.Source{Company: scope.SourceCompany, Filter: scope.Filter, FilterSHA256: scope.FilterSHA256},
		Counts: collector.Counts{ListReportedTotal: 1, ListItems: 1, UniqueSourceJobID: 1, DetailSucceeded: 1},
	}
	input := PublishInput{
		Run: run, Scope: scope, Manifest: manifest, Observations: []Observation{integrationObservation("v1", at)},
		Archive: ArchiveResult{URI: "s3://test-bucket/run/" + run.ID, ManifestSHA256: strings.Repeat("0", 64)},
	}
	if err := repository.Publish(ctx, input); err != nil {
		t.Fatal(err)
	}
	if err := repository.Publish(ctx, input); err != nil {
		t.Fatal(err)
	}
	assertCounts(t, repository, ctx, 1, 1, 1, 1, 0)
	var publishedAuditEvents int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM collection_run_audit_events WHERE run_id = $1 AND code = 'run_published'`, run.ID).Scan(&publishedAuditEvents); err != nil {
		t.Fatal(err)
	}
	if publishedAuditEvents != 1 {
		t.Fatalf("published audit events = %d, want 1", publishedAuditEvents)
	}
}

func TestRepositoryWritesCanonicalVersionsAndQueuesOnlyNewBodies(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	configurationID := insertPublishedAnalysisConfiguration(t, repository)
	base := time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC)
	first := publishFixtureRun(t, repository, scope, base, 1, []Observation{integrationObservationWithTitleAndBody("Synthetic title A", "v1", base)}, nil)
	second := publishFixtureRun(t, repository, scope, base.Add(time.Minute), 1, []Observation{integrationObservationWithTitleAndBody("Synthetic title B", "v1", base.Add(time.Minute))}, nil)
	third := publishFixtureRun(t, repository, scope, base.Add(2*time.Minute), 1, []Observation{integrationObservationWithTitleAndBody("Synthetic title B", "v2", base.Add(2*time.Minute))}, nil)
	publishFixtureRun(t, repository, scope, base.Add(3*time.Minute), 1, nil, map[string]RecheckResult{"8771": {Outcome: RecheckDefinitelyUnavailable, Code: "synthetic_unavailable"}})
	fifth := publishFixtureRun(t, repository, scope, base.Add(4*time.Minute), 1, []Observation{integrationObservationWithTitleAndBody("Synthetic title B", "v2", base.Add(4*time.Minute))}, nil)

	var metadataVersions, bodyVersions, references, bodyPeriods, closedBodyPeriods, queueItems int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM job_source_metadata_versions),
  (SELECT count(*) FROM job_body_content_versions),
  (SELECT count(*) FROM run_job_version_references),
  (SELECT count(*) FROM job_body_content_periods),
  (SELECT count(*) FROM job_body_content_periods WHERE valid_to IS NOT NULL),
  (SELECT count(*) FROM technology_analysis_work_items WHERE configuration_id = $1)`, configurationID).Scan(
		&metadataVersions, &bodyVersions, &references, &bodyPeriods, &closedBodyPeriods, &queueItems); err != nil {
		t.Fatal(err)
	}
	if metadataVersions != 2 || bodyVersions != 2 || references != 4 || bodyPeriods != 3 || closedBodyPeriods != 2 || queueItems != 2 {
		t.Fatalf("canonical publication counts = metadata:%d body:%d references:%d bodyPeriods:%d closedBodyPeriods:%d queue:%d", metadataVersions, bodyVersions, references, bodyPeriods, closedBodyPeriods, queueItems)
	}
	firstMetadata, firstBody := canonicalReferenceForRun(t, repository, first.ID)
	secondMetadata, secondBody := canonicalReferenceForRun(t, repository, second.ID)
	thirdMetadata, thirdBody := canonicalReferenceForRun(t, repository, third.ID)
	_, fifthBody := canonicalReferenceForRun(t, repository, fifth.ID)
	if firstMetadata == secondMetadata || secondMetadata != thirdMetadata {
		t.Fatal("metadata-only title change did not produce exactly one new metadata version")
	}
	if firstBody != secondBody || secondBody == thirdBody || thirdBody != fifthBody {
		t.Fatal("body version references do not preserve metadata-only reuse, body changes, and reappearance")
	}
	var pendingNewBodyItems int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM technology_analysis_work_items
WHERE configuration_id = $1 AND status = 'pending' AND trigger = 'new_body'`, configurationID).Scan(&pendingNewBodyItems); err != nil {
		t.Fatal(err)
	}
	if pendingNewBodyItems != 2 {
		t.Fatalf("pending new-body work items = %d, want 2", pendingNewBodyItems)
	}
}

func TestRepositoryBackfillsVersionedContentWithoutChangingLegacyHistory(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()
	scope, err := repository.EnsureScope(ctx, MihoyoTechnicalScope())
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 22, 2, 0, 0, 0, time.UTC)
	publishFixtureRun(t, repository, scope, base, 1, []Observation{integrationObservation("v1", base)}, nil)
	publishFixtureRun(t, repository, scope, base.Add(time.Minute), 1, []Observation{integrationObservation("v1", base.Add(time.Minute))}, nil)
	publishFixtureRun(t, repository, scope, base.Add(2*time.Minute), 1, []Observation{integrationObservation("v2", base.Add(2*time.Minute))}, nil)
	if _, err := repository.pool.Exec(ctx, `
DELETE FROM technology_analysis_work_items;
DELETE FROM job_body_content_periods;
DELETE FROM run_job_version_references;
DELETE FROM job_body_content_versions;
DELETE FROM job_source_metadata_versions`); err != nil {
		t.Fatal(err)
	}

	var legacyJobs, legacyVersions, legacyRunJobs, legacyPeriods, archiveReferences int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM jobs),
  (SELECT count(*) FROM jd_versions),
  (SELECT count(*) FROM run_jobs),
  (SELECT count(*) FROM job_content_periods),
  (SELECT count(*) FROM collection_runs WHERE archive_uri IS NOT NULL)`).Scan(
		&legacyJobs, &legacyVersions, &legacyRunJobs, &legacyPeriods, &archiveReferences); err != nil {
		t.Fatal(err)
	}

	first, err := repository.BackfillVersionedContent(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.PublishedObservations != 3 || first.VersionReferencesCreated != 3 || first.BodyPeriodsCreated != 2 || first.SkippedObservations != 0 || first.SkippedBodyPeriods != 0 {
		t.Fatalf("unexpected first backfill summary: %#v", first)
	}
	var metadataVersions, bodyVersions, versionReferences, bodyPeriods int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM job_source_metadata_versions),
  (SELECT count(*) FROM job_body_content_versions),
  (SELECT count(*) FROM run_job_version_references),
  (SELECT count(*) FROM job_body_content_periods)`).Scan(&metadataVersions, &bodyVersions, &versionReferences, &bodyPeriods); err != nil {
		t.Fatal(err)
	}
	if metadataVersions != 2 || bodyVersions != 2 || versionReferences != 3 || bodyPeriods != 2 {
		t.Fatalf("backfill counts = metadata:%d body:%d refs:%d periods:%d", metadataVersions, bodyVersions, versionReferences, bodyPeriods)
	}

	second, err := repository.BackfillVersionedContent(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second.VersionReferencesCreated != 0 || second.BodyPeriodsCreated != 0 {
		t.Fatalf("second backfill was not idempotent: %#v", second)
	}
	var gotLegacyJobs, gotLegacyVersions, gotLegacyRunJobs, gotLegacyPeriods, gotArchiveReferences int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM jobs),
  (SELECT count(*) FROM jd_versions),
  (SELECT count(*) FROM run_jobs),
  (SELECT count(*) FROM job_content_periods),
  (SELECT count(*) FROM collection_runs WHERE archive_uri IS NOT NULL)`).Scan(
		&gotLegacyJobs, &gotLegacyVersions, &gotLegacyRunJobs, &gotLegacyPeriods, &gotArchiveReferences); err != nil {
		t.Fatal(err)
	}
	if gotLegacyJobs != legacyJobs || gotLegacyVersions != legacyVersions || gotLegacyRunJobs != legacyRunJobs || gotLegacyPeriods != legacyPeriods || gotArchiveReferences != archiveReferences {
		t.Fatal("backfill changed legacy job history or archive references")
	}
}

func TestServicePublishesAndSafelyRecordsFailures(t *testing.T) {
	repository := newIntegrationRepository(t)
	ctx := context.Background()

	t.Run("successful_fixture_run_has_ordered_stage_audit", func(t *testing.T) {
		runID := newUUID()
		outputDir := serviceFixtureTask(t, runID)
		store := &memoryObjectStore{objects: map[string]memoryObject{}}
		service := Service{
			Repository: repository,
			Collector:  fixtureCollector{},
			Archiver:   Archiver{Store: store, Prefix: "raw"},
		}
		if err := service.Run(ctx, RunOptions{RunID: runID, OutputDir: outputDir, Trigger: TriggerInitialization, AttemptNumber: 1}); err != nil {
			t.Fatal(err)
		}
		if len(store.objects) != 3 {
			t.Fatalf("archived objects = %d, want list, detail, and manifest", len(store.objects))
		}
		assertCounts(t, repository, ctx, 1, 1, 1, 1, 0)
		assertAuditSequence(t, repository, ctx, runID, []string{
			"collecting/collection_started",
			"validating/batch_validated",
			"archiving/archive_started",
			"persisting/publication_started",
			"published/run_published",
		})
	})

	t.Run("archive_failure_does_not_publish_or_leak_source_text", func(t *testing.T) {
		runID := newUUID()
		outputDir := serviceFixtureTask(t, runID)
		service := Service{
			Repository: repository,
			Collector:  fixtureCollector{},
			Archiver:   &fakeArchiver{err: errors.New("password=pretend-secret <p>confidential JD body</p>")},
		}
		err := service.Run(ctx, RunOptions{RunID: runID, OutputDir: outputDir, Trigger: TriggerScheduled, AttemptNumber: 1})
		var runFailure RunFailure
		if !errors.As(err, &runFailure) || runFailure.Stage != "archiving" || runFailure.Code != "archive_failed" {
			t.Fatalf("error = %#v, want safe archive RunFailure", err)
		}
		assertCounts(t, repository, ctx, 1, 1, 1, 1, 0)
		assertFailedRunIsSafe(t, repository, ctx, runID, "archive_failed", []string{"pretend-secret", "confidential JD body", "password="})
	})

	t.Run("database_failure_after_archive_keeps_prior_history", func(t *testing.T) {
		runID := newUUID()
		outputDir := serviceFixtureTask(t, runID)
		archiver := &fakeArchiver{result: ArchiveResult{
			URI: "s3://test-bucket/unpublished/" + runID, ManifestSHA256: "not-a-valid-sha256",
		}}
		service := Service{Repository: repository, Collector: fixtureCollector{}, Archiver: archiver}
		err := service.Run(ctx, RunOptions{RunID: runID, OutputDir: outputDir, Trigger: TriggerRetry, AttemptNumber: 2})
		var runFailure RunFailure
		if !errors.As(err, &runFailure) || runFailure.Stage != "persisting" || runFailure.Code != "publication_failed" {
			t.Fatalf("error = %#v, want safe persistence RunFailure", err)
		}
		if !archiver.called {
			t.Fatal("expected archive before the database failure")
		}
		assertCounts(t, repository, ctx, 1, 1, 1, 1, 0)
		assertFailedRunIsSafe(t, repository, ctx, runID, "publication_failed", nil)
	})

	t.Run("list_omission_with_available_detail_stays_open_and_is_safe", func(t *testing.T) {
		runID := newUUID()
		outputDir := serviceFixtureTaskForJob(t, runID, "9000")
		service := Service{
			Repository: repository,
			Collector:  fixtureCollector{},
			Archiver: &fakeArchiver{result: ArchiveResult{
				URI: "s3://test-bucket/omission/" + runID, ManifestSHA256: strings.Repeat("b", 64),
			}},
			Rechecker: fakeRechecker{results: map[string]RecheckResult{
				"8771": {Outcome: RecheckAvailable, Code: "detail_available"},
			}},
		}
		if err := service.Run(ctx, RunOptions{RunID: runID, OutputDir: outputDir, Trigger: TriggerScheduled, AttemptNumber: 1}); err != nil {
			t.Fatal(err)
		}
		var open8771, safeAudit int
		if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM job_presence_periods AS p JOIN jobs AS j ON j.id = p.job_id WHERE j.source_job_id = '8771' AND p.closed_at IS NULL),
  (SELECT count(*) FROM collection_run_audit_events WHERE run_id = $1 AND code = 'detail_available' AND message NOT LIKE '%Build services%' AND diagnostics::text NOT LIKE '%Build services%')`, runID).Scan(&open8771, &safeAudit); err != nil {
			t.Fatal(err)
		}
		if open8771 != 1 || safeAudit == 0 {
			t.Fatalf("list omission result = open:%d safe audit:%d, want 1 and non-zero", open8771, safeAudit)
		}
	})
}

type fixtureCollector struct{}

func (fixtureCollector) Run(_ context.Context, _ string, outputDir, _ string) (collector.Manifest, error) {
	return collector.ReadManifest(filepath.Join(outputDir, "manifest.json"))
}

type fakeArchiver struct {
	result ArchiveResult
	err    error
	called bool
}

type fakeRechecker struct {
	results map[string]RecheckResult
}

func (checker fakeRechecker) Recheck(_ context.Context, sourceJobID string) RecheckResult {
	if result, ok := checker.results[sourceJobID]; ok {
		return result
	}
	return RecheckResult{Outcome: RecheckIndeterminate, Code: "fixture_indeterminate"}
}

func (archiver *fakeArchiver) Archive(_ context.Context, _ string, _ Scope, _ string, _ collector.Manifest) (ArchiveResult, error) {
	archiver.called = true
	if archiver.err != nil {
		return ArchiveResult{}, archiver.err
	}
	return archiver.result, nil
}

func serviceFixtureTask(t *testing.T, runID string) string {
	return serviceFixtureTaskForJob(t, runID, "8771")
}

func serviceFixtureTaskForJob(t *testing.T, runID, sourceJobID string) string {
	t.Helper()
	list := fmt.Sprintf(`{"success":true,"data":{"total":1,"list":[{"id":%q,"title":"Server","addressDetailList":[{"addressDetail":"Shanghai"}]}]}}`, sourceJobID)
	detail := fmt.Sprintf(`{"success":true,"data":{"id":%q,"title":"Server","description":"<p>Build services</p>","jobRequire":"<p>Go</p>"}}`, sourceJobID)
	root, manifest := writeTaskDirectoryForID(t, sourceJobID, list, detail)
	manifest.RunID = runID
	contents, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
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
	acquireJobsyncIntegrationTestLock(t, repository)
	if _, err := repository.pool.Exec(context.Background(), `
TRUNCATE collection_run_audit_events, job_body_content_periods,
         run_job_version_references, job_body_content_versions,
         job_source_metadata_versions, job_content_periods, job_presence_periods,
         run_jobs, jd_versions, jobs, observation_scope_publication_state,
         collection_runs, observation_scopes CASCADE`); err != nil {
		t.Fatalf("clear disposable test database: %v", err)
	}
	if _, err := repository.pool.Exec(context.Background(), `
INSERT INTO technology_analysis_publication_state (singleton)
VALUES (true)
ON CONFLICT (singleton) DO NOTHING`); err != nil {
		t.Fatalf("restore disposable analysis publication state: %v", err)
	}
	return repository
}

func acquireJobsyncIntegrationTestLock(t *testing.T, repository *Repository) {
	t.Helper()
	connection, err := repository.pool.Acquire(context.Background())
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

func publishFixtureRun(t *testing.T, repository *Repository, scope StoredScope, at time.Time, attempt int, observations []Observation, rechecks map[string]RecheckResult) Run {
	t.Helper()
	run, err := repository.CreateRun(context.Background(), scope, newUUID(), TriggerInitialization, attempt, at)
	if err != nil {
		t.Fatal(err)
	}
	manifest := collector.Manifest{
		SchemaVersion: 1,
		Status:        "complete",
		RunID:         run.ID,
		StartedAt:     at,
		FinishedAt:    at.Add(time.Second),
		Source:        collector.Source{Company: scope.SourceCompany, Filter: scope.Filter, FilterSHA256: scope.FilterSHA256},
		Counts: collector.Counts{
			ListReportedTotal: len(observations), ListItems: len(observations), UniqueSourceJobID: len(observations), DetailSucceeded: len(observations),
		},
	}
	if err := repository.Publish(context.Background(), PublishInput{
		Run: run, Scope: scope, Manifest: manifest, Observations: observations, Rechecks: rechecks,
		Archive: ArchiveResult{URI: "s3://test-bucket/run/" + run.ID, ManifestSHA256: "0000000000000000000000000000000000000000000000000000000000000000"},
	}); err != nil {
		t.Fatal(err)
	}
	return run
}

func insertPublishedAnalysisConfiguration(t *testing.T, repository *Repository) string {
	t.Helper()
	const (
		dictionaryID    = "00000000-0000-4000-8000-000000000801"
		configurationID = "00000000-0000-4000-8000-000000000802"
	)
	ctx := context.Background()
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_dictionary_versions (
    id, version_label, content_sha256, status, source_document, created_by, published_at
) VALUES ($1, 'synthetic-published-dictionary', $2, 'published', 'synthetic test', 'test', now())`, dictionaryID, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.pool.Exec(ctx, `
INSERT INTO technology_analysis_configurations (
    id, parser_version, dictionary_version_id, rule_set_sha256, status, created_by, published_at
) VALUES ($1, 'synthetic-parser', $2, $3, 'published', 'test', now())`, configurationID, dictionaryID, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	command, err := repository.pool.Exec(ctx, `
UPDATE technology_analysis_publication_state
SET configuration_id = $1, updated_at = now()
WHERE singleton = true`, configurationID)
	if err != nil {
		t.Fatal(err)
	}
	if command.RowsAffected() != 1 {
		t.Fatal("analysis publication state was not initialized")
	}
	t.Cleanup(func() {
		cleanupContext := context.Background()
		_, _ = repository.pool.Exec(cleanupContext, `DELETE FROM technology_analysis_work_items WHERE configuration_id = $1`, configurationID)
		_, _ = repository.pool.Exec(cleanupContext, `UPDATE technology_analysis_publication_state SET configuration_id = NULL, updated_at = now() WHERE singleton = true`)
		_, _ = repository.pool.Exec(cleanupContext, `DELETE FROM technology_analysis_configurations WHERE id = $1`, configurationID)
		_, _ = repository.pool.Exec(cleanupContext, `DELETE FROM technology_dictionary_versions WHERE id = $1`, dictionaryID)
	})
	return configurationID
}

func canonicalReferenceForRun(t *testing.T, repository *Repository, runID string) (metadataID, bodyID string) {
	t.Helper()
	if err := repository.pool.QueryRow(context.Background(), `
SELECT source_metadata_version_id::text, body_content_version_id::text
FROM run_job_version_references
WHERE run_id = $1`, runID).Scan(&metadataID, &bodyID); err != nil {
		t.Fatal(err)
	}
	return metadataID, bodyID
}

func integrationObservation(version string, at time.Time) Observation {
	return integrationObservationWithTitleAndBody("Synthetic "+version, version, at)
}

func integrationObservationWithTitleAndBody(title, bodyVersion string, at time.Time) Observation {
	observation, err := NormalizeObservation(SourceCompanyMihoyo,
		map[string]any{"id": "8771", "addressDetailList": []any{map[string]any{"addressId": "1", "addressDetail": "Synthetic city"}}},
		map[string]any{
			"id": "8771", "title": title,
			"description": "Synthetic description " + bodyVersion,
			"jobRequire":  "Synthetic requirement",
			"addition":    "Synthetic addition",
		}, at)
	if err != nil {
		panic(err)
	}
	return observation
}

func assertCounts(t *testing.T, repository *Repository, ctx context.Context, jobs, versions, presence, content, closedPresence int) {
	t.Helper()
	var gotJobs, gotVersions, gotPresence, gotContent, gotClosed int
	if err := repository.pool.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM jobs),
  (SELECT count(*) FROM jd_versions),
  (SELECT count(*) FROM job_presence_periods),
  (SELECT count(*) FROM job_content_periods),
  (SELECT count(*) FROM job_presence_periods WHERE closed_at IS NOT NULL)`).Scan(&gotJobs, &gotVersions, &gotPresence, &gotContent, &gotClosed); err != nil {
		t.Fatal(err)
	}
	if gotJobs != jobs || gotVersions != versions || gotPresence != presence || gotContent != content || gotClosed != closedPresence {
		t.Fatalf("counts = jobs:%d versions:%d presence:%d content:%d closed:%d, want jobs:%d versions:%d presence:%d content:%d closed:%d", gotJobs, gotVersions, gotPresence, gotContent, gotClosed, jobs, versions, presence, content, closedPresence)
	}
}

func assertPeriodConstraints(t *testing.T, repository *Repository, ctx context.Context, scopeID string) {
	t.Helper()
	_, err := repository.pool.Exec(ctx, `
INSERT INTO job_presence_periods (
    observation_scope_id, job_id, opened_run_id, opened_at, last_seen_run_id, last_seen_at
)
SELECT observation_scope_id, job_id, opened_run_id, opened_at, last_seen_run_id, last_seen_at
FROM job_presence_periods
WHERE observation_scope_id = $1 AND closed_at IS NULL
LIMIT 1`, scopeID)
	assertUniqueViolation(t, err, "second open presence period")

	_, err = repository.pool.Exec(ctx, `
INSERT INTO job_content_periods (presence_period_id, jd_version_id, valid_from_run_id, valid_from)
SELECT presence_period_id, jd_version_id, valid_from_run_id, valid_from
FROM job_content_periods
WHERE valid_to IS NULL
LIMIT 1`)
	assertUniqueViolation(t, err, "second active content period")

	_, err = repository.pool.Exec(ctx, `
INSERT INTO collection_run_audit_events (run_id, stage, severity, code, attempt_number, message)
VALUES ('00000000-0000-4000-8000-000000000001', 'collecting', 'error', 'test_fk', 1, 'test')`)
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || pgError.Code != "23503" {
		t.Fatalf("audit foreign key error = %v, want PostgreSQL foreign key violation", err)
	}
}

func assertUniqueViolation(t *testing.T, err error, label string) {
	t.Helper()
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || pgError.Code != "23505" {
		t.Fatalf("%s error = %v, want PostgreSQL unique violation", label, err)
	}
}

func assertCurrentStateQueryPlan(t *testing.T, repository *Repository, ctx context.Context, scopeID string) {
	t.Helper()
	rows, err := repository.pool.Query(ctx, `
EXPLAIN (COSTS OFF)
SELECT p.id, j.source_job_id
FROM job_presence_periods AS p
JOIN jobs AS j ON j.id = p.job_id
WHERE p.observation_scope_id = $1 AND p.closed_at IS NULL
ORDER BY j.source_job_id`, scopeID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"jd_versions", "raw_fields", "display_text"} {
		if strings.Contains(plan.String(), forbidden) {
			t.Fatalf("current-state query plan unexpectedly scans %q:\n%s", forbidden, plan.String())
		}
	}
	var indexCount int
	if err := repository.pool.QueryRow(ctx, `
SELECT count(*)
FROM pg_indexes
WHERE schemaname = current_schema()
  AND indexname IN ('job_presence_periods_scope_current_idx', 'collection_runs_scope_data_date_idx', 'collection_run_audit_events_recent_failures_idx')`).Scan(&indexCount); err != nil {
		t.Fatal(err)
	}
	if indexCount != 3 {
		t.Fatalf("expected synchronization access indexes, got %d", indexCount)
	}
}

func assertAuditSequence(t *testing.T, repository *Repository, ctx context.Context, runID string, want []string) {
	t.Helper()
	rows, err := repository.pool.Query(ctx, `
SELECT stage || '/' || code
FROM collection_run_audit_events
WHERE run_id = $1
ORDER BY occurred_at, id`, runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := make([]string, 0)
	for rows.Next() {
		var event string
		if err := rows.Scan(&event); err != nil {
			t.Fatal(err)
		}
		got = append(got, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("audit sequence = %#v, want %#v", got, want)
	}
}

func assertFailedRunIsSafe(t *testing.T, repository *Repository, ctx context.Context, runID, wantCode string, forbidden []string) {
	t.Helper()
	var status, summary, code, message string
	if err := repository.pool.QueryRow(ctx, `
SELECT run.status, run.failure_summary, event.code, event.message
FROM collection_runs AS run
JOIN collection_run_audit_events AS event ON event.run_id = run.id
WHERE run.id = $1 AND event.severity = 'error'
ORDER BY event.occurred_at, event.id
LIMIT 1`, runID).Scan(&status, &summary, &code, &message); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || code != wantCode {
		t.Fatalf("failed run = status:%s code:%s, want failed/%s", status, code, wantCode)
	}
	for _, value := range forbidden {
		if strings.Contains(summary, value) || strings.Contains(message, value) {
			t.Fatalf("unsafe failure text %q leaked into audit: summary=%q message=%q", value, summary, message)
		}
	}
}
